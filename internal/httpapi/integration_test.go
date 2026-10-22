package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/config"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// integrationPool поднимает временную схему в базе из DATABASE_URL, применяет к ней миграцию
// и удаляет схему после теста. Рабочие таблицы в public не затрагиваются.
// Нет настроек или базы — тест пропускается.
func integrationPool(t *testing.T) *pgxpool.Pool {
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

	migration, err := os.ReadFile("migrations/00001_init.sql")
	if err != nil {
		t.Fatalf("чтение миграции: %v", err)
	}
	up, _, _ := strings.Cut(string(migration), "-- +goose Down")
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("применение миграции: %v", err)
	}
	return pool
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

// Весь путь на настоящей базе: настоящие запросы sqlc, настоящий пароль и токен.
func TestAuthFlowOnRealDatabase(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	e := env{h: NewHandler(pool, q, auth.NewThrottle(5, time.Minute, time.Now))}

	// Вход.
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("вход: статус %d, тело %s", rec.Code, rec.Body)
	}
	token := sessionFrom(t, rec).Value

	// В базе хеш токена, а не сам токен.
	var count, hashLen int
	if err := pool.QueryRow(ctx, "SELECT count(*), coalesce(max(octet_length(token_hash)), 0) FROM sessions").Scan(&count, &hashLen); err != nil {
		t.Fatal(err)
	}
	if count != 1 || hashLen != 32 {
		t.Fatalf("сессий %d, длина хеша %d; ожидали 1 и 32", count, hashLen)
	}
	var tokenStored bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM sessions WHERE token_hash = $1)", []byte(token)).Scan(&tokenStored); err != nil || tokenStored {
		t.Fatalf("сырой токен найден в таблице sessions (err=%v)", err)
	}

	// Текущий пользователь.
	if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), testLogin) {
		t.Fatalf("/me: статус %d, тело %s", rec.Code, rec.Body)
	}

	// Выход отзывает сессию в базе.
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", "", withCookie(token)); rec.Code != http.StatusNoContent {
		t.Fatalf("выход: статус %d", rec.Code)
	}
	var revoked bool
	if err := pool.QueryRow(ctx, "SELECT bool_and(revoked_at IS NOT NULL) FROM sessions").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("сессия не отозвана (err=%v)", err)
	}
	if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("/me после выхода: статус %d, ожидали 401", rec.Code)
	}

	// Неверный пароль и блокировка после серии неудач.
	for i := 0; i < 5; i++ {
		if rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1")); rec.Code != http.StatusUnauthorized {
			t.Fatalf("неверный пароль %d: статус %d, ожидали 401", i+1, rec.Code)
		}
	}
	if rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("после 5 неудач: статус %d, ожидали 429", rec.Code)
	}
}
