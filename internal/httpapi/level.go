package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/levelsuggest"
)

// GetLevelSuggestion предлагает сменить уровень по последним ответам на упражнения своего уровня. Ничего не записывает.
func (s *authServer) GetLevelSuggestion(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	st, err := s.store.GetSettings(ctx, user.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeJSON(w, http.StatusOK, api.LevelSuggestionResponse{}) // онбординг не пройден: уровня ещё нет
		return
	case err != nil:
		s.failExercises(w, fmt.Errorf("настройки для предложения уровня: %w", err))
		return
	}
	now := s.now()
	since := now.AddDate(0, 0, -levelsuggest.HistoryDays)
	newestFirst, err := s.store.ListLevelAnswers(ctx, db.ListLevelAnswersParams{
		UserID: user.ID, Since: ts(since), Topic: "level:" + st.CurrentLevel, MaxRows: levelsuggest.Window,
	})
	if err != nil {
		s.failExercises(w, fmt.Errorf("ответы для предложения уровня: %w", err))
		return
	}
	var dismissedAt *time.Time
	switch d, err := s.store.LastLevelSuggestionDismissal(ctx, user.ID); {
	case err == nil:
		t := d.Time
		dismissedAt = &t
	case !errors.Is(err, pgx.ErrNoRows):
		s.failExercises(w, fmt.Errorf("отказ от предложения уровня: %w", err))
		return
	}
	// Запрос отдаёт новые первыми, правилу нужен порядок от старых к новым.
	oldestFirst := slices.Clone(newestFirst)
	slices.Reverse(oldestFirst)
	sug := levelsuggest.Suggest(st.CurrentLevel, oldestFirst, dismissedAt, now)
	if sug == nil {
		writeJSON(w, http.StatusOK, api.LevelSuggestionResponse{})
		return
	}
	writeJSON(w, http.StatusOK, api.LevelSuggestionResponse{Suggestion: &api.LevelSuggestion{
		Direction: api.LevelSuggestionDirection(sug.Direction),
		From:      api.Level(sug.From),
		To:        api.Level(sug.To),
		Correct:   sug.Correct,
		Total:     sug.Total,
	}})
}

// DismissLevelSuggestion записывает отказ («Пока нет»): предложение прячется на 14 суток. Повтор продлевает срок.
func (s *authServer) DismissLevelSuggestion(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	level := ""
	switch st, err := s.store.GetSettings(ctx, user.ID); {
	case err == nil:
		level = st.CurrentLevel
	case !errors.Is(err, pgx.ErrNoRows):
		s.failExercises(w, fmt.Errorf("настройки для отказа от предложения: %w", err))
		return
	}
	payload, err := json.Marshal(map[string]any{"level": level})
	if err != nil {
		s.failExercises(w, err)
		return
	}
	if _, err := s.store.AppendEvent(ctx, db.AppendEventParams{
		UserID: user.ID, Type: "level.suggestion.dismissed", SchemaVersion: 1, OccurredAt: ts(s.now()), Payload: payload,
	}); err != nil {
		s.failExercises(w, fmt.Errorf("запись отказа от предложения уровня: %w", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
