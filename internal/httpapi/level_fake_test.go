package httpapi

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// fakeLevelAnswer — ответ на упражнение темы для ListLevelAnswers.
type fakeLevelAnswer struct {
	UserID  int64
	Topic   string
	Correct bool
	At      time.Time
}

// ListLevelAnswers отдаёт верность ответов темы пользователя не раньше Since: сначала новые, не больше MaxRows (как запрос).
func (f *fakeStore) ListLevelAnswers(_ context.Context, arg db.ListLevelAnswersParams) ([]bool, error) {
	var rows []fakeLevelAnswer
	for _, a := range f.levelAnswers {
		if a.UserID == arg.UserID && a.Topic == arg.Topic && !a.At.Before(arg.Since.Time) {
			rows = append(rows, a)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].At.After(rows[j].At) })
	if len(rows) > int(arg.MaxRows) {
		rows = rows[:arg.MaxRows]
	}
	out := make([]bool, len(rows))
	for i, r := range rows {
		out[i] = r.Correct
	}
	return out, nil
}

// LastLevelSuggestionDismissal ищет последнее событие отказа среди событий журнала.
func (f *fakeStore) LastLevelSuggestionDismissal(_ context.Context, userID int64) (pgtype.Timestamptz, error) {
	var last time.Time
	found := false
	for _, e := range f.events {
		if e.UserID == userID && e.Type == "level.suggestion.dismissed" && (!found || e.OccurredAt.Time.After(last)) {
			last, found = e.OccurredAt.Time, true
		}
	}
	if !found {
		return pgtype.Timestamptz{}, pgx.ErrNoRows
	}
	return ts(last), nil
}
