package words

import (
	"slices"
	"strings"
	"testing"
)

// flat превращает формы в сортированный список строк: фразы склеены пробелом.
func flat(forms [][]string) []string {
	var out []string
	for _, f := range forms {
		out = append(out, strings.Join(f, " "))
	}
	slices.Sort(out)
	return out
}

func TestFormsExact(t *testing.T) {
	tests := []struct {
		name, lemma, pos string
		want             []string
	}{
		{"наречие — только лемма", "quickly", "adverb", []string{"quickly"}},
		{"неизвестная часть речи", "run", "", []string{"run"}},
		{"модальный глагол", "can", "modal auxiliary", []string{"can"}},
		{"регистр", "Run", "noun", []string{"run", "runs"}},
		{"существительное на -s/-x/-ch/-sh", "box", "noun", []string{"box", "boxes"}},
		{"существительное на согласная + y", "city", "noun", []string{"cities", "city"}},
		{"существительное на гласная + y", "day", "noun", []string{"day", "days"}},
		{"неправильное множественное", "child", "noun", []string{"child", "children"}},
		{"существительное на -o допускает оба", "potato", "noun", []string{"potato", "potatoes", "potatos"}},
		{"глагол правильный", "walk", "verb", []string{"walk", "walked", "walking", "walks"}},
		{"глагол на -e", "like", "verb", []string{"like", "liked", "likes", "liking"}},
		{"глагол на согласная + y", "study", "verb", []string{"studied", "studies", "study", "studying"}},
		{"глагол с удвоением", "stop", "verb", []string{"stop", "stoped", "stopped", "stopping", "stoping", "stops"}},
		{"глагол на -ie", "die", "verb", []string{"die", "died", "dies", "dying"}},
		{"неправильный глагол go", "go", "verb", []string{"go", "goes", "going", "gone", "went"}},
		{"неправильный глагол run", "run", "verb", []string{"ran", "run", "running", "runs"}},
		{"глагол be", "be", "be-verb", []string{"am", "are", "be", "been", "being", "is", "was", "were"}},
		{"глагол do", "do", "do-verb", []string{"did", "do", "does", "doing", "done"}},
		{"глагол have", "have", "have-verb", []string{"has", "had", "have", "having"}},
		{"прилагательное правильное", "fast", "adjective", []string{"fast", "faster", "fastest"}},
		{"прилагательное на -e", "nice", "adjective", []string{"nice", "nicer", "nicest"}},
		{"прилагательное на согласная + y", "happy", "adjective", []string{"happier", "happiest", "happy"}},
		{"прилагательное с удвоением", "big", "adjective", []string{"big", "bigger", "biger", "biggest", "bigest"}},
		{"неправильное прилагательное", "good", "adjective", []string{"best", "better", "good"}},
		{"фраза ищется как есть", "turn to", "verb", []string{"turn to"}},
		{"варианты через слэш обрабатываются отдельно", "cafe/café", "noun", []string{"cafe", "cafes", "café", "cafés"}},
		{"точки и регистр в сокращении", "a.m./A.M./am/AM", "adverb", []string{"a m", "am"}},
		{"пустая лемма", "", "noun", nil},
		{"одни знаки", "-- / ..", "noun", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flat(Forms(tc.lemma, tc.pos))
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("Forms(%q, %q) = %q, ожидали %q", tc.lemma, tc.pos, got, want)
			}
		})
	}
}

func TestFormsHaveOnlyLettersAndDigits(t *testing.T) {
	for _, lemma := range []string{"run' | 'x", "a&b", "x:*", "(y)", "p<->q", `back\slash`, "it's"} {
		for _, f := range Forms(lemma, "verb") {
			for _, w := range f {
				if w == "" || strings.ContainsAny(w, "'\\&|!():*<>- ") {
					t.Fatalf("Forms(%q): слово %q с недопустимыми знаками", lemma, w)
				}
			}
		}
	}
}

func TestFormsAreDeduplicatedAndBounded(t *testing.T) {
	got := flat(Forms("run/run/RUN", "verb"))
	if !slices.Equal(got, []string{"ran", "run", "running", "runs"}) {
		t.Fatalf("дубли не убраны: %q", got)
	}
	var variants []string
	for i := 0; i < 200; i++ {
		variants = append(variants, string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	if n := len(Forms(strings.Join(variants, "/"), "verb")); n != MaxForms {
		t.Fatalf("форм %d, больше предела %d", n, MaxForms)
	}
}

func TestTSQuery(t *testing.T) {
	tests := []struct {
		name  string
		forms [][]string
		want  string
	}{
		{"одно слово", [][]string{{"run"}}, `'run'`},
		{"несколько форм", [][]string{{"run"}, {"runs"}}, `'run' | 'runs'`},
		{"фраза", [][]string{{"turn", "to"}, {"turned", "to"}}, `'turn' <-> 'to' | 'turned' <-> 'to'`},
		{"пусто", nil, ``},
	}
	for _, tc := range tests {
		if got := TSQuery(tc.forms); got != tc.want {
			t.Fatalf("%s: TSQuery = %q, ожидали %q", tc.name, got, tc.want)
		}
	}
}

func TestIrregularPast(t *testing.T) {
	yes := []string{"went", "Went", "bought", "lied", "found", "ate", "said", "showed"}
	// причастия (gone, taken...) и неоднозначные слова (saw, bit, left...) — не прошедшее время.
	no := []string{"go", "goes", "going", "walked", "put", "read", "beginning", "lies", "dog", "",
		"gone", "seen", "taken", "written", "given", "known", "eaten", "broken", "drunk", "worn", "lain", "gotten",
		"bit", "left", "rose", "saw", "lay", "bore", "shot", "led", "got"}
	for _, w := range yes {
		if !IrregularPast(w) {
			t.Errorf("IrregularPast(%q) = false, ожидали true", w)
		}
	}
	for _, w := range no {
		if IrregularPast(w) {
			t.Errorf("IrregularPast(%q) = true, ожидали false", w)
		}
	}
}
