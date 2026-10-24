package httpapi

import (
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

// ListMistakes — заглушка контракта GET /api/v1/mistakes: настоящая ручка появится в следующем шаге банка ошибок.
func (s *authServer) ListMistakes(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	writeError(w, http.StatusNotImplemented, api.ErrorErrorCodeInternal, "Банк ошибок ещё не готов")
}
