package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/settings"
)

const (
	maxSettingsBody = 4 << 10
	dateLayout      = "2006-01-02"
)

func (s *authServer) GetSettings(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	row, err := s.store.GetSettings(r.Context(), user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeOnboardingRequired, "Сначала настрой цель и уровень")
		return
	}
	if err != nil {
		s.failSettings(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPISettings(row))
}

func (s *authServer) PutSettings(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req api.Settings
	r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный запрос")
		return
	}

	in := settings.Input{
		CurrentLevel: string(req.CurrentLevel),
		TargetLevel:  string(req.TargetLevel),
		DailyMinutes: req.DailyMinutes,
		Timezone:     req.Timezone,
	}
	if req.TargetDate != nil {
		d, err := time.Parse(dateLayout, *req.TargetDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Дата цели должна быть в виде ГГГГ-ММ-ДД")
			return
		}
		in.TargetDate = &d
	}
	valid, err := settings.Validate(in, time.Now())
	if err != nil {
		var ve *settings.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, ve.Message)
			return
		}
		s.failSettings(w, err)
		return
	}

	row, err := s.store.UpsertSettings(r.Context(), db.UpsertSettingsParams{
		UserID:          user.ID,
		CurrentLevel:    valid.CurrentLevel,
		TargetLevel:     valid.TargetLevel,
		TargetDate:      valid.TargetDate,
		DailyMinMinutes: valid.DailyMinutes,
		Timezone:        valid.Timezone,
	})
	if err != nil {
		s.failSettings(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPISettings(row))
}

func toAPISettings(row db.Setting) api.Settings {
	out := api.Settings{
		CurrentLevel: api.Level(row.CurrentLevel),
		TargetLevel:  api.Level(row.TargetLevel),
		DailyMinutes: int(row.DailyMinMinutes),
		Timezone:     row.Timezone,
	}
	if row.TargetDate != nil {
		d := row.TargetDate.Format(dateLayout)
		out.TargetDate = &d
	}
	return out
}

func (s *authServer) failSettings(w http.ResponseWriter, err error) {
	slog.Error("ошибка настроек", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}
