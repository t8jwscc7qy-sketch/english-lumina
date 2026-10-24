// Package testdb поднимает временную схему в базе из DATABASE_URL для интеграционных тестов.
// Рабочие таблицы в public не затрагиваются. Нет настроек или базы — тест пропускается.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
)

// Pool создаёт схему test_<время>, применяет к ней все миграции по порядку и удаляет схему после теста.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	t.Chdir(moduleRoot(t)) // config.Load читает .env из текущего каталога
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("нет настроек базы: %v", err)
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("подключение к базе: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("база недоступна: %v", err)
	}
	schema := pgx.Identifier{fmt.Sprintf("test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("создание схемы: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("удаление схемы %s: %v", schema, err)
		}
		admin.Close()
	})

	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("разбор DATABASE_URL: %v", err)
	}
	pcfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatalf("пул: %v", err)
	}
	t.Cleanup(pool.Close) // закроется раньше, чем удалится схема

	Up(t, pool)
	return pool
}

// Up применяет части «Up» всех миграций по возрастанию имён.
func Up(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, m := range readMigrations(t) {
		if _, err := pool.Exec(context.Background(), m.up); err != nil {
			t.Fatalf("миграция %s (up): %v", m.name, err)
		}
	}
}

// Down применяет части «Down» всех миграций в обратном порядке.
func Down(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ms := readMigrations(t)
	slices.Reverse(ms)
	for _, m := range ms {
		if _, err := pool.Exec(context.Background(), m.down); err != nil {
			t.Fatalf("миграция %s (down): %v", m.name, err)
		}
	}
}

type migration struct{ name, up, down string }

func readMigrations(t *testing.T) []migration {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(moduleRoot(t), "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	var out []migration
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("чтение миграции: %v", err)
		}
		upPart, downPart, _ := strings.Cut(string(raw), "-- +goose Down")
		upPart = strings.Replace(upPart, "-- +goose Up", "", 1)
		out = append(out, migration{name: filepath.Base(f), up: stripGooseMarkers(upPart), down: stripGooseMarkers(downPart)})
	}
	return out
}

// Директивы goose StatementBegin/End нужны только самому goose; для PostgreSQL это комментарии,
// но убираем их, чтобы текст был чистым SQL.
func stripGooseMarkers(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-- +goose") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod не найден")
		}
		dir = parent
	}
}
