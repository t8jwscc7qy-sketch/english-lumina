package testdb

import (
	"context"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestSheetQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	// Избранное: первый вызов меняет состояние, повторный — нет; снятие тоже один раз.
	if n, err := q.SheetSetFavorite(ctx, db.SheetSetFavoriteParams{UserID: user.ID, SheetID: "to-be", At: ts(now)}); err != nil || n != 1 {
		t.Fatalf("поставить: %d %v", n, err)
	}
	if n, err := q.SheetSetFavorite(ctx, db.SheetSetFavoriteParams{UserID: user.ID, SheetID: "to-be", At: ts(now.Add(time.Hour))}); err != nil || n != 0 {
		t.Fatalf("поставить повторно: %d %v", n, err)
	}
	if n, err := q.SheetClearFavorite(ctx, db.SheetClearFavoriteParams{UserID: user.ID, SheetID: "to-be"}); err != nil || n != 1 {
		t.Fatalf("снять: %d %v", n, err)
	}
	if n, err := q.SheetClearFavorite(ctx, db.SheetClearFavoriteParams{UserID: user.ID, SheetID: "to-be"}); err != nil || n != 0 {
		t.Fatalf("снять повторно: %d %v", n, err)
	}
	if n, err := q.SheetClearFavorite(ctx, db.SheetClearFavoriteParams{UserID: user.ID, SheetID: "never"}); err != nil || n != 0 {
		t.Fatalf("снять у шпаргалки без состояния: %d %v", n, err)
	}

	// «Проверено» ставится один раз, время не перезаписывается.
	first := now.Add(-time.Minute)
	if n, err := q.SheetSetReviewed(ctx, db.SheetSetReviewedParams{UserID: user.ID, SheetID: "to-be", At: ts(first), Hash: "h1"}); err != nil || n != 1 {
		t.Fatalf("проверено: %d %v", n, err)
	}
	if n, err := q.SheetSetReviewed(ctx, db.SheetSetReviewedParams{UserID: user.ID, SheetID: "to-be", At: ts(now), Hash: "h1"}); err != nil || n != 0 {
		t.Fatalf("проверено повторно: %d %v", n, err)
	}
	// Текст изменился (другой хеш): проверка ставится заново.
	if n, err := q.SheetSetReviewed(ctx, db.SheetSetReviewedParams{UserID: user.ID, SheetID: "to-be", At: ts(now), Hash: "h2"}); err != nil || n != 1 {
		t.Fatalf("проверено после правки текста: %d %v", n, err)
	}
	first = now
	if err := q.SheetTouchOpened(ctx, db.SheetTouchOpenedParams{UserID: user.ID, SheetID: "articles", At: ts(now)}); err != nil {
		t.Fatal(err)
	}
	states, err := q.SheetStates(ctx, user.ID)
	if err != nil || len(states) != 2 {
		t.Fatalf("состояния: %+v %v", states, err)
	}
	for _, s := range states {
		if s.SheetID == "to-be" && (s.FavoriteAt.Valid || !s.ReviewedAt.Valid || !s.ReviewedAt.Time.Equal(first.Truncate(time.Microsecond))) {
			t.Fatalf("to-be: %+v", s)
		}
	}
	if st, err := q.SheetStates(ctx, user.ID+1); err != nil || len(st) != 0 {
		t.Fatalf("чужие состояния не видны: %+v %v", st, err)
	}

	// Статусы слов: лемма в любой части речи, состояние изучения и карточка пользователя.
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1.5", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	lex := func(lemma, pos string) int64 {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: pos, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	goID, runN := lex("go", "verb"), lex("run", "noun")
	_ = lex("run", "verb")
	_ = lex("see", "verb")
	if _, err := pool.Exec(ctx, `INSERT INTO cards (user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, remaining_steps) VALUES ($1, $2, now(), 1, 1, 0, 5, 3, 2, 0)`, user.ID, goID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO word_learning (user_id, lexeme_id, status, source, learned_at) VALUES ($1, $2, 'learned', 'lesson', now())`, user.ID, runN); err != nil {
		t.Fatal(err)
	}
	rows, err := q.SheetWordStatuses(ctx, db.SheetWordStatusesParams{UserID: user.ID, Lemmas: []string{"go", "run", "see", "missing"}, Poss: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	type st struct {
		learning *string
		card     bool
		lapses   int32
	}
	byLemma := map[string][]st{}
	for _, r := range rows {
		byLemma[r.Lemma] = append(byLemma[r.Lemma], st{r.LearningStatus, r.HasCard, r.Lapses})
	}
	if g := byLemma["go"]; len(g) != 1 || !g[0].card || g[0].lapses != 3 || g[0].learning != nil {
		t.Fatalf("go: %+v", g)
	}
	if r := byLemma["run"]; len(r) != 2 {
		t.Fatalf("у run два слова: %+v", r)
	}
	if s := byLemma["see"]; len(s) != 1 || s[0].card || s[0].learning != nil {
		t.Fatalf("see: %+v", s)
	}
	// Фильтр по части речи: у run остаётся только существительное с состоянием «изучено».
	only, err := q.SheetWordStatuses(ctx, db.SheetWordStatusesParams{UserID: user.ID, Lemmas: []string{"run"}, Poss: []string{"noun"}})
	if err != nil || len(only) != 1 || only[0].LearningStatus == nil || *only[0].LearningStatus != "learned" {
		t.Fatalf("run/noun: %+v %v", only, err)
	}
	if _, ok := byLemma["missing"]; ok {
		t.Fatal("несуществующей леммы нет")
	}
}
