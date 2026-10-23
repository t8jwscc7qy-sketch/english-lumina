package httpapi

import (
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

// Ручки карточек реализуются в следующих задачах; пока отвечают 501, чтобы пакет собирался.
func notImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, api.ErrorErrorCodeInternal, "Пока не реализовано")
}

func (s *authServer) AddCard(w http.ResponseWriter, _ *http.Request)      { notImplemented(w) }
func (s *authServer) GetCardQueue(w http.ResponseWriter, _ *http.Request) { notImplemented(w) }
func (s *authServer) ReviewCard(w http.ResponseWriter, _ *http.Request)   { notImplemented(w) }
