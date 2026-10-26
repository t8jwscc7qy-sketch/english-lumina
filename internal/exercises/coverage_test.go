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

func TestCoverageOKAllowsOneUnknownInLongerSentences(t *testing.T) {
	known := KnownForms([]LemmaPos{{"dog", "noun"}, {"walk", "verb"}, {"park", "noun"}, {"happy", "adjective"}, {"morning", "noun"}})
	short := CasedTokens("The dog walks in the cat park.") // 5 учитываемых слов, 1 неизвестное: 0,8
	if CoverageOK(short, known, 0.95) {
		t.Fatal("в коротком предложении неизвестное слово при пороге 0,95 не допускается")
	}
	if !CoverageOK(short, known, 0.7) {
		t.Fatal("при пороге 0,7 одно неизвестное из пяти допускается")
	}
	eight := CasedTokens("Dog walk park happy morning dog park cat.") // 8 слов, неизвестно только cat
	if !CoverageOK(eight, known, 0.95) {
		t.Fatal("с восьми слов допускается одно неизвестное")
	}
	if CoverageOK(CasedTokens("Dog walk cat cat park happy morning dog."), known, 0.95) {
		t.Fatal("два неизвестных при пороге 0,95 не допускаются")
	}
}

func TestCoverageOKBoundary(t *testing.T) {
	known := map[string]bool{}
	var tokens []string
	for i := 0; i < 20; i++ {
		w := string(rune('a'+i)) + "xx"
		tokens = append(tokens, w)
		if i < 19 {
			known[w] = true
		}
	}
	if !CoverageOK(tokens, known, float64(float32(0.95))-1e-6) {
		t.Fatal("19 из 20 проходят порог 0,95 (с учётом float32)")
	}
	tokens[0] = "unknown1"
	delete(known, "axx")
	if CoverageOK(tokens, known, 0.95) {
		t.Fatal("18 из 20 не проходят порог 0,95")
	}
}

func TestCoveragePossessivesAndHyphens(t *testing.T) {
	known := KnownForms([]LemmaPos{{"dog", "noun"}, {"ice", "noun"}, {"cream", "noun"}, {"like", "verb"}, {"eat", "verb"}, {"bone", "noun"}, {"toy", "noun"}})
	for _, text := range []string{"I like the dog's bone", "We eat ice-cream", "Those are the dogs' toys"} {
		_, unknown := counts(CasedTokens(text), known)
		if unknown != 0 {
			t.Errorf("%q: неизвестных %d", text, unknown)
		}
	}
	if isKnown("ice-pick", known) {
		t.Error("составное слово с неизвестной частью неизвестно")
	}
}
