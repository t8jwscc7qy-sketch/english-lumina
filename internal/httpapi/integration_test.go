package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

// Весь путь на настоящей базе: настоящие запросы sqlc, настоящий пароль и токен.
func TestAuthFlowOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
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

// Настройки на настоящей базе: upsert sqlc, тип date и CHECK таблицы settings.
func TestSettingsFlowOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
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
	token := login(t, e)

	if rec := e.do(http.MethodGet, settingsPath, "", withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("GET до онбординга: статус %d, ожидали 404", rec.Code)
	}

	date := time.Now().AddDate(0, 8, 0).Format(dateLayout)
	body := settingsBody(t, func(m map[string]any) { m["targetDate"] = date })
	rec := e.do(http.MethodPut, settingsPath, body, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: статус %d, тело %s", rec.Code, rec.Body)
	}
	got := e.do(http.MethodGet, settingsPath, "", withCookie(token))
	if got.Code != http.StatusOK || got.Body.String() != rec.Body.String() {
		t.Fatalf("GET: статус %d, тело %s; ожидали %s", got.Code, got.Body, rec.Body)
	}

	// Повторный PUT заменяет строку, а не добавляет вторую.
	body = settingsBody(t, func(m map[string]any) { m["dailyMinutes"] = 30; m["targetDate"] = nil })
	if rec := e.do(http.MethodPut, settingsPath, body, withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("второй PUT: статус %d, тело %s", rec.Code, rec.Body)
	}
	var rows, minutes int
	var hasDate bool
	if err := pool.QueryRow(ctx, "SELECT count(*), max(daily_min_minutes), bool_or(target_date IS NOT NULL) FROM settings").Scan(&rows, &minutes, &hasDate); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || minutes != 30 || hasDate {
		t.Fatalf("строк %d, минут %d, дата есть %v; ожидали 1, 30, false", rows, minutes, hasDate)
	}

	// Цель ниже уровня отсекается до базы и ничего не меняет.
	bad := settingsBody(t, func(m map[string]any) { m["currentLevel"] = "C1"; m["targetLevel"] = "A1" })
	if rec := e.do(http.MethodPut, settingsPath, bad, withCookie(token)); rec.Code != http.StatusBadRequest {
		t.Fatalf("цель ниже уровня: статус %d, ожидали 400", rec.Code)
	}
}
