package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{"база доступна", nil, http.StatusOK, "ok"},
		{"база недоступна", errors.New("connection refused"), http.StatusServiceUnavailable, "db unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			NewHandler(fakePinger{err: tt.pingErr}, nil, nil).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("статус = %d, ожидали %d", rec.Code, tt.wantStatus)
			}
			var body struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("тело не JSON: %v", err)
			}
			if body.Status != tt.wantBody {
				t.Fatalf("status = %q, ожидали %q", body.Status, tt.wantBody)
			}
		})
	}
}

func TestHealthRejectsPost(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(fakePinger{}, nil, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("статус = %d, ожидали 405", rec.Code)
	}
}
