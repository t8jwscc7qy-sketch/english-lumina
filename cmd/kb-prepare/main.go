// Команда kb-prepare: подготовка источников базы знаний. `go run ./cmd/kb-prepare kaikki` скачивает файл Kaikki
// (≈ 2,9 ГБ, один раз), проверяет контрольную сумму, потоком отбирает записи английских слов из таблицы lexemes в
// компактный файл .cache/kb/kaikki-en.jsonl (с .meta) и регистрирует источник в data_sources. Шаг медленный и нужен
// только при новой версии файла или новом списке слов; быстрая сборка базы знаний — cmd/kb-build.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/kaikki"
)

const cacheDir = ".cache" // вне Git (см. .gitignore)

func main() {
	// Ctrl+C отменяет контекст: проход прерывается, недописанный файл удаляется.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 1 || args[0] != "kaikki" {
		return errors.New("использование: go run ./cmd/kb-prepare kaikki")
	}
	if kaikki.FileSHA256 == "" {
		return errors.New("контрольная сумма файла Kaikki не закреплена: скачайте файл вручную в " +
			filepath.Join(cacheDir, "sources", "kaikki", kaikki.FileName) + ", посчитайте shasum -a 256 и впишите сумму в internal/sources/kaikki/kaikki.go")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("подключение к базе: %w", err)
	}
	defer pool.Close()
	q := db.New(pool)

	lemmas, err := q.KbListLemmas(ctx)
	if err != nil {
		return fmt.Errorf("список слов: %w", err)
	}
	set := kb.LemmaSet(lemmas)

	// Общий тайм-аут клиента importer.NewHTTPClient (2 минуты) для файла в несколько гигабайт мал:
	// ограничиваем только установку соединения и ожидание ответа, остальное прерывает контекст.
	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment, TLSHandshakeTimeout: 30 * time.Second, ResponseHeaderTimeout: time.Minute,
	}}
	src := filepath.Join(cacheDir, "sources", "kaikki", kaikki.FileName)
	fmt.Fprintf(os.Stderr, "файл Kaikki: %s (если его нет, скачивается ≈ 3 ГБ)\n", src)
	if err := importer.Download(ctx, client, kaikki.URL, src, kaikki.FileSHA256, kaikki.MaxFileSize); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	outDir := filepath.Join(cacheDir, "kb")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	outPath := filepath.Join(outDir, "kaikki-en.jsonl")
	tmp, err := os.Create(outPath + ".tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(outPath + ".tmp") }() // после переименования удалять нечего

	var read atomic.Int64
	stopProgress := progress(ctx, &read, info.Size())
	st, err := Prepare(ctx, &countingReader{r: f, n: &read}, set, tmp)
	stopProgress()
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(outPath+".tmp", outPath); err != nil {
		return err
	}
	if err := kb.WriteMeta(outPath+".meta", kb.Meta{FileSHA256: kaikki.FileSHA256, LemmasHash: kb.LemmasHash(lemmas), Selected: st.Selected}); err != nil {
		return err
	}

	d := kaikki.Describe()
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: d.Name, Version: d.Version, License: d.License, Url: d.URL, Attribution: d.Attribution})
	if err != nil {
		return fmt.Errorf("регистрация источника: %w", err)
	}
	rows := int32(min(st.Selected, 1<<31-1)) //nolint:gosec // записей порядка десятков тысяч
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: ds.ID, RowCount: &rows}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "Kaikki: прочитано строк %d, английских %d, отобрано %d записей → %s\n", st.Lines, st.English, st.Selected, outPath)
	return err
}

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// progress раз в 30 секунд печатает долю прочитанного архива; возвращает функцию остановки.
func progress(ctx context.Context, read *atomic.Int64, total int64) func() {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				fmt.Fprintf(os.Stderr, "прочитано %d%% архива\n", read.Load()*100/max(total, 1))
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() { close(done) }
}
