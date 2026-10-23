package httpapi

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Память вместо базы для тестов ручек карточек. InTx делает снимок и при ошибке возвращает его:
// так тесты видят откат так же, как в настоящей транзакции.

func (f *fakeStore) addLexeme(id int64, lemma, pos, level string) {
	if f.lexemesByID == nil {
		f.lexemesByID = map[int64]db.Lexeme{}
		f.cardLevels = map[int64]string{}
	}
	f.lexemesByID[id] = db.Lexeme{ID: id, Lemma: lemma, Pos: pos}
	f.cardLevels[id] = level
}

func (f *fakeStore) GetLexemeByPair(_ context.Context, arg db.GetLexemeByPairParams) (db.Lexeme, error) {
	for _, lx := range f.lexemesByID {
		if lx.Lemma == arg.Lemma && lx.Pos == arg.Pos {
			return lx, nil
		}
	}
	return db.Lexeme{}, pgx.ErrNoRows
}

func (f *fakeStore) InsertCard(_ context.Context, arg db.InsertCardParams) (int64, error) {
	for _, c := range f.cards {
		if c.UserID == arg.UserID && c.LexemeID == arg.LexemeID {
			return 0, pgx.ErrNoRows
		}
	}
	if f.cards == nil {
		f.cards = map[int64]db.Card{}
	}
	f.nextCardID++
	f.cards[f.nextCardID] = db.Card{
		ID: f.nextCardID, UserID: arg.UserID, LexemeID: arg.LexemeID, Due: arg.Due, Stability: arg.Stability,
		Difficulty: arg.Difficulty, ScheduledDays: arg.ScheduledDays, Reps: arg.Reps, Lapses: arg.Lapses,
		State: arg.State, LastReview: arg.LastReview, RemainingSteps: arg.RemainingSteps,
	}
	return f.nextCardID, nil
}

func (f *fakeStore) GetCardByLexeme(_ context.Context, arg db.GetCardByLexemeParams) (db.Card, error) {
	for _, c := range f.cards {
		if c.UserID == arg.UserID && c.LexemeID == arg.LexemeID {
			return c, nil
		}
	}
	return db.Card{}, pgx.ErrNoRows
}

func (f *fakeStore) GetCardForUpdate(_ context.Context, arg db.GetCardForUpdateParams) (db.GetCardForUpdateRow, error) {
	c, ok := f.cards[arg.ID]
	if !ok || c.UserID != arg.UserID {
		return db.GetCardForUpdateRow{}, pgx.ErrNoRows
	}
	lx := f.lexemesByID[c.LexemeID]
	return db.GetCardForUpdateRow{
		ID: c.ID, UserID: c.UserID, LexemeID: c.LexemeID, Due: c.Due, Stability: c.Stability, Difficulty: c.Difficulty,
		ScheduledDays: c.ScheduledDays, Reps: c.Reps, Lapses: c.Lapses, State: c.State, LastReview: c.LastReview,
		RemainingSteps: c.RemainingSteps, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Lemma: lx.Lemma, Pos: lx.Pos,
	}, nil
}

func (f *fakeStore) UpdateCardState(_ context.Context, arg db.UpdateCardStateParams) error {
	c, ok := f.cards[arg.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	c.Due, c.Stability, c.Difficulty, c.ScheduledDays, c.Reps = arg.Due, arg.Stability, arg.Difficulty, arg.ScheduledDays, arg.Reps
	c.Lapses, c.State, c.LastReview, c.RemainingSteps = arg.Lapses, arg.State, arg.LastReview, arg.RemainingSteps
	f.cards[arg.ID] = c
	return nil
}

func (f *fakeStore) NextDueCard(_ context.Context, arg db.NextDueCardParams) (db.NextDueCardRow, error) {
	var best *db.Card
	for _, c := range f.cards {
		if c.UserID != arg.UserID || c.Due.Time.After(arg.Now.Time) {
			continue
		}
		if best == nil || c.Due.Time.Before(best.Due.Time) || (c.Due.Time.Equal(best.Due.Time) && c.ID < best.ID) {
			c := c
			best = &c
		}
	}
	if best == nil {
		return db.NextDueCardRow{}, pgx.ErrNoRows
	}
	lx := f.lexemesByID[best.LexemeID]
	return db.NextDueCardRow{
		ID: best.ID, UserID: best.UserID, LexemeID: best.LexemeID, Due: best.Due, Stability: best.Stability,
		Difficulty: best.Difficulty, ScheduledDays: best.ScheduledDays, Reps: best.Reps, Lapses: best.Lapses,
		State: best.State, LastReview: best.LastReview, RemainingSteps: best.RemainingSteps,
		Lemma: lx.Lemma, Pos: lx.Pos, Level: f.cardLevels[best.LexemeID],
	}, nil
}

func (f *fakeStore) CountCards(_ context.Context, arg db.CountCardsParams) (db.CountCardsRow, error) {
	var out db.CountCardsRow
	for _, c := range f.cards {
		if c.UserID != arg.UserID {
			continue
		}
		out.Total++
		if !c.Due.Time.After(arg.Now.Time) {
			out.Due++
		}
	}
	return out, nil
}

func (f *fakeStore) ListCardLexemeIDs(_ context.Context, arg db.ListCardLexemeIDsParams) ([]int64, error) {
	var out []int64
	for _, c := range f.cards {
		if c.UserID == arg.UserID && slices.Contains(arg.Ids, c.LexemeID) {
			out = append(out, c.LexemeID)
		}
	}
	return out, nil
}

func (f *fakeStore) AppendEvent(_ context.Context, arg db.AppendEventParams) (int64, error) {
	if f.failEvents {
		return 0, errors.New("журнал недоступен")
	}
	if f.eventKeys == nil {
		f.eventKeys = map[string]bool{}
	}
	if arg.IdempotencyKey != nil {
		key := fmt.Sprintf("%d:%s", arg.UserID, *arg.IdempotencyKey)
		if f.eventKeys[key] {
			return 0, pgx.ErrNoRows
		}
		f.eventKeys[key] = true
	}
	id := int64(len(f.events) + 1)
	f.events = append(f.events, db.Event{
		ID: id, UserID: arg.UserID, Type: arg.Type, SchemaVersion: arg.SchemaVersion, OccurredAt: arg.OccurredAt,
		Payload: arg.Payload, IdempotencyKey: arg.IdempotencyKey,
	})
	return id, nil
}

func (f *fakeStore) InTx(_ context.Context, fn func(AuthStore) error) error {
	cards, events, keys, next := maps.Clone(f.cards), slices.Clone(f.events), maps.Clone(f.eventKeys), f.nextCardID
	if err := fn(f); err != nil {
		f.cards, f.events, f.eventKeys, f.nextCardID = cards, events, keys, next
		return err
	}
	return nil
}
