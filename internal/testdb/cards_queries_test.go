package testdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

type cardFixture struct {
	q       *db.Queries
	userID  int64
	otherID int64
	lexRun  int64
	lexGo   int64
}

// fixture: два пользователя, два слова; у «run» два источника с уровнями B2 и C1 (общий уровень слова — B2).
func newCardFixture(t *testing.T) cardFixture {
	t.Helper()
	pool := Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	u1, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	// Вторая строка users запрещена (единственный пользователь), поэтому «чужой» пользователь — другой id без строки.
	src1, _ := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1", License: "l", Url: "u", Attribution: "a"})
	src2, _ := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "octanove", Version: "1", License: "l", Url: "u", Attribution: "a"})
	run, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "run", Pos: "verb", SourceID: src1.ID})
	if err != nil {
		t.Fatal(err)
	}
	goID, _ := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "go", Pos: "verb", SourceID: src1.ID})
	for _, c := range []db.UpsertLexemeCefrParams{
		{LexemeID: run, SourceID: src1.ID, Level: "B2", Extra: []byte(`{}`)},
		{LexemeID: run, SourceID: src2.ID, Level: "C1", Extra: []byte(`{}`)},
		{LexemeID: goID, SourceID: src1.ID, Level: "A1", Extra: []byte(`{}`)},
	} {
		if err := q.UpsertLexemeCefr(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	return cardFixture{q: q, userID: u1.ID, otherID: u1.ID + 1000, lexRun: run, lexGo: goID}
}

func (f cardFixture) insert(t *testing.T, lexeme int64, due time.Time) int64 {
	t.Helper()
	id, err := f.q.InsertCard(context.Background(), db.InsertCardParams{
		UserID: f.userID, LexemeID: lexeme, Due: ts(due),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCardQueries(t *testing.T) {
	f := newCardFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

	t.Run("GetLexemeByPair", func(t *testing.T) {
		lx, err := f.q.GetLexemeByPair(ctx, db.GetLexemeByPairParams{Lemma: "run", Pos: "verb"})
		if err != nil || lx.ID != f.lexRun {
			t.Fatalf("run/verb: %+v (%v)", lx, err)
		}
		if _, err := f.q.GetLexemeByPair(ctx, db.GetLexemeByPairParams{Lemma: "run", Pos: "noun"}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("run/noun: ожидали ErrNoRows, получили %v", err)
		}
	})

	runCard := f.insert(t, f.lexRun, now.Add(-time.Hour))
	goCard := f.insert(t, f.lexGo, now.Add(-2*time.Hour))

	t.Run("InsertCard: повтор даёт ErrNoRows, исходная карточка находится", func(t *testing.T) {
		_, err := f.q.InsertCard(ctx, db.InsertCardParams{UserID: f.userID, LexemeID: f.lexRun, Due: ts(now)})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("ожидали ErrNoRows, получили %v", err)
		}
		c, err := f.q.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: f.userID, LexemeID: f.lexRun})
		if err != nil || c.ID != runCard {
			t.Fatalf("карточка %+v (%v), ожидали id %d", c, err, runCard)
		}
	})

	t.Run("NextDueCard: самый ранний срок, уровень слова — самый низкий", func(t *testing.T) {
		c, err := f.q.NextDueCard(ctx, db.NextDueCardParams{UserID: f.userID, Now: ts(now)})
		if err != nil {
			t.Fatal(err)
		}
		if c.ID != goCard || c.Lemma != "go" || c.Pos != "verb" || c.Level != "A1" {
			t.Fatalf("следующая: %+v, ожидали go/A1", c)
		}
		// Убираем go из очереди (срок в будущем): остаётся run, у которого уровни B2 и C1 → B2.
		if err := f.q.UpdateCardState(ctx, db.UpdateCardStateParams{ID: goCard, Due: ts(now.Add(time.Hour)), State: 2}); err != nil {
			t.Fatal(err)
		}
		c, err = f.q.NextDueCard(ctx, db.NextDueCardParams{UserID: f.userID, Now: ts(now)})
		if err != nil || c.ID != runCard || c.Level != "B2" {
			t.Fatalf("следующая: %+v (%v), ожидали run/B2", c, err)
		}
		if err := f.q.UpdateCardState(ctx, db.UpdateCardStateParams{ID: runCard, Due: ts(now.Add(time.Hour)), State: 2}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.q.NextDueCard(ctx, db.NextDueCardParams{UserID: f.userID, Now: ts(now)}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("очередь должна быть пуста, получили %v", err)
		}
	})

	t.Run("CountCards", func(t *testing.T) {
		got, err := f.q.CountCards(ctx, db.CountCardsParams{UserID: f.userID, Now: ts(now)})
		if err != nil || got.Total != 2 || got.Due != 0 {
			t.Fatalf("после переноса сроков: %+v (%v), ожидали всего 2, просрочено 0", got, err)
		}
		got, err = f.q.CountCards(ctx, db.CountCardsParams{UserID: f.userID, Now: ts(now.Add(2 * time.Hour))})
		if err != nil || got.Total != 2 || got.Due != 2 {
			t.Fatalf("через два часа: %+v (%v), ожидали 2 из 2", got, err)
		}
		got, err = f.q.CountCards(ctx, db.CountCardsParams{UserID: f.otherID, Now: ts(now)})
		if err != nil || got.Total != 0 {
			t.Fatalf("у чужого пользователя карточек нет: %+v (%v)", got, err)
		}
	})

	t.Run("GetCardForUpdate: свой найден, чужой нет", func(t *testing.T) {
		c, err := f.q.GetCardForUpdate(ctx, db.GetCardForUpdateParams{ID: runCard, UserID: f.userID})
		if err != nil || c.Lemma != "run" || c.Pos != "verb" || c.LexemeID != f.lexRun {
			t.Fatalf("карточка %+v (%v)", c, err)
		}
		if _, err := f.q.GetCardForUpdate(ctx, db.GetCardForUpdateParams{ID: runCard, UserID: f.otherID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("чужой пользователь: ожидали ErrNoRows, получили %v", err)
		}
		if _, err := f.q.GetCardForUpdate(ctx, db.GetCardForUpdateParams{ID: 999999, UserID: f.userID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("несуществующая: ожидали ErrNoRows, получили %v", err)
		}
	})

	t.Run("UpdateCardState меняет поля и updated_at", func(t *testing.T) {
		before, _ := f.q.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: f.userID, LexemeID: f.lexRun})
		due := now.Add(72 * time.Hour)
		err := f.q.UpdateCardState(ctx, db.UpdateCardStateParams{
			ID: runCard, Due: ts(due), Stability: 4.5, Difficulty: 6.25, ScheduledDays: 3, Reps: 2, Lapses: 1,
			State: 2, LastReview: ts(now), RemainingSteps: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		c, _ := f.q.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: f.userID, LexemeID: f.lexRun})
		if !c.Due.Time.Equal(due) || c.Stability != 4.5 || c.Difficulty != 6.25 || c.ScheduledDays != 3 || c.Reps != 2 ||
			c.Lapses != 1 || c.State != 2 || !c.LastReview.Valid || !c.LastReview.Time.Equal(now) || c.RemainingSteps != 1 {
			t.Fatalf("после обновления: %+v", c)
		}
		if !c.UpdatedAt.Time.After(before.UpdatedAt.Time) && !c.UpdatedAt.Time.Equal(before.UpdatedAt.Time) {
			t.Fatalf("updated_at: %v → %v", before.UpdatedAt.Time, c.UpdatedAt.Time)
		}
	})

	t.Run("ListCardLexemeIDs: только слова пользователя", func(t *testing.T) {
		ids, err := f.q.ListCardLexemeIDs(ctx, db.ListCardLexemeIDsParams{UserID: f.userID, Ids: []int64{f.lexRun, f.lexGo, 999999}})
		if err != nil || len(ids) != 2 {
			t.Fatalf("ids %v (%v)", ids, err)
		}
		ids, err = f.q.ListCardLexemeIDs(ctx, db.ListCardLexemeIDsParams{UserID: f.otherID, Ids: []int64{f.lexRun}})
		if err != nil || len(ids) != 0 {
			t.Fatalf("у чужого пользователя ids %v (%v)", ids, err)
		}
	})
}
