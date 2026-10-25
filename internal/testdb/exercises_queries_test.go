package testdb

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestExerciseQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	// Своя схема: фикстура карточек использует другой пул, поэтому всё нужное создаём здесь.
	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	other := u.ID + 1000

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
	good := sent("en", 1, "I have a very good dog", 6)
	ruLong := sent("ru", 2, "У меня есть очень хорошая собака", 6)
	ruShort := sent("ru", 3, "У меня хорошая собака", 4)
	pair(good, ruLong)
	pair(good, ruShort)
	tooShort := sent("en", 4, "Run now", 2)
	pair(tooShort, sent("ru", 5, "Беги сейчас", 2))
	tooLong := sent("en", 6, "one two three four five six seven eight nine ten eleven twelve thirteen", 13)
	pair(tooLong, sent("ru", 7, "один два три четыре пять шесть семь восемь девять десять одиннадцать двенадцать тринадцать", 13))
	sent("en", 8, "No translation for this sentence", 5)
	good2 := sent("en", 9, "She has a small red car", 6)
	pair(good2, sent("ru", 10, "У неё маленькая красная машина", 5))

	t.Run("SampleSentences: 4-12 слов, с переводом, самый короткий, по возрастанию id от from_id", func(t *testing.T) {
		rows, err := q.SampleSentences(ctx, db.SampleSentencesParams{FromID: 0, MaxRows: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 || rows[0].ID != good || rows[1].ID != good2 {
			t.Fatalf("получили %+v, ожидали предложения %d и %d", rows, good, good2)
		}
		if rows[0].RuID != ruShort || rows[0].RuText != "У меня хорошая собака" || rows[0].TatoebaID != 1 {
			t.Fatalf("перевод должен быть коротким: %+v", rows[0])
		}
		rows, err = q.SampleSentences(ctx, db.SampleSentencesParams{FromID: good + 1, MaxRows: 10})
		if err != nil || len(rows) != 1 || rows[0].ID != good2 {
			t.Fatalf("от good+1: %+v (%v)", rows, err)
		}
		rows, err = q.SampleSentences(ctx, db.SampleSentencesParams{FromID: 0, MaxRows: 1})
		if err != nil || len(rows) != 1 {
			t.Fatalf("лимит 1: %+v (%v)", rows, err)
		}
	})

	t.Run("SentenceLevels: уровень слова, место в частотах, пустые значения", func(t *testing.T) {
		cefr, _ := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1", License: "l", Url: "u", Attribution: "a"})
		oct, _ := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "octanove", Version: "1", License: "l", Url: "u", Attribution: "a"})
		fq, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "frequencywords", Version: "2018", License: "l", Url: "u", Attribution: "a"})
		if err != nil {
			t.Fatal(err)
		}
		run, _ := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "Run", Pos: "verb", SourceID: cefr.ID})
		goL, _ := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "go", Pos: "verb", SourceID: cefr.ID})
		for _, c := range []db.UpsertLexemeCefrParams{
			{LexemeID: run, SourceID: cefr.ID, Level: "B2", Extra: []byte(`{}`)},
			{LexemeID: run, SourceID: oct.ID, Level: "C1", Extra: []byte(`{}`)},
			{LexemeID: goL, SourceID: cefr.ID, Level: "A1", Extra: []byte(`{}`)},
		} {
			if err := q.UpsertLexemeCefr(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		for i, w := range []string{"go", "the"} {
			if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: fq.ID, Word: w, Count: 100, Rank: int32(i + 1)}); err != nil {
				t.Fatal(err)
			}
		}
		// Пока импорт не завершён, частоты не видны.
		rows, err := q.SentenceLevels(ctx, []string{"go"})
		if err != nil || len(rows) != 1 || rows[0].Level == nil || *rows[0].Level != "A1" || rows[0].Rank != nil {
			t.Fatalf("до завершения импорта: %+v (%v)", rows, err)
		}
		n := int32(2)
		if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: fq.ID, RowCount: &n}); err != nil {
			t.Fatal(err)
		}
		rows, err = q.SentenceLevels(ctx, []string{"the", "run", "go", "zzz", "go"})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]db.SentenceLevelsRow{}
		for _, r := range rows {
			got[r.Word] = r
		}
		if len(rows) != 4 {
			t.Fatalf("слова без повторов: %+v", rows)
		}
		if r := got["go"]; r.Level == nil || *r.Level != "A1" || r.Rank == nil || *r.Rank != 1 {
			t.Fatalf("go: %+v", r)
		}
		if r := got["run"]; r.Level == nil || *r.Level != "B2" || r.Rank != nil {
			t.Fatalf("run (лемма Run, уровни B2 и C1 → B2): %+v", r)
		}
		if r := got["the"]; r.Level != nil || r.Rank == nil || *r.Rank != 2 {
			t.Fatalf("the: %+v", r)
		}
		if r := got["zzz"]; r.Level != nil || r.Rank != nil {
			t.Fatalf("zzz: %+v", r)
		}
	})

	t.Run("упражнение: вставка, блокировка, ответ, использованные предложения", func(t *testing.T) {
		id, err := q.InsertExercise(ctx, db.InsertExerciseParams{
			UserID: u.ID, Kind: "gap", SentenceID: &good, Topics: []string{"grammar:articles", "level:A1"},
			Payload: []byte(`{"p":1}`), Answer: []byte(`{"a":["a"]}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		ex, err := q.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: u.ID})
		if err != nil || ex.SentenceID == nil || *ex.SentenceID != good || ex.AnsweredAt.Valid || !slices.Equal(ex.Topics, []string{"grammar:articles", "level:A1"}) {
			t.Fatalf("упражнение %+v (%v)", ex, err)
		}
		if _, err := q.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: other}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("чужое упражнение: ожидали ErrNoRows, получили %v", err)
		}

		// Блокировка: пока открыта транзакция с FOR UPDATE, вторая не может взять строку без ожидания.
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := db.New(tx).GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: u.ID}); err != nil {
			t.Fatal(err)
		}
		var locked bool
		if err := pool.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM exercises WHERE id = $1 FOR UPDATE SKIP LOCKED)`, id).Scan(&locked); err != nil || !locked {
			t.Fatalf("строка должна быть заблокирована: %v (%v)", locked, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}

		if err := q.MarkExerciseAnswered(ctx, db.MarkExerciseAnsweredParams{ID: id, AnsweredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
			t.Fatal(err)
		}
		ex, _ = q.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: u.ID})
		if !ex.AnsweredAt.Valid {
			t.Fatal("answered_at не проставлен")
		}
		first := ex.AnsweredAt.Time
		if err := q.MarkExerciseAnswered(ctx, db.MarkExerciseAnsweredParams{ID: id, AnsweredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
			t.Fatal(err)
		}
		ex, _ = q.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: u.ID})
		if !ex.AnsweredAt.Time.Equal(first) {
			t.Fatal("повторный MarkExerciseAnswered не должен менять время")
		}

		used, err := q.UsedSentences(ctx, db.UsedSentencesParams{UserID: u.ID, Topic: "grammar:articles"})
		if err != nil || !slices.Equal(used, []int64{good}) {
			t.Fatalf("использованные: %v (%v)", used, err)
		}
		used, _ = q.UsedSentences(ctx, db.UsedSentencesParams{UserID: u.ID, Topic: "grammar:future"})
		if len(used) != 0 {
			t.Fatalf("другая тема: %v", used)
		}
		used, _ = q.UsedSentences(ctx, db.UsedSentencesParams{UserID: other, Topic: "grammar:articles"})
		if len(used) != 0 {
			t.Fatalf("чужой пользователь: %v", used)
		}
		// Неверный вид упражнения и пустые темы отклоняет база.
		if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "x", SentenceID: &good, Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
			t.Fatal("kind=x должен отклоняться")
		}
		if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "gap", SentenceID: &good, Topics: []string{}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
			t.Fatal("пустые темы должны отклоняться")
		}
	})

	t.Run("рейтинги тем: upsert, накопление попыток, список, изоляция пользователей", func(t *testing.T) {
		if _, err := q.GetTopicRatingForUpdate(ctx, db.GetTopicRatingForUpdateParams{UserID: u.ID, Topic: "t1"}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("нет рейтинга: ожидали ErrNoRows, получили %v", err)
		}
		up := func(topic string, rating float64, delta int32) {
			t.Helper()
			err := q.UpsertTopicRating(ctx, db.UpsertTopicRatingParams{UserID: u.ID, Topic: topic, Rating: rating, Deviation: 200, Volatility: 0.06, AttemptsDelta: delta})
			if err != nil {
				t.Fatal(err)
			}
		}
		up("t2", 1400, 1)
		up("t1", 1500, 1)
		up("t1", 1550, 1)
		r, err := q.GetTopicRatingForUpdate(ctx, db.GetTopicRatingForUpdateParams{UserID: u.ID, Topic: "t1"})
		if err != nil || r.Rating != 1550 || r.Attempts != 2 || r.Deviation != 200 {
			t.Fatalf("t1: %+v (%v)", r, err)
		}
		list, err := q.ListTopicRatings(ctx, u.ID)
		if err != nil || len(list) != 2 || list[0].Topic != "t1" || list[1].Topic != "t2" {
			t.Fatalf("список: %+v (%v)", list, err)
		}
		list, err = q.ListTopicRatings(ctx, other)
		if err != nil || len(list) != 0 {
			t.Fatalf("чужой пользователь: %+v (%v)", list, err)
		}
	})
}
