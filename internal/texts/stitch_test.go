package texts

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// world строит синтетический мир: nKnown известных слов kNN и целевые слова tN (по одной форме), у каждого целевого
// слова sentencesPer предложений вида «kA kB kC tN kD kE kF» из семи слов, где соседние предложения делят слово.
func world(nKnown, nTargets, sentencesPer int) (map[string]bool, []Candidate) {
	known := map[string]bool{}
	for i := 0; i < nKnown; i++ {
		known[fmt.Sprintf("kw%02d", i)] = true
	}
	var cands []Candidate
	id := int64(0)
	for t := 0; t < nTargets; t++ {
		target := fmt.Sprintf("tw%02d", t)
		c := Candidate{Target: target, Forms: map[string]bool{target: true}}
		for s := 0; s < sentencesPer; s++ {
			id++
			w := func(k int) string { return fmt.Sprintf("kw%02d", (t*3+s*2+k)%nKnown) }
			tokens := []string{w(0), w(1), w(2), target, w(3), w(4), w(5)}
			c.Sentences = append(c.Sentences, Sentence{ID: id, En: strings.Join(tokens, " "), Ru: "перевод", Tokens: tokens})
		}
		cands = append(cands, c)
	}
	return known, cands
}

func TestStitchHitsGoalAndConstraints(t *testing.T) {
	known, cands := world(40, 12, 4)
	text, ok := Stitch(known, cands, Options{Goal: 0.85, Seed: 1})
	if !ok {
		t.Fatal("текст не собран")
	}
	if text.Coverage < 0.80 || text.Coverage > 0.90 {
		t.Fatalf("понимание %v вне 0,80–0,90", text.Coverage)
	}
	if n := len(text.Targets); n < 3 || n > 6 {
		t.Fatalf("целевых слов %d", n)
	}
	if n := len(text.Sentences); n < 6 || n > 10 {
		t.Fatalf("предложений %d", n)
	}
	ids := map[int64]bool{}
	for _, s := range text.Sentences {
		if ids[s.ID] {
			t.Fatalf("предложение %d повторяется", s.ID)
		}
		ids[s.ID] = true
	}
	for _, target := range text.Targets {
		n := 0
		for _, s := range text.Sentences {
			for _, tok := range s.Tokens {
				if tok == target {
					n++
				}
			}
		}
		if n < 2 {
			t.Errorf("целевое слово %s встречается %d раз, нужно не меньше двух", target, n)
		}
	}
}

func TestStitchIsReproducible(t *testing.T) {
	known, cands := world(40, 12, 4)
	a, _ := Stitch(known, cands, Options{Seed: 7})
	b, _ := Stitch(known, cands, Options{Seed: 7})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("при том же зерне текст должен быть тем же")
	}
	same := true
	for seed := int64(1); seed <= 12 && same; seed++ {
		c, _ := Stitch(known, cands, Options{Seed: seed})
		same = reflect.DeepEqual(a.Targets, c.Targets) && reflect.DeepEqual(a.Sentences, c.Sentences)
	}
	if same {
		t.Fatal("разные зёрна должны давать разные тексты")
	}
}

func TestStitchSkipsTargetsWithOneSentence(t *testing.T) {
	known, cands := world(30, 6, 3)
	cands = append([]Candidate{{Target: "lonely", Forms: map[string]bool{"lonely": true}, Sentences: []Sentence{{ID: 999, Tokens: []string{"kw01", "kw02", "kw03", "lonely", "kw04", "kw05"}}}}}, cands...)
	text, ok := Stitch(known, cands, Options{Seed: 1})
	if !ok {
		t.Fatal("текст не собран")
	}
	for _, target := range text.Targets {
		if target == "lonely" {
			t.Fatal("слово с одним предложением не берётся: не повторить дважды")
		}
	}
}

func TestStitchHonestWhenGoalUnreachable(t *testing.T) {
	// Мало известных слов: в каждом предложении 1 целевое и 1 лишнее неизвестное из 6.
	known := map[string]bool{"kw00": true, "kw01": true, "kw02": true, "kw03": true}
	var cands []Candidate
	id := int64(0)
	for i := 0; i < 5; i++ {
		target := fmt.Sprintf("tw%02d", i)
		c := Candidate{Target: target, Forms: map[string]bool{target: true}}
		for s := 0; s < 3; s++ {
			id++
			c.Sentences = append(c.Sentences, Sentence{ID: id, Tokens: []string{"kw00", "kw01", target, "extra" + fmt.Sprint(id), "kw02", "kw03"}})
		}
		cands = append(cands, c)
	}
	text, ok := Stitch(known, cands, Options{Goal: 0.9, Seed: 1})
	if !ok {
		t.Fatal("ближайший вариант всё равно отдаётся")
	}
	if text.Coverage > 0.75 {
		t.Fatalf("понимание %v должно честно отражать нехватку известных слов", text.Coverage)
	}
}

func TestStitchRejectsUnusableInput(t *testing.T) {
	known, cands := world(30, 5, 3)
	if _, ok := Stitch(known, nil, Options{}); ok {
		t.Fatal("без кандидатов собрать нечего")
	}
	// Предложения короче 4 слов и без целевого слова не годятся.
	short := []Candidate{{Target: "tw00", Forms: map[string]bool{"tw00": true}, Sentences: []Sentence{{ID: 1, Tokens: []string{"kw01", "tw00"}}, {ID: 2, Tokens: []string{"kw01", "kw02", "kw03", "kw04", "kw05"}}}}}
	if _, ok := Stitch(known, short, Options{}); ok {
		t.Fatal("короткие предложения и предложения без цели отбрасываются")
	}
	// Слишком много лишних неизвестных в предложении.
	many := []Candidate{{Target: "tw00", Forms: map[string]bool{"tw00": true}, Sentences: []Sentence{
		{ID: 1, Tokens: []string{"kw01", "tw00", "u1", "u2", "kw02"}}, {ID: 2, Tokens: []string{"kw01", "tw00", "u3", "u4", "kw02"}},
	}}}
	if _, ok := Stitch(known, many, Options{}); ok {
		t.Fatal("два лишних неизвестных слова в предложении — слишком много")
	}
	_ = cands
}

func TestStitchDoesNotCountNamesAndFunctionWords(t *testing.T) {
	known := map[string]bool{"kw01": true, "kw02": true}
	c := Candidate{Target: "tw", Forms: map[string]bool{"tw": true}}
	for i := int64(1); i <= 3; i++ {
		c.Sentences = append(c.Sentences, Sentence{ID: i, Tokens: []string{"The", "kw01", "tw", "and", "Anna", "kw02"}}) // The, and — служебные; Anna — имя
	}
	text, ok := Stitch(known, []Candidate{c}, Options{Goal: 0.85, MinSentences: 3, MinTargets: 1, Seed: 1})
	if !ok || text.Coverage < 0.79 {
		t.Fatalf("служебные слова и имя не считаются неизвестными: %v (ok=%v)", text.Coverage, ok)
	}
}

// Предложения двух тем перемешаны по номерам: связность возможна только если порядок строится по общим словам.
func TestStitchOrdersByCohesion(t *testing.T) {
	known := map[string]bool{"garden": true, "flower": true, "engine": true, "wheel": true}
	mk := func(target string, ids []int64, w1, w2 string) Candidate {
		c := Candidate{Target: target, Forms: map[string]bool{target: true}}
		for _, id := range ids {
			c.Sentences = append(c.Sentences, Sentence{ID: id, Tokens: []string{"the", w1, "and", w2, target, "with"}})
		}
		return c
	}
	cands := []Candidate{mk("tulip", []int64{1, 3}, "garden", "flower"), mk("motor", []int64{2, 4}, "engine", "wheel")}
	text, ok := Stitch(known, cands, Options{MinTargets: 2, MaxTargets: 2, MinSentences: 4, Goal: 0.85, Tolerance: 0.1, Seed: 1})
	if !ok || len(text.Sentences) != 4 {
		t.Fatalf("ok=%v, предложений %d", ok, len(text.Sentences))
	}
	if text.Cohesion < 0.6 {
		t.Fatalf("связность %v: порядок %v должен группировать предложения по общим словам", text.Cohesion, ids(text))
	}
	a, b := ids(text)[0], ids(text)[1]
	if a%2 != b%2 {
		t.Fatalf("первые два предложения из разных тем: %v", ids(text))
	}
}

func ids(t Text) []int64 {
	out := make([]int64, len(t.Sentences))
	for i, s := range t.Sentences {
		out[i] = s.ID
	}
	return out
}
