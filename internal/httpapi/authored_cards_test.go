package httpapi

import (
	"context"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Обычные тесты не должны зависеть от того, какие слова встречаются в авторских текстах: источник фраз отключён.
func init() { cardAuthoredSentences = func([][]string) []exampleSentence { return nil } }

func TestCardPhrasesStartWithAuthoredSentences(t *testing.T) {
	cardAuthoredSentences = authoredSentences
	t.Cleanup(func() { cardAuthoredSentences = func([][]string) []exampleSentence { return nil } })
	e := newExerciseEnv(t)
	e.store.examplesFor = func(string) []db.ListExamplesRow {
		return []db.ListExamplesRow{{EnID: 1, EnTatoebaID: 1001, EnText: "The dog is on the road.", EnAuthor: "tom", RuTatoebaID: 2001, RuText: "Собака на дороге.", RuAuthor: "ivan"}}
	}
	got, err := exampleSentences(context.Background(), e.store, "dog", "noun")
	if err != nil || len(got) < 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[0].Row.EnAuthor != authoredAuthor || got[0].Row.EnTatoebaID != 0 || got[0].Row.RuText == "" {
		t.Fatalf("первой идёт фраза из авторского текста: %+v", got[0].Row)
	}
	authored := 0
	for _, g := range got {
		if g.Row.EnAuthor == authoredAuthor {
			authored++
		}
		if g.End <= g.Start {
			t.Fatalf("место слова во фразе не найдено: %+v", g)
		}
	}
	if authored > authoredSentenceLimit {
		t.Fatalf("авторских фраз %d, не больше %d", authored, authoredSentenceLimit)
	}
	if last := got[len(got)-1]; last.Row.EnAuthor != "tom" {
		t.Fatalf("фразы Tatoeba остаются после авторских: %+v", last.Row)
	}
	// Слова нет в авторских текстах — остаются только фразы Tatoeba.
	if none, _ := exampleSentences(context.Background(), e.store, "zebra", "noun"); len(none) > 0 && none[0].Row.EnAuthor == authoredAuthor {
		t.Fatal("для слова вне авторских текстов авторских фраз нет")
	}
}
