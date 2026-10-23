package exercises

import (
	"slices"
	"testing"
)

func TestTokens(t *testing.T) {
	tests := map[string][]string{
		"I have a dog.":           {"i", "have", "a", "dog"},
		"Don't go, Tom's friend!": {"don't", "go", "tom's", "friend"},
		"It’s a well-known fact.": {"it's", "a", "well-known", "fact"},
		"'Quoted' - word -- end":  {"quoted", "word", "end"},
		"Room 101 is open.":       {"room", "101", "is", "open"},
		"":                        {},
	}
	for in, want := range tests {
		if got := Tokens(in); !slices.Equal(got, want) {
			t.Errorf("Tokens(%q) = %q, ждали %q", in, got, want)
		}
	}
}

func TestSentenceLevel(t *testing.T) {
	known := map[string]WordInfo{
		"i": {Level: "A1"}, "dog": {Level: "B1"}, "big": {Level: "A2"}, "is": {Level: "A1"}, "the": {Level: "A1"},
		"tom": {Rank: 300}, "walk": {Rank: 5000}, "zorb": {Rank: 50000}, "rare": {Rank: 20000}, "edge": {Rank: 20001},
	}
	tests := []struct {
		name   string
		tokens []string
		want   string
		ok     bool
	}{
		{"максимум", []string{"i", "dog", "is", "big"}, "B1", true},
		{"частое слово без уровня", []string{"tom", "is", "big"}, "A2", true},
		{"граница частоты", []string{"rare", "is", "big"}, "A2", true},
		{"чуть реже границы", []string{"edge", "is", "big"}, "", false},
		{"редкое слово", []string{"the", "zorb", "is", "big"}, "", false},
		{"слова нет в словаре", []string{"the", "unknown", "big"}, "", false},
		{"ни у кого нет уровня", []string{"tom", "walk"}, "", false},
		{"апостроф нейтрален", []string{"tom's", "dog", "is", "big"}, "B1", true},
		{"только слова с апострофом", []string{"don't", "it's"}, "", false},
		{"апостроф и редкое слово", []string{"tom's", "zorb", "is", "big"}, "", false},
		{"пусто", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SentenceLevel(tt.tokens, known)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("= %q, %v; ждали %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPastSimpleTarget(t *testing.T) {
	tests := map[string]string{
		"we walked home":                    "walked",
		"he went to school":                 "went",
		"i have gone home":                  "",
		"she is tired today":                "",
		"they like the dog":                 "",
		"we have walked, then they stopped": "stopped",
	}
	for in, want := range tests {
		if got := PastSimpleTarget(Tokens(in)); got != want {
			t.Errorf("PastSimpleTarget(%q) = %q, ждали %q", in, got, want)
		}
	}
}

func TestBaseCandidates(t *testing.T) {
	tests := map[string][]string{
		"walked":  {"walk", "walke"},
		"hoped":   {"hop", "hope"},
		"liked":   {"lik", "like"},
		"stopped": {"stopp", "stoppe", "stop"},
		"studied": {"study"},
		"used":    nil, // короче пяти букв: не разбираем
		"died":    nil,
		"went":    nil,
		"bought":  nil,
	}
	for in, want := range tests {
		if got := BaseCandidates(in); !slices.Equal(got, want) {
			t.Errorf("BaseCandidates(%q) = %q, ждали %q", in, got, want)
		}
	}
}

func TestPastDistractors(t *testing.T) {
	w := func(ws ...string) map[string]WordInfo {
		m := map[string]WordInfo{}
		for _, x := range ws {
			m[x] = WordInfo{Rank: 100}
		}
		return m
	}
	tests := []struct {
		name   string
		target string
		known  map[string]WordInfo
		want   []string // nil — отвлекающих нет; иначе обязательные
	}{
		{"правильный", "walked", w("walk"), []string{"walk", "walks", "walking"}},
		{"на -e", "liked", w("like"), []string{"like", "likes", "liking"}},
		{"удвоение", "stopped", w("stop"), []string{"stop", "stops", "stopping"}},
		{"на -ied", "studied", w("study"), []string{"study", "studies", "studying"}},
		{"неоднозначно hop/hope", "hoped", w("hop", "hope"), nil},
		{"основа неизвестна", "walked", w(), nil},
		{"неправильный без словаря", "went", w(), []string{"go", "goes", "going"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PastDistractors(tt.target, tt.known)
			if tt.want == nil {
				if len(got) != 0 {
					t.Fatalf("ждали пусто, получили %q", got)
				}
				return
			}
			for _, x := range tt.want {
				if !slices.Contains(got, x) {
					t.Fatalf("%q без %q", got, x)
				}
			}
			if slices.Contains(got, tt.target) {
				t.Fatalf("среди отвлекающих сама цель: %q", got)
			}
		})
	}
}
