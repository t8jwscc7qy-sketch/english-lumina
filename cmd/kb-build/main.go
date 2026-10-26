// Команда kb-build: быстрая сборка базы знаний kb_* из слов, частот, прежнего словаря и подготовленного файла Kaikki
// (cmd/kb-prepare). `go run ./cmd/kb-build` собирает и записывает, `-dry-run` только печатает отчёт качества.
// Сборка с качеством ниже порогов не записывается. Перед запуском: миграции применены, kb-prepare выполнен.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/kaikki"
)

const cacheDir = ".cache" // вне Git (см. .gitignore)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("kb-build", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "только отчёт качества, без записи в базу")
	explain := fs.String("explain", "", "слова через запятую (лемма/часть_речи): показать кандидатов-переводов с оценками, без записи")
	if err := fs.Parse(args); err != nil {
		return err
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
	opts := Options{Check: kb.DefaultOptions()}
	var in *os.File
	path := filepath.Join(cacheDir, "kb", "kaikki-en.jsonl")
	if f, err := os.Open(path); err == nil {
		defer func() { _ = f.Close() }()
		meta, err := kb.ReadMeta(path + ".meta")
		if err != nil {
			return fmt.Errorf("нет сведений о файле Kaikki (%s.meta), запустите go run ./cmd/kb-prepare kaikki: %w", path, err)
		}
		if meta.FileSHA256 != kaikki.FileSHA256 {
			return errors.New("подготовленный файл сделан по другой версии Kaikki, чем закреплена в коде: запустите go run ./cmd/kb-prepare kaikki")
		}
		if meta.LemmasHash != kb.LemmasHash(lemmas) {
			return errors.New("список слов изменился после подготовки Kaikki: запустите go run ./cmd/kb-prepare kaikki")
		}
		in = f
		opts.Sources = map[string]any{"kaikki": map[string]any{"version": kaikki.DumpDate, "fileSha256": meta.FileSHA256, "selected": meta.Selected}}
	} else if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "файла Kaikki нет (запустите kb-prepare kaikki): только прежние источники; контрольный список будет ниже порога, и обычная сборка, скорее всего, не пройдёт проверку качества")
	} else {
		return err
	}

	if *explain != "" {
		var r io.Reader
		if in != nil {
			r = in
		}
		return Explain(ctx, pool, r, strings.Split(*explain, ","), os.Stdout)
	}

	var rep kb.Report
	var buildErr error
	switch {
	case *dry && in != nil:
		rep, buildErr = Preview(ctx, pool, in)
	case *dry:
		rep, buildErr = Preview(ctx, pool, nil)
	case in != nil:
		rep, buildErr = Build(ctx, pool, in, opts)
	default:
		rep, buildErr = Build(ctx, pool, nil, opts)
	}
	if err := PrintReport(os.Stdout, rep); err != nil {
		return err
	}
	if buildErr != nil {
		return buildErr
	}
	if *dry {
		fmt.Println(ThresholdLine(rep, opts.Check))
		fmt.Println("пробный запуск: в базу ничего не записано")
	} else {
		fmt.Println("база знаний собрана")
	}
	return nil
}
