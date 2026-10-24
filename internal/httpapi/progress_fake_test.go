package httpapi

import (
	"context"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// fakeActivity — событие занятий пользователя для фейка ListActivityEvents.
type fakeActivity struct {
	UserID int64
	Row    db.ListActivityEventsRow
}

// ListActivityEvents отдаёт события пользователя не раньше Since в порядке времени (как запрос в базе).
func (f *fakeStore) ListActivityEvents(_ context.Context, arg db.ListActivityEventsParams) ([]db.ListActivityEventsRow, error) {
	var out []db.ListActivityEventsRow
	for _, a := range f.activity {
		if a.UserID == arg.UserID && !a.Row.OccurredAt.Time.Before(arg.Since.Time) {
			out = append(out, a.Row)
		}
	}
	sortActivity(out)
	return out, nil
}

func sortActivity(rows []db.ListActivityEventsRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].OccurredAt.Time.Before(rows[j-1].OccurredAt.Time); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
