package httpapi

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Память вместо базы для банка ошибок. Условия повторяют реальные запросы (queries/mistakes.sql):
// одна открытая ошибка на (user, sentence, kind), порядок по сроку, счётчики по времени.
// Откат при ошибке делает fakeStore.InTx (снимок в exerciseSnapshot).

func (f *fakeStore) InsertMistake(_ context.Context, arg db.InsertMistakeParams) (int64, error) {
	for _, m := range f.mistakes {
		if m.UserID == arg.UserID && m.SentenceID == arg.SentenceID && m.Kind == arg.Kind && m.Status == "open" {
			return 0, &pgconn.PgError{Code: "23505"} // как частичный уникальный индекс
		}
	}
	if f.mistakes == nil {
		f.mistakes = map[int64]db.Mistake{}
	}
	f.nextMistakeID++
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	f.mistakes[f.nextMistakeID] = db.Mistake{
		ID: f.nextMistakeID, UserID: arg.UserID, Kind: arg.Kind, SentenceID: arg.SentenceID,
		Topics: slices.Clone(arg.Topics), Payload: arg.Payload, Answer: arg.Answer, Given: arg.Given,
		Status: "open", NextDue: arg.NextDue, FirstWrongAt: arg.FirstWrongAt, SourceExerciseID: arg.SourceExerciseID,
		CreatedAt: now, UpdatedAt: now,
	}
	return f.nextMistakeID, nil
}

func (f *fakeStore) GetOpenMistakeForUpdate(_ context.Context, arg db.GetOpenMistakeForUpdateParams) (db.Mistake, error) {
	if f.missOpenOnce { // имитация гонки: поиск не видит строку, которую вставка потом найдёт
		f.missOpenOnce = false
		return db.Mistake{}, pgx.ErrNoRows
	}
	for _, m := range f.mistakes {
		if m.UserID == arg.UserID && m.SentenceID == arg.SentenceID && m.Kind == arg.Kind && m.Status == "open" {
			return m, nil
		}
	}
	return db.Mistake{}, pgx.ErrNoRows
}

func (f *fakeStore) GetMistakeForUpdate(_ context.Context, arg db.GetMistakeForUpdateParams) (db.Mistake, error) {
	m, ok := f.mistakes[arg.ID]
	if !ok || m.UserID != arg.UserID {
		return db.Mistake{}, pgx.ErrNoRows
	}
	return m, nil
}

func (f *fakeStore) ResetMistake(_ context.Context, arg db.ResetMistakeParams) error {
	m, ok := f.mistakes[arg.ID]
	if !ok {
		return nil
	}
	m.Streak, m.Given, m.NextDue = 0, arg.Given, arg.NextDue
	m.Payload, m.Answer, m.Topics = arg.Payload, arg.Answer, slices.Clone(arg.Topics)
	f.mistakes[arg.ID] = m
	return nil
}

func (f *fakeStore) UpdateMistakeProgress(_ context.Context, arg db.UpdateMistakeProgressParams) error {
	m, ok := f.mistakes[arg.ID]
	if !ok {
		return nil
	}
	m.Streak, m.Status, m.NextDue, m.ClosedAt, m.Given = arg.Streak, arg.Status, arg.NextDue, arg.ClosedAt, arg.Given
	f.mistakes[arg.ID] = m
	return nil
}

// openSorted — открытые ошибки пользователя по сроку, затем по номеру.
func (f *fakeStore) openSorted(userID int64) []db.Mistake {
	var out []db.Mistake
	for _, m := range f.mistakes {
		if m.UserID == userID && m.Status == "open" {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b db.Mistake) int {
		if c := a.NextDue.Time.Compare(b.NextDue.Time); c != 0 {
			return c
		}
		return int(a.ID - b.ID)
	})
	return out
}

func (f *fakeStore) NextDueMistake(_ context.Context, arg db.NextDueMistakeParams) (db.Mistake, error) {
	for _, m := range f.openSorted(arg.UserID) {
		if !m.NextDue.Time.After(arg.Now.Time) {
			return m, nil
		}
	}
	return db.Mistake{}, pgx.ErrNoRows
}

func (f *fakeStore) ListOpenMistakes(_ context.Context, arg db.ListOpenMistakesParams) ([]db.Mistake, error) {
	out := f.openSorted(arg.UserID)
	if int(arg.MaxRows) < len(out) {
		out = out[:arg.MaxRows]
	}
	return out, nil
}

func (f *fakeStore) CountMistakes(_ context.Context, arg db.CountMistakesParams) (db.CountMistakesRow, error) {
	var c db.CountMistakesRow
	for _, m := range f.mistakes {
		if m.UserID != arg.UserID {
			continue
		}
		switch {
		case m.Status == "open":
			c.Open++
			if !m.NextDue.Time.After(arg.Now.Time) {
				c.DueNow++
			}
		case m.Status == "closed" && m.ClosedAt.Valid && !m.ClosedAt.Time.Before(arg.WeekAgo.Time):
			c.ClosedThisWeek++
		}
	}
	return c, nil
}

func (f *fakeStore) CountRepeatsToday(_ context.Context, arg db.CountRepeatsTodayParams) (int64, error) {
	var n int64
	for _, e := range f.exercises {
		if e.UserID == arg.UserID && e.MistakeID != nil && e.AnsweredAt.Valid && !e.AnsweredAt.Time.Before(arg.Since.Time) {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) LastTwoExerciseRepeatFlags(_ context.Context, userID int64) ([]bool, error) {
	var answered []db.Exercise
	for _, e := range f.exercises {
		if e.UserID == userID && e.AnsweredAt.Valid {
			answered = append(answered, e)
		}
	}
	slices.SortFunc(answered, func(a, b db.Exercise) int {
		if c := b.AnsweredAt.Time.Compare(a.AnsweredAt.Time); c != 0 {
			return c
		}
		return int(b.ID - a.ID)
	})
	out := []bool{}
	for _, e := range answered[:min(2, len(answered))] {
		out = append(out, e.MistakeID != nil)
	}
	return out, nil
}

func (f *fakeStore) GetUnansweredRepeat(_ context.Context, arg db.GetUnansweredRepeatParams) (db.Exercise, error) {
	var best db.Exercise
	for _, e := range f.exercises {
		if e.UserID == arg.UserID && e.MistakeID != nil && arg.MistakeID != nil && *e.MistakeID == *arg.MistakeID &&
			!e.AnsweredAt.Valid && e.ID > best.ID {
			best = e
		}
	}
	if best.ID == 0 {
		return db.Exercise{}, pgx.ErrNoRows
	}
	return best, nil
}
