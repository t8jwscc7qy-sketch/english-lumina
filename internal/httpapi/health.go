// Package httpapi содержит HTTP-обработчики сервера.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
)

// Pinger проверяет доступность базы данных (его реализует *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

const pingTimeout = 2 * time.Second

// NewHandler собирает маршруты сервера: /health открыт, /api/auth/* описан в api/openapi.yaml.
func NewHandler(db Pinger, store AuthStore, throttle *auth.Throttle) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health(db))
	return api.HandlerWithOptions(newAuthServer(store, throttle), api.StdHTTPServerOptions{
		BaseRouter:  mux,
		Middlewares: []api.MiddlewareFunc{checkOrigin},
	})
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

// writeError отвечает ошибкой единого формата: {"error":{"code":"...","message":"..."}}.
func writeError(w http.ResponseWriter, status int, code api.ErrorErrorCode, message string) {
	var body api.Error
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}
