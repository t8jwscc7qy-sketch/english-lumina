// Package httpapi содержит HTTP-обработчики сервера.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Pinger проверяет доступность базы данных (его реализует *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

const pingTimeout = 2 * time.Second

// NewHandler собирает маршруты сервера.
func NewHandler(db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health(db))
	return mux
}

func health(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			slog.Error("проверка базы не прошла", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("запись ответа", "err", err)
	}
}
