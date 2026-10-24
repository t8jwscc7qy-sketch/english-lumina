package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
)

// Все ошибки API имеют один формат: {"error":{"code":"...","message":"..."}}.
func TestErrorFormat(t *testing.T) {
	e := newEnv(t)
	post := func(body string, opts ...reqOpt) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/v1/auth/login", body, opts...)
	}
	tests := []struct {
		name     string
		rec      func() *httptest.ResponseRecorder
		wantCode int
		wantErr  string
	}{
		{"битый запрос", func() *httptest.ResponseRecorder { return post("{not json") }, http.StatusBadRequest, "invalid_request"},
		{"неверный логин или пароль", func() *httptest.ResponseRecorder { return post(loginBody(testLogin, "wrong-password-1")) }, http.StatusUnauthorized, "invalid_credentials"},
		{"чужой Origin", func() *httptest.ResponseRecorder {
			return post(loginBody(testLogin, testPassword), withHeader("Origin", "https://evil.example"))
		}, http.StatusForbidden, "forbidden_origin"},
		{"нет сессии", func() *httptest.ResponseRecorder { return e.do(http.MethodGet, "/api/v1/auth/me", "") }, http.StatusUnauthorized, "unauthorized"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.rec()
			if rec.Code != tt.wantCode {
				t.Fatalf("статус %d, ожидали %d", rec.Code, tt.wantCode)
			}
			var body struct {
				Error struct{ Code, Message string }
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("тело не JSON нужной формы: %v", err)
			}
			if body.Error.Code != tt.wantErr || strings.TrimSpace(body.Error.Message) == "" {
				t.Fatalf("error = %+v, ожидали code %q и непустое message", body.Error, tt.wantErr)
			}
		})
	}
}

func TestErrorFormatThrottleAndBusy(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1"))
	}
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	assertCode(t, rec, http.StatusTooManyRequests, "too_many_attempts")

	store := newFakeStore(t)
	s := newAuthServer(store, auth.NewThrottle(5, time.Minute, time.Now))
	for i := 0; i < cap(s.verifySem); i++ {
		s.verifySem <- struct{}{}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody(testLogin, testPassword)))
	rr := httptest.NewRecorder()
	s.Login(rr, req)
	assertCode(t, rr, http.StatusServiceUnavailable, "busy")
}

func assertCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Error struct{ Code string }
	}
	if rec.Code != status {
		t.Fatalf("статус %d, ожидали %d", rec.Code, status)
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.Error.Code != code {
		t.Fatalf("код ошибки %q (err=%v), ожидали %q", body.Error.Code, err, code)
	}
}
