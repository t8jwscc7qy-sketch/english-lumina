package testdb

import (
	"context"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestTranslationQueriesAndMatchExercise(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)

	done, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "freedict-eng-rus", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	half, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "freedict-eng-rus", Version: "2", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	add := func(src int64, lemma, pos string, sense int16, ru string) {
		t.Helper()
		if err := q.UpsertWordTranslation(ctx, db.UpsertWordTranslationParams{SourceID: src, Lemma: lemma, Pos: pos, SenseNo: sense, Ru: ru}); err != nil {
			t.Fatal(err)
		}
	}
	add(done.ID, "run", "v", 2, "бежать")
	add(done.ID, "run", "v", 1, "бегать")
	add(done.ID, "run", "n", 1, "бег")
	add(done.ID, "apple", "n", 1, "яблоко")
	add(done.ID, "other", "n", 1, "другое")
	add(half.ID, "run", "v", 1, "НЕДОГРУЖЕНО") // версия без завершённого импорта не подмешивается
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: done.ID, RowCount: ptr(int32(5))}); err != nil {
		t.Fatal(err)
	}

	rows, err := q.ListTranslationsForWords(ctx, db.ListTranslationsForWordsParams{SourceName: "freedict-eng-rus", Lemmas: []string{"run", "apple", "missing"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Lemma+"/"+r.Pos+"/"+r.Ru)
	}
	want := []string{"apple/n/яблоко", "run/n/бег", "run/v/бегать", "run/v/бежать"}
	if len(got) != len(want) {
		t.Fatalf("получили %v, ждали %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("получили %v, ждали %v (порядок: слово, часть речи, значение)", got, want)
		}
	}

	// Сопоставление: упражнение без предложения.
	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "match", Topics: []string{"skill:match"}, Payload: []byte(`{}`), Answer: []byte(`{}`)})
	if err != nil {
		t.Fatalf("упражнение match без предложения: %v", err)
	}
	ex, err := q.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: id, UserID: u.ID})
	if err != nil || ex.SentenceID != nil || ex.Kind != "match" {
		t.Fatalf("прочитано %+v (%v)", ex, err)
	}
	// Остальные виды без предложения по-прежнему не нужны, но неизвестный вид запрещён проверкой.
	if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "x", Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
		t.Fatal("неизвестный вид должен отклоняться")
	}
	used, err := q.UsedSentences(ctx, db.UsedSentencesParams{UserID: u.ID, Topic: "skill:match"})
	if err != nil || len(used) != 0 {
		t.Fatalf("UsedSentences не должен падать на упражнении без предложения: %v %v", used, err)
	}
}

func ptr[T any](v T) *T { return &v }
