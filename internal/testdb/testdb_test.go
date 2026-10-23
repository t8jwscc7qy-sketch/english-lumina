package testdb

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
		t.Fatalf("to_regclass(%s): %v", name, err)
	}
	return exists
}

var allTables = []string{"users", "settings", "data_sources", "lexemes", "lexeme_cefr"}

func TestMigrationsUpDownUp(t *testing.T) {
	pool := Pool(t)
	for _, name := range allTables {
		if !tableExists(t, pool, name) {
			t.Fatalf("после Up нет таблицы %s", name)
		}
	}

	Down(t, pool)
	for _, name := range allTables {
		if tableExists(t, pool, name) {
			t.Fatalf("после Down осталась таблица %s", name)
		}
	}

	Up(t, pool)
	for _, name := range allTables {
		if !tableExists(t, pool, name) {
			t.Fatalf("после повторного Up нет таблицы %s", name)
		}
	}
}

func TestKnowledgeSchemaConstraints(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()

	var srcID int64
	err := pool.QueryRow(ctx,
		`INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a') RETURNING id`).Scan(&srcID)
	if err != nil {
		t.Fatalf("data_sources: %v", err)
	}
	var lexID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO lexemes (lemma, pos, source_id) VALUES ('run', 'verb', $1) RETURNING id`, srcID).Scan(&lexID); err != nil {
		t.Fatalf("lexemes: %v", err)
	}

	tests := []struct {
		name string
		sql  string
		args []any
		code string
	}{
		{"уровень D1", `INSERT INTO lexeme_cefr (lexeme_id, source_id, level) VALUES ($1, $2, 'D1')`, []any{lexID, srcID}, "23514"},
		{"extra не объект", `INSERT INTO lexeme_cefr (lexeme_id, source_id, level, extra) VALUES ($1, $2, 'A1', '[]')`, []any{lexID, srcID}, "23514"},
		{"повтор слова и части речи", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('run', 'verb', $1)`, []any{srcID}, "23505"},
		{"повтор имени и версии", `INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a')`, nil, "23505"},
		{"пустое слово", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('', 'verb', $1)`, []any{srcID}, "23514"},
		{"слово длиннее 64", `INSERT INTO lexemes (lemma, pos, source_id) VALUES (repeat('x', 65), 'verb', $1)`, []any{srcID}, "23514"},
		{"источник не существует", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('x', 'verb', 999999)`, nil, "23503"},
		{"отрицательное row_count", `UPDATE data_sources SET row_count = -1`, nil, "23514"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, tc.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("ожидали код %s, получили %v", tc.code, err)
			}
		})
	}

	// Источник нельзя удалить, пока на него ссылаются слова (RESTRICT).
	if _, err := pool.Exec(ctx, `DELETE FROM data_sources WHERE id = $1`, srcID); err == nil {
		t.Fatal("источник со словами удалился")
	}
}
