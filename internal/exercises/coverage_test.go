package exercises

import (
	"fmt"
	"testing"
	"time"
)

func TestCoverage(t *testing.T) {
	known := KnownForms([]LemmaPos{{"run", "verb"}, {"morning", "noun"}, {"quick", "adjective"}})
	cases := []struct {
		name, text string
		want       float64
	}{
		{"всё известно, служебные слова не считаются", "I run in the morning", 1},
		{"форма известного слова", "He ran quickly", 2.0 / 3}, // ran известно, quickly нет, he служебное
		{"неизвестное слово", "The dog runs", 2.0 / 3},
		{"имя посреди предложения не считается", "I run with Anna", 1},
		{"имя в начале считается словом", "Anna runs", 0.5},
		{"число не считается", "I run 5 times", 2.0 / 3},
		{"пустое предложение", "", 1},
	}
	for _, c := range cases {
		got := Coverage(CasedTokens(c.text), known)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: %q → %v, ждали %v", c.name, c.text, got, c.want)
		}
	}
}

func TestCoverageThreshold(t *testing.T) {
	known := map[string]bool{}
	var tokens []string
	for i := 0; i < 20; i++ {
		w := fmt.Sprintf("w%c", 'a'+rune(i))
		tokens = append(tokens, w)
		if i < 19 {
			known[w] = true
		}
	}
	if got := Coverage(tokens, known); got < 0.95 || got > 0.9501 {
		t.Fatalf("19 из 20 = 0,95, получили %v", got)
	}
}

func TestKnownFormsIsFastFor5000Words(t *testing.T) {
	var lemmas []LemmaPos
	for i := 0; i < 5000; i++ {
		lemmas = append(lemmas, LemmaPos{fmt.Sprintf("word%c%c%c", 'a'+rune(i/676%26), 'a'+rune(i/26%26), 'a'+rune(i%26)), "verb"})
	}
	start := time.Now()
	known := KnownForms(lemmas)
	if d := time.Since(start); d > 200*time.Millisecond { // запас под -race; без него обычно в несколько раз быстрее
		t.Fatalf("5000 слов строятся %v", d)
	}
	if !known["wordaaa"] {
		t.Fatal("лемма должна быть среди форм")
	}
}
