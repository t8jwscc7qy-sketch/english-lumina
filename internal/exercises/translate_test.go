package exercises

import (
	"slices"
	"strings"
	"testing"
)

func TestCheckTranslateExact(t *testing.T) {
	tests := []struct {
		answers []string
		given   string
		want    TranslateResult
	}{
		{[]string{"I don't know."}, "i dont know", TranslateWrong}, // без апострофа это другое слово
		{[]string{"I don't know."}, "I do not know", TranslateExact},
		{[]string{"I don't know."}, "  I  DON’T know!  ", TranslateExact},
		{[]string{"I don't know."}, "I don't, know", TranslateExact},
		{[]string{"I'm tired."}, "I am tired", TranslateExact},
		{[]string{"He can't swim."}, "he cannot swim", TranslateExact},
		{[]string{"She won't come."}, "she will not come", TranslateExact},
		{[]string{"Wait! Stop!"}, "wait stop", TranslateExact},
	}
	for _, tc := range tests {
		if got := CheckTranslate(tc.answers, tc.given); got.Result != tc.want {
			t.Errorf("%v / %q: %v, ждали %v", tc.answers, tc.given, got.Result, tc.want)
		}
	}
}

func TestCheckTranslateAmbiguousContractionsStay(t *testing.T) {
	if got := CheckTranslate([]string{"It's raining."}, "it is raining"); got.Result != TranslateWrong {
		t.Fatalf("'s не раскрывается: %v", got)
	}
	got := CheckTranslate([]string{"It's raining.", "It is raining."}, "It is raining")
	if got.Result != TranslateExact || got.Matched != "It is raining." {
		t.Fatalf("альтернатива принята: %+v", got)
	}
}

func TestCheckTranslateTypo(t *testing.T) {
	answers := []string{"We eat red apples every morning."}
	for name, given := range map[string]string{
		"удаление":     "we eat red aples every morning",
		"вставка":      "we eat red appples every morning",
		"замена":       "we eat red appxes every morning",
		"перестановка": "we eat red appels every morning",
	} {
		got := CheckTranslate(answers, given)
		if got.Result != TranslateTypo || got.Matched != answers[0] {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := CheckTranslate(answers, "we eat red aples evry morning"); got.Result != TranslateWrong {
		t.Fatalf("две опечатки: %+v", got)
	}
}

func TestCheckTranslateTypoNeedsLongPhrase(t *testing.T) {
	if got := CheckTranslate([]string{"He is here."}, "we is here"); got.Result != TranslateWrong {
		t.Fatalf("короткая фраза не прощает букву: %+v", got)
	}
	if got := CheckTranslate([]string{"I like my small house."}, "i like my smal house"); got.Result != TranslateTypo {
		t.Fatalf("длинная фраза прощает: %+v", got)
	}
	// граница 12 символов после приведения: «i like pens» — 11, «i like cakes» — 12
	if got := CheckTranslate([]string{"I like pens."}, "i like pans"); got.Result != TranslateWrong {
		t.Fatalf("11 символов: %+v", got)
	}
	if got := CheckTranslate([]string{"I like cakes."}, "i like cokes"); got.Result != TranslateTypo {
		t.Fatalf("12 символов: %+v", got)
	}
}

func TestCheckTranslatePrefersExactOverTypo(t *testing.T) {
	answers := []string{"I read books every day.", "I read book every day."}
	got := CheckTranslate(answers, "I read book every day")
	if got.Result != TranslateExact || got.Matched != answers[1] {
		t.Fatalf("%+v", got)
	}
}

func TestCheckTranslateEmptyAndLong(t *testing.T) {
	answers := []string{"We eat red apples every morning."}
	for _, given := range []string{"", "   ", "\n\t", strings.Repeat("a", 201), "🙂🙂🙂", strings.Repeat("🙂", 300)} {
		if got := CheckTranslate(answers, given); got.Result != TranslateWrong {
			t.Errorf("%q: %+v", given, got)
		}
	}
	if got := CheckTranslate(nil, "anything at all here"); got.Result != TranslateWrong {
		t.Fatalf("без ответов: %+v", got)
	}
}

func TestBuildTranslate(t *testing.T) {
	tr, ok := BuildTranslate("We eat red apples every morning.", []string{
		"Every morning we eat red apples.",
		"we eat red apples every morning", // повтор исходного после приведения
		"He said \"go\" now.",             // недопустимый символ: пропускается, остальное не ломается
	})
	if !ok || !slices.Equal(tr.Answers, []string{"We eat red apples every morning.", "Every morning we eat red apples."}) {
		t.Fatalf("%+v %v", tr, ok)
	}
}

func TestBuildTranslateBounds(t *testing.T) {
	tests := []struct {
		s  string
		ok bool
	}{
		{"I love you.", false},                                      // 3 слова: альтернатив из базы для коротких нет
		{"I love you so much.", true},                               // 4 слова
		{"We eat red apples in the big house today.", true},         // 9 слов
		{"We eat red apples in the big house today now.", false},    // 10 слов
		{strings.TrimSpace(strings.Repeat("abcdefghi ", 9)), false}, // 9 слов, но длиннее 80 символов
		{"He said \"hi\" to me.", false},
		{"Tom, wait; stop.", false},
		{"Is it (really) so?", false},
		{"Wait... what?", false},
		{"Wait… what is this?", false},
		{"Here is the thing: stop.", false},
		{"Café au lait please.", false},
		{"It's a well-known fact.", true},
		{"It’s a well-known fact.", true},
		{"Call me at 5, please.", true},
	}
	for _, tc := range tests {
		if _, ok := BuildTranslate(tc.s, nil); ok != tc.ok {
			t.Errorf("%q: %v, ждали %v", tc.s, ok, tc.ok)
		}
	}
}

func TestCheckTranslateHyphenAndApostrophes(t *testing.T) {
	for _, tc := range []struct{ answer, given string }{
		{"It is a well-known fact.", "it is a well known fact"},
		{"It is a well known fact.", "It is a well-known fact"},
		{"I don't know.", "I don`t know"},
		{"I don't know.", "I donʼt know"},
		{"I don't know.", "I don´t know"},
	} {
		if got := CheckTranslate([]string{tc.answer}, tc.given); got.Result != TranslateExact {
			t.Errorf("%q против %q: %v, ждали Exact", tc.given, tc.answer, got.Result)
		}
	}
}
