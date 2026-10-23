package httpapi

import (
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

// Заглушки: ручки упражнений появятся в следующих задачах. Вход проверяется уже сейчас.

func (s *authServer) GetNextExercise(w http.ResponseWriter, r *http.Request) { s.exerciseStub(w, r) }

func (s *authServer) AnswerExercise(w http.ResponseWriter, r *http.Request) { s.exerciseStub(w, r) }

func (s *authServer) ListExerciseTopics(w http.ResponseWriter, r *http.Request) { s.exerciseStub(w, r) }

func (s *authServer) exerciseStub(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	writeError(w, http.StatusNotImplemented, api.ErrorErrorCodeNotImplemented, "Упражнения ещё не готовы")
}
