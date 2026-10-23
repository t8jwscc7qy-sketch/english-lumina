package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Beginner открывает транзакцию; его реализует *pgxpool.Pool.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Result — итог запуска.
type Result struct {
	Info     Info
	Rows     int
	Duration time.Duration
}

// Run выполняет источник: Fetch (вне транзакции, это сеть) → Parse и Load построчно в одной транзакции.
// Любая ошибка откатывает всё: полузагруженного источника в базе не бывает. Источник без строк — ошибка.
func Run(ctx context.Context, pool Beginner, src Source, cacheDir string) (Result, error) {
	start := time.Now()
	info := src.Describe()
	fail := func(err error) (Result, error) {
		return Result{}, fmt.Errorf("импорт %s: %w", info.Name, err)
	}

	path, err := src.Fetch(ctx, cacheDir)
	if err != nil {
		return fail(err)
	}
	f, err := os.Open(path)
	if err != nil {
		return fail(fmt.Errorf("открытие файла: %w", err))
	}
	defer func() { _ = f.Close() }() // файл только читался

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fail(fmt.Errorf("начало транзакции: %w", err))
	}
	// После Commit откат ничего не делает; контекст без отмены нужен, чтобы откат дошёл и при отмене запуска.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := db.New(tx)

	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{
		Name: info.Name, Version: info.Version, License: info.License, Url: info.URL, Attribution: info.Attribution,
	})
	if err != nil {
		return fail(fmt.Errorf("запись источника: %w", err))
	}

	rows := 0
	err = src.Parse(ctx, f, func(row any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := src.Load(ctx, q, ds.ID, row); err != nil {
			return err
		}
		rows++
		return nil
	})
	if err != nil {
		return fail(err)
	}
	if rows == 0 {
		return fail(errors.New("в источнике нет строк"))
	}

	n := int32(rows) //nolint:gosec // число строк справочника заведомо меньше 2^31
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: ds.ID, RowCount: &n}); err != nil {
		return fail(fmt.Errorf("завершение записи источника: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return fail(fmt.Errorf("commit: %w", err))
	}
	return Result{Info: info, Rows: rows, Duration: time.Since(start)}, nil
}
