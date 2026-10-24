package exercises

import (
	"slices"
	"testing"
)

var goForms = []string{"go", "goes", "went", "gone", "going"}

func TestBuildFormBasic(t *testing.T) {
	g, answer, ok := BuildForm("Yesterday she went to the cinema with her sister.", "verb", goForms, 1)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if answer != "went" {
		t.Fatalf("answer = %q", answer)
	}
	if len(g.Choices) < 3 || len(g.Choices) > 4 {
		t.Fatalf("вариантов %d: %v", len(g.Choices), g.Choices)
	}
	if !slices.Contains(g.Choices, "went") {
		t.Fatalf("нет went: %v", g.Choices)
	}
	seen := map[string]bool{}
	for _, c := range g.Choices {
		if seen[c] {
			t.Fatalf("повтор %q: %v", c, g.Choices)
		}
		seen[c] = true
		if c != "went" && !slices.Contains(goForms, c) {
			t.Fatalf("чужой вариант %q", c)
		}
	}
	if !slices.Contains(g.Choices, g.Answers[0]) {
		t.Fatalf("Answers[0] = %q не среди вариантов %v", g.Answers[0], g.Choices)
	}
}

func TestBuildFormRejectsDoubleForm(t *testing.T) {
	if _, _, ok := BuildForm("He went there and then went home yesterday.", "verb", goForms, 1); ok {
		t.Fatal("две одинаковые формы: ожидали отказ")
	}
}

func TestBuildFormRejectsTwoFormsOfWord(t *testing.T) {
	if _, _, ok := BuildForm("She goes home and went there yesterday.", "verb", goForms, 1); ok {
		t.Fatal("две разные формы: ожидали отказ")
	}
}

func TestBuildFormVerbNeedsTimeMarker(t *testing.T) {
	if _, _, ok := BuildForm("She went to the cinema with her sister.", "verb", goForms, 1); ok {
		t.Fatal("без маркера времени глагол должен отклоняться")
	}
	if _, _, ok := BuildForm("She went to the cinema with her sister yesterday.", "verb", goForms, 1); !ok {
		t.Fatal("с yesterday ожидали упражнение")
	}
	if _, _, ok := BuildForm("I saw two dogs in the garden.", "noun", []string{"dog", "dogs", "dog's", "dogs'"}, 1); !ok {
		t.Fatal("существительному маркер не нужен")
	}
}

func TestBuildFormCapitalized(t *testing.T) {
	g, answer, ok := BuildForm("Went home early yesterday.", "verb", goForms, 1)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if answer != "Went" || g.Answers[0] != "Went" {
		t.Fatalf("answer = %q, Answers = %v", answer, g.Answers)
	}
	if !Check(g.Answers, "went") {
		t.Fatal("Check должен принимать went")
	}
}

func TestBuildFormRejectsAmbiguousLemma(t *testing.T) {
	read := []string{"read", "reads", "reading"}
	for _, s := range []string{"She reads books every day.", "She is reading a book now.", "He read it yesterday."} {
		if _, _, ok := BuildForm(s, "verb", read, 1); ok {
			t.Errorf("%q: read неоднозначно, ожидали отказ", s)
		}
	}
	if _, _, ok := BuildForm("She walked home yesterday.", "verb", []string{"walk", "walked"}, 1); ok {
		t.Error("меньше трёх форм: ожидали отказ")
	}
	if _, _, ok := BuildForm("She walked home yesterday.", "verb", []string{"walk", "walked", "walked", "walk"}, 1); ok {
		t.Error("дубли форм не считаются: ожидали отказ")
	}
}

func TestBuildFormDeterministic(t *testing.T) {
	s := "Yesterday she went to the cinema."
	a, _, _ := BuildForm(s, "verb", goForms, 7)
	b, _, _ := BuildForm(s, "verb", goForms, 7)
	if !slices.Equal(a.Choices, b.Choices) {
		t.Fatalf("%v != %v", a.Choices, b.Choices)
	}
}
