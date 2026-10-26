package learn

import (
	"slices"
	"testing"
)

func ids(cs []Candidate) []int64 {
	out := make([]int64, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func yes(Candidate) bool                      { return true }
func never(_, _ int64) bool                   { return false }
func c(id int64, lemma, pos string) Candidate { return Candidate{ID: id, Lemma: lemma, Pos: pos} }

func TestPickBlockKeepsOrderAndLimit(t *testing.T) {
	cands := []Candidate{c(1, "apple", "noun"), c(2, "run", "verb"), c(3, "big", "adjective"), c(4, "house", "noun")}
	if got := ids(PickBlock(cands, 3, yes, never)); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("порядок и лимит: %v", got)
	}
	if got := PickBlock(cands, 0, yes, never); len(got) != 0 {
		t.Fatalf("n = 0: %v", got)
	}
	if got := PickBlock(nil, 5, yes, never); len(got) != 0 {
		t.Fatalf("пустой список: %v", got)
	}
	if got := PickBlock(cands, -2, yes, never); len(got) != 0 {
		t.Fatalf("n < 0: %v", got)
	}
}

func TestPickBlockAtMostThreePerPos(t *testing.T) {
	cands := []Candidate{c(1, "apple", "noun"), c(2, "house", "noun"), c(3, "water", "noun"), c(4, "table", "noun"), c(5, "run", "verb"), c(6, "dog", "noun")}
	if got := ids(PickBlock(cands, 7, yes, never)); !slices.Equal(got, []int64{1, 2, 3, 5}) {
		t.Fatalf("не больше трёх существительных: %v", got)
	}
}

func TestPickBlockSkipsRelatedBothDirections(t *testing.T) {
	cands := []Candidate{c(1, "big", "adjective"), c(2, "large", "adjective"), c(3, "small", "adjective"), c(4, "tiny", "adjective")}
	related := func(a, b int64) bool { return (a == 1 && b == 2) || (a == 4 && b == 3) } // направление не важно
	if got := ids(PickBlock(cands, 4, yes, related)); !slices.Equal(got, []int64{1, 3}) {
		t.Fatalf("связанные слова в блок не ставятся (в любую сторону): %v", got)
	}
}

func TestPickBlockSkipsNearSpellings(t *testing.T) {
	cands := []Candidate{c(1, "quite", "adverb"), c(2, "quiet", "adjective"), c(3, "cat", "noun"), c(4, "cut", "verb"), c(5, "house", "noun"), c(6, "horse", "noun")}
	got := ids(PickBlock(cands, 6, yes, never))
	if slices.Contains(got, 2) || slices.Contains(got, 4) || slices.Contains(got, 6) {
		t.Fatalf("близкие написания (перестановка, замена буквы) вместе нельзя: %v", got)
	}
	if !slices.Equal(got, []int64{1, 3, 5}) {
		t.Fatalf("%v", got)
	}
}

func TestPickBlockShortWordsAreNotNear(t *testing.T) {
	// слова из двух букв отличаются «одной буквой» постоянно (is/it, to/so): правило их не касается
	cands := []Candidate{c(1, "is", "verb"), c(2, "it", "pronoun"), c(3, "to", "particle")}
	if got := ids(PickBlock(cands, 3, yes, never)); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("%v", got)
	}
}

func TestPickBlockSkipsWithoutExample(t *testing.T) {
	cands := []Candidate{c(1, "alpha", "noun"), c(2, "beta", "noun"), c(3, "gamma", "noun")}
	ok := func(x Candidate) bool { return x.ID != 2 }
	if got := ids(PickBlock(cands, 3, ok, never)); !slices.Equal(got, []int64{1, 3}) {
		t.Fatalf("слово без примера пропускается: %v", got)
	}
}

func TestPickBlockRevisitStaysFirstAndOkNotAskedAfterLimit(t *testing.T) {
	cands := []Candidate{{ID: 9, Lemma: "hard", Pos: "verb", Revisit: true}, c(1, "apple", "noun"), c(2, "house", "noun")}
	asked := 0
	ok := func(Candidate) bool { asked++; return true }
	got := PickBlock(cands, 2, ok, never)
	if got[0].ID != 9 || !got[0].Revisit || len(got) != 2 {
		t.Fatalf("повторное знакомство первым: %+v", got)
	}
	if asked != 2 {
		t.Fatalf("проверка примеров (запрос к базе) вызывалась %d раз, ждали 2: после заполнения блока слова не проверяются", asked)
	}
}
