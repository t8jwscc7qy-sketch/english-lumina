package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
)

func withRemote(addr string) reqOpt { return func(r *http.Request) { r.RemoteAddr = addr } }

// Логин вне шаблона users.login отсекается как 400: иначе \u0000 дошёл бы до PostgreSQL и дал 500.
func TestLoginRejectsBadLoginFormat(t *testing.T) {
	e := newEnv(t)
	for name, login := range map[string]string{
		"нулевой байт": "vl\u0000ad",
		"заглавные":    "Vlad",
		"пробел":       "vl ad",
		"кириллица":    "владимир",
	} {
		t.Run(name, func(t *testing.T) {
			rec := e.do(http.MethodPost, "/api/auth/login", loginBody(login, testPassword))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("статус %d, ожидали 400", rec.Code)
			}
		})
	}
}

// Битый UTF-8 в User-Agent не должен ломать вход: PostgreSQL отверг бы такую строку.
func TestLoginInvalidUserAgentUTF8(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodPost, "/api/auth/login", loginBody(testLogin, testPassword),
		withHeader("User-Agent", "Mozilla/5.0 \xff\xfe broken"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("статус %d, ожидали 204", rec.Code)
	}
	for _, s := range e.store.sessions {
		if s.UserAgent == nil || !strings.HasPrefix(*s.UserAgent, "Mozilla/5.0") {
			t.Fatalf("User-Agent сохранён неверно: %v", s.UserAgent)
		}
		for _, r := range *s.UserAgent {
			if r == '�' {
				t.Fatalf("в User-Agent остался недопустимый UTF-8: %q", *s.UserAgent)
			}
		}
	}
}

// Адреса одной сети IPv6 /64 считаются одним клиентом: иначе лимит обходится сменой адреса.
func TestThrottleIPv6SharesPrefix(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.do(http.MethodPost, "/api/auth/login", loginBody(testLogin, "wrong-password-1"),
			withRemote("[2001:db8:1:2:aaaa::"+string(rune('1'+i))+"]:5000"))
	}
	rec := e.do(http.MethodPost, "/api/auth/login", loginBody(testLogin, testPassword),
		withRemote("[2001:db8:1:2:ffff::9]:5000"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("статус %d, ожидали 429", rec.Code)
	}
	// Другая сеть /64 не задета.
	rec = e.do(http.MethodPost, "/api/auth/login", loginBody(testLogin, testPassword),
		withRemote("[2001:db8:1:3::1]:5000"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("соседняя сеть: статус %d, ожидали 204", rec.Code)
	}
}

// Одновременных проверок пароля не больше maxConcurrentVerify: каждая занимает ~19 МиБ.
func TestLoginBusyWhenVerifySlotsTaken(t *testing.T) {
	store := newFakeStore(t)
	s := newAuthServer(store, auth.NewThrottle(5, time.Minute, time.Now))
	for i := 0; i < cap(s.verifySem); i++ {
		s.verifySem <- struct{}{}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody(testLogin, testPassword)))
	req.RemoteAddr = "203.0.113.7:1"
	rec := httptest.NewRecorder()
	s.Login(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("статус %d, ожидали 503", rec.Code)
	}
	if len(store.sessions) != 0 {
		t.Fatal("сессия создана без проверки пароля")
	}

	<-s.verifySem // освободилось место
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody(testLogin, testPassword)))
	req.RemoteAddr = "203.0.113.8:1"
	s.Login(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("после освобождения: статус %d, ожидали 204", rec.Code)
	}
}

// Пароль, по ошибке набранный в поле логина, не должен попасть в журнал.
func TestFailedLoginDoesNotLogLoginField(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	e := newEnv(t)
	const typed = "my-secret-pass-typed-here"
	rec := e.do(http.MethodPost, "/api/auth/login", loginBody(typed, "x"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", rec.Code)
	}
	if strings.Contains(buf.String(), typed) {
		t.Fatalf("значение поля логина попало в журнал: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "203.0.113.7") {
		t.Errorf("в журнале нет IP клиента: %s", buf.String())
	}
}
