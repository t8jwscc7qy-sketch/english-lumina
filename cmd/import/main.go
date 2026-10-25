// Команда import загружает внешние базы знаний в PostgreSQL.
//
//	go run ./cmd/import list      — показать доступные источники
//	go run ./cmd/import <имя>     — загрузить источник (повторный запуск той же версии ничего не дублирует)
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/cefrj"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/freedict"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/frequencywords"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/octanove"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/tatoeba"
)

// cacheDir лежит вне Git (см. .gitignore): скачанные базы в репозиторий не попадают.
const cacheDir = ".cache"

// runner выполняет источник; в main он подключается к базе, в тестах подменяется.
type runner func(ctx context.Context, src importer.Source, cacheDir string) (importer.Result, error)

func main() {
	// Ctrl+C отменяет контекст: идущий импорт откатывается, полузагруженных данных не остаётся.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	reg := importer.NewRegistry()
	reg.Register(cefrj.New()) // новый источник подключается одной такой строкой
	reg.Register(frequencywords.New())
	reg.Register(freedict.New())
	reg.Register(octanove.New())
	reg.Register(tatoeba.New())

	if err := run(ctx, os.Args[1:], os.Stdout, reg, runOnDatabase); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

// runOnDatabase подключается к базе только когда есть что импортировать: list и ошибки аргументов базы не требуют.
func runOnDatabase(ctx context.Context, src importer.Source, cacheDir string) (importer.Result, error) {
	cfg, err := config.Load()
	if err != nil {
		return importer.Result{}, err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return importer.Result{}, fmt.Errorf("подключение к базе: %w", err)
	}
	defer pool.Close()
	return importer.Run(ctx, pool, src, cacheDir)
}

func run(ctx context.Context, args []string, stdout io.Writer, reg *importer.Registry, exec runner) error {
	names := strings.Join(reg.Names(), ", ")
	if len(args) != 1 {
		return fmt.Errorf("использование: go run ./cmd/import <имя>|list (источники: %s)", names)
	}
	if args[0] == "list" {
		for _, n := range reg.Names() {
			if _, err := fmt.Fprintln(stdout, n); err != nil {
				return err
			}
		}
		return nil
	}
	src, ok := reg.Get(args[0])
	if !ok {
		return fmt.Errorf("неизвестный источник %q; доступные: %s", args[0], names)
	}

	res, err := exec(ctx, src, cacheDir)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Источник %s %s: загружено %d строк за %.1f с\n",
		res.Info.Name, res.Info.Version, res.Rows, res.Duration.Seconds())
	return err
}
