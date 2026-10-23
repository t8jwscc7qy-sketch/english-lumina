package httpapi

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// PgStore — хранилище поверх пула PostgreSQL: обычные запросы плюс транзакции через InTx.
type PgStore struct {
	*db.Queries
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{Queries: db.New(pool), pool: pool}
}

// InTx открывает транзакцию и вызывает fn с хранилищем на ней. Ошибка или паника в fn откатывают транзакцию
// (паника не глотается), успех фиксирует. Соединение всегда возвращается в пул.
func (s *PgStore) InTx(ctx context.Context, fn func(AuthStore) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начало транзакции: %w", err)
	}
	// После Commit откат ничего не делает; контекст без отмены нужен, чтобы откат дошёл и при отмене запроса.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(&txStore{Queries: db.New(tx), tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// txStore — хранилище внутри уже открытой транзакции. Фиксирует и откатывает её только внешний InTx.
type txStore struct {
	*db.Queries
	tx pgx.Tx
}

// InTx внутри транзакции работает на той же транзакции: второй не открывается.
func (s *txStore) InTx(_ context.Context, fn func(AuthStore) error) error { return fn(s) }
