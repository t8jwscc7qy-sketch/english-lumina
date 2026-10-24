package testdb

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestExercisesOrderKind(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I went home yesterday.", Author: "a", WordCount: 4, SourceID: src.ID})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(kind string) error {
		_, err := pool.Exec(ctx, `INSERT INTO exercises (user_id, kind, sentence_id, topics, payload, answer)
			VALUES ($1, $2, $3, ARRAY['skill:order'], '{}', '{}')`, u.ID, kind, s.ID)
		return err
	}
	if err := insert("order"); err != nil {
		t.Fatalf("вид order должен быть допустим: %v", err)
	}
	if err := insert("gap"); err != nil {
		t.Fatalf("вид gap должен остаться допустимым: %v", err)
	}
	if err := insert("x"); err == nil {
		t.Fatal("неизвестный вид должен отклоняться")
	}

	// Откат только этой миграции: с упражнением вида order он падает с понятной ошибкой, без него проходит.
	var down string
	for _, m := range readMigrations(t) {
		if strings.HasPrefix(m.name, "00008") {
			down = m.down
		}
	}
	if down == "" {
		t.Fatal("миграция 00008 не найдена")
	}
	if _, err := pool.Exec(ctx, down); err == nil {
		t.Fatal("откат с упражнением вида order должен падать")
	} else if !strings.Contains(err.Error(), "order") {
		t.Fatalf("ошибка отката должна называть причину: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM exercises WHERE kind = 'order'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("откат без упражнений order: %v", err)
	}
	if err := insert("order"); err == nil {
		t.Fatal("после отката вид order не должен быть допустим")
	}
}

func TestListEnglishAlternatives(t *testing.T) {
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
	ru := sent("ru", 1, "Вчера я пошёл домой", 4)
	ruOther := sent("ru", 2, "Другой перевод", 2)
	a := sent("en", 10, "I went home yesterday.", 4)
	b := sent("en", 11, "Yesterday I went home.", 4)
	short := sent("en", 12, "I went.", 2)
	long := sent("en", 13, "Yesterday after the long meeting I went straight home alone tonight.", 11)
	unrelated := sent("en", 14, "Unrelated sentence about cats here.", 5)
	for _, e := range []int64{a, b, short, long} {
		pair(e, ru)
	}
	pair(unrelated, ruOther)

	rows, err := q.ListEnglishAlternatives(ctx, db.ListEnglishAlternativesParams{EnID: a, RuID: ru, MaxRows: 5})
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if !slices.Equal(ids, []int64{b}) {
		t.Fatalf("альтернативы %v, ожидали только %d (само предложение, короткое, длинное и чужое отбрасываются)", ids, b)
	}
	if rows[0].Text != "Yesterday I went home." {
		t.Fatalf("текст: %q", rows[0].Text)
	}

	c := sent("en", 15, "I walked home yesterday.", 4)
	pair(c, ru)
	rows, err = q.ListEnglishAlternatives(ctx, db.ListEnglishAlternativesParams{EnID: a, RuID: ru, MaxRows: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("лимит 1: %+v (%v)", rows, err)
	}
}

// Альтернативы берутся только по показанному русскому переводу и только с тем же числом слов:
// иначе лимит съедают чужие предложения, а вопрос засчитывается как ответ на утверждение.
func TestListEnglishAlternativesShownTranslationAndWordCount(t *testing.T) {
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
	ru1 := sent("ru", 1, "Он сейчас здесь", 3)
	ru2 := sent("ru", 2, "Он здесь сейчас, да", 4)
	a := sent("en", 10, "He is here now.", 4)
	five := sent("en", 11, "He is right here now.", 5) // другое число слов, id меньше нужного
	question := sent("en", 12, "Is he here now?", 4)   // связан только со вторым переводом
	good := sent("en", 13, "Now he is here.", 4)
	pair(a, ru1)
	pair(a, ru2)
	pair(five, ru1)
	pair(question, ru2)
	pair(good, ru1)

	rows, err := q.ListEnglishAlternatives(ctx, db.ListEnglishAlternativesParams{EnID: a, RuID: ru1, MaxRows: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != good {
		t.Fatalf("ожидали только %d (тот же перевод и 4 слова), получили %+v", good, rows)
	}
}
