package words

import (
	"slices"
	"testing"
)

func TestSingleWords(t *testing.T) {
	forms := Forms("run", "verb")
	got := SingleWords(forms)
	slices.Sort(got)
	if !slices.Equal(got, []string{"ran", "run", "running", "runs"}) {
		t.Fatalf("SingleWords = %v", got)
	}
	// Фразы пропускаются: в списке частот только отдельные слова.
	if got := SingleWords(Forms("turn to", "verb")); len(got) != 0 {
		t.Fatalf("у фразы однословных форм нет: %v", got)
	}
	// Варианты через слэш: однословные берутся, фразы-сокращения нет.
	mixed := SingleWords(Forms("cafe/café", "noun"))
	slices.Sort(mixed)
	if !slices.Equal(mixed, []string{"cafe", "cafes", "café", "cafés"}) {
		t.Fatalf("SingleWords(cafe/café) = %v", mixed)
	}
	if got := SingleWords(nil); got != nil {
		t.Fatalf("SingleWords(nil) = %v", got)
	}
}

func TestBestRank(t *testing.T) {
	ranks := map[string]int{"run": 17, "running": 5, "ran": 66, "runner": 300}
	tests := []struct {
		name       string
		lemma, pos string
		want       int
		wantOK     bool
	}{
		{"лучшее место среди форм", "run", "verb", 5, true},
		{"существительное: только свои формы", "runner", "noun", 300, true},
		{"форма в списке есть, лемма нет", "walk", "verb", 0, false},
		{"слова нет в списке", "zebra", "noun", 0, false},
		{"фраза без места", "turn to", "verb", 0, false},
		{"вариант через слэш: лучшее по вариантам", "ran/run", "verb", 5, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := BestRank(Forms(tc.lemma, tc.pos), ranks)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("BestRank(%q, %q) = (%d, %v), ожидали (%d, %v)", tc.lemma, tc.pos, got, ok, tc.want, tc.wantOK)
			}
		})
	}
	if _, ok := BestRank(Forms("run", "verb"), nil); ok {
		t.Fatal("пустая карта рангов не должна давать место")
	}
}
