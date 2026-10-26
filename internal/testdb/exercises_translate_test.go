package testdb

import (
	"context"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Миграция 00013: вид translate проходит проверку kind; откат падает, пока такие упражнения есть.
func TestMigration00013TranslateKind(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)

	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "translate", Topics: []string{"skill:translate"}, Payload: []byte(`{}`), Answer: []byte(`{}`)})
	if err != nil {
		t.Fatalf("упражнение translate: %v", err)
	}
	if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "typo", Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
		t.Fatal("неизвестный вид должен отклоняться")
	}

	// Банк ошибок: у mistakes своя проверка вида, она тоже должна принимать translate.
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	sent, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "We eat red apples every morning.", Author: "a", WordCount: 6, SourceID: src.ID})
	if err != nil {
		t.Fatal(err)
	}
	insertMistake := `INSERT INTO mistakes (user_id, kind, sentence_id, topics, payload, answer, given, next_due, first_wrong_at)
		VALUES ($1, 'translate', $2, ARRAY['skill:translate'], '{}', '{}', 'x', now(), now())`
	if _, err := pool.Exec(ctx, insertMistake, u.ID, sent.ID); err != nil {
		t.Fatalf("ошибка вида translate в банке: %v", err)
	}

	down := downSQL(t, "00013_exercises_translate.sql")
	_, err = pool.Exec(ctx, down)
	if err == nil || !strings.Contains(err.Error(), "откат 00013 невозможен") {
		t.Fatalf("откат с упражнением translate должен падать с понятным текстом, получили %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM exercises WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	// Ошибка translate в банке тоже мешает откату.
	if _, err := pool.Exec(ctx, down); err == nil || !strings.Contains(err.Error(), "откат 00013 невозможен") {
		t.Fatalf("откат с ошибкой translate в банке должен падать, получили %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM mistakes WHERE kind = 'translate'"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("откат без упражнений и ошибок translate: %v", err)
	}
	if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "translate", Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
		t.Fatal("после отката вид translate запрещён")
	}
}

func downSQL(t *testing.T, name string) string {
	t.Helper()
	for _, m := range readMigrations(t) {
		if m.name == name {
			return m.down
		}
	}
	t.Fatalf("миграция %s не найдена", name)
	return ""
}

// Для «перевода» верны переводы любой длины (4–10 слов): «I said I'd try» и «I said I would try» — один смысл.
func TestListTranslateAlternatives(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	sent := func(lang string, tid int64, text string, wc int16) int64 {
		s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: lang, TatoebaID: tid, Text: text, Author: "a", WordCount: wc, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	pair := func(en, ru int64) {
		if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en, RuID: ru, SourceID: src.ID}); err != nil {
			t.Fatal(err)
		}
	}
	ru := sent("ru", 1, "Я сказал, что попробую.", 4)
	ruOther := sent("ru", 2, "Другой перевод", 2)
	mine := sent("en", 10, "I said I'd try.", 4)
	longer := sent("en", 11, "I said I would try.", 5)
	same := sent("en", 12, "I said that I'd try.", 5)
	short := sent("en", 13, "I tried.", 2)
	tooLong := sent("en", 14, "I said to everyone in the room that I would certainly try.", 12)
	unrelated := sent("en", 15, "Unrelated sentence about cats here.", 5)
	for _, e := range []int64{mine, longer, same, short, tooLong} {
		pair(e, ru)
	}
	pair(unrelated, ruOther)

	rows, err := q.ListTranslateAlternatives(ctx, db.ListTranslateAlternativesParams{EnID: mine, RuID: ru, MaxRows: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, r := range rows {
		got = append(got, r.ID)
	}
	want := []int64{longer, same} // другая длина берётся, 2 и 12 слов нет, само предложение и чужой перевод нет
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("альтернативы %v, ждали %v", got, want)
	}
	rows, err = q.ListTranslateAlternatives(ctx, db.ListTranslateAlternativesParams{EnID: mine, RuID: ru, MaxRows: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("лимит 1: %v %v", rows, err)
	}
}
