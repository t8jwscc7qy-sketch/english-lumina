package httpapi

import (
	"context"
	"slices"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Фейк альтернатив повторяет SQL: те же русские переводы, не само предложение, 4–10 слов, по id, с лимитом.
func TestFakeListEnglishAlternatives(t *testing.T) {
	f := &fakeStore{}
	f.addPair(1, 100, "I went home yesterday.", 4)
	f.addPair(2, 100, "Yesterday I went home.", 4)
	f.addPair(3, 100, "I went.", 2)
	f.addPair(4, 101, "Something else entirely here.", 4)
	f.addPair(5, 100, "Yesterday after the long meeting I went straight home alone tonight.", 11)

	rows, err := f.ListEnglishAlternatives(context.Background(), db.ListEnglishAlternativesParams{EnID: 1, MaxRows: 5})
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if !slices.Equal(ids, []int64{2}) {
		t.Fatalf("альтернативы %v, ожидали [2]", ids)
	}
	f.addPair(6, 100, "I walked home yesterday.", 4)
	rows, _ = f.ListEnglishAlternatives(context.Background(), db.ListEnglishAlternativesParams{EnID: 1, MaxRows: 1})
	if len(rows) != 1 {
		t.Fatalf("лимит 1: %+v", rows)
	}
}
