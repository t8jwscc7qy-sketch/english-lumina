package exercises

import (
	"slices"
	"strings"
	"testing"
)

func lowerSorted(tiles []string) []string {
	out := make([]string, len(tiles))
	for i, t := range tiles {
		out[i] = strings.ToLower(t)
	}
	slices.Sort(out)
	return out
}

func TestBuildOrderBasic(t *testing.T) {
	o, ok := BuildOrder("I went home yesterday.", nil, 1)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if o.Answers[0] != "I went home yesterday." {
		t.Fatalf("Answers[0] = %q", o.Answers[0])
	}
	want := []string{"I", "went", "home", "yesterday"} // завершающей точки в плитках нет
	if !slices.Equal(lowerSorted(o.Tiles), lowerSorted(want)) {
		t.Fatalf("плитки %v не из слов предложения", o.Tiles)
	}
	if slices.Equal(o.Tiles, want) {
		t.Fatalf("плитки в исходном порядке: %v", o.Tiles)
	}
}

func TestBuildOrderLowercasesFirstWord(t *testing.T) {
	o, ok := BuildOrder("Yesterday I went home.", nil, 2)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if !slices.Contains(o.Tiles, "yesterday") || slices.Contains(o.Tiles, "Yesterday") {
		t.Fatalf("первое слово должно быть строчным: %v", o.Tiles)
	}
	if o.Answers[0] != "Yesterday I went home." {
		t.Fatalf("Answers[0] = %q", o.Answers[0])
	}
	// Местоимение I остаётся заглавным даже не в начале.
	if !slices.Contains(o.Tiles, "I") {
		t.Fatalf("I должно остаться заглавным: %v", o.Tiles)
	}
}

func TestBuildOrderKeepsApostropheAndHyphen(t *testing.T) {
	o, ok := BuildOrder("I'm well-known here today.", nil, 3)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if !slices.Contains(o.Tiles, "I'm") || !slices.Contains(o.Tiles, "well-known") {
		t.Fatalf("плитки: %v", o.Tiles)
	}
}

func TestBuildOrderCommaStaysOnWord(t *testing.T) {
	o, ok := BuildOrder("Yes, I went home today.", nil, 6)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if !slices.Contains(o.Tiles, "yes,") || !slices.Contains(o.Tiles, "today") {
		t.Fatalf("запятая остаётся на слове, завершающая точка уходит: %v", o.Tiles)
	}
}

func TestBuildOrderRejectsCapitalized(t *testing.T) {
	for _, s := range []string{"Tom went to Boston today.", "I met Tom today.", "We saw Mr. Smith today."} {
		if _, ok := BuildOrder(s, nil, 1); ok {
			t.Errorf("%q: имя собственное, ожидали отказ", s)
		}
	}
	if _, ok := BuildOrder("I'll go home today.", nil, 1); !ok {
		t.Error("I'll — не имя, ожидали упражнение")
	}
}

func TestBuildOrderLength(t *testing.T) {
	cases := []struct {
		s  string
		ok bool
	}{
		{"I am here.", false}, // 3 токена
		{"I am here now.", true},
		{"a b c d e f g h i j", true},    // 10
		{"a b c d e f g h i j k", false}, // 11
	}
	for _, c := range cases {
		if _, ok := BuildOrder(c.s, nil, 1); ok != c.ok {
			t.Errorf("%q: ok = %v, ожидали %v", c.s, ok, c.ok)
		}
	}
}

func TestBuildOrderAlternatives(t *testing.T) {
	o, ok := BuildOrder("I went home yesterday.", []string{
		"Yesterday I went home.",  // те же токены, другой порядок
		"We went home yesterday.", // другой состав
		"I went home yesterday.",  // дубль исходного
	}, 4)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if !slices.Contains(o.Answers, "Yesterday I went home.") {
		t.Fatalf("альтернатива пропала: %v", o.Answers)
	}
	if slices.Contains(o.Answers, "We went home yesterday.") {
		t.Fatalf("альтернатива с другим составом должна отбрасываться: %v", o.Answers)
	}
	if len(o.Answers) != 2 {
		t.Fatalf("Answers = %v, ожидали исходное и одну альтернативу", o.Answers)
	}
}

func TestBuildOrderDeterministic(t *testing.T) {
	a, _ := BuildOrder("I went home yesterday.", nil, 7)
	b, _ := BuildOrder("I went home yesterday.", nil, 7)
	if !slices.Equal(a.Tiles, b.Tiles) {
		t.Fatalf("один seed дал разные плитки: %v и %v", a.Tiles, b.Tiles)
	}
	for seed := int64(0); seed < 50; seed++ {
		o, ok := BuildOrder("I went home yesterday.", nil, seed)
		if !ok {
			t.Fatalf("seed %d: отказ", seed)
		}
		if CheckOrder(o.Answers, strings.Join(o.Tiles, " ")) {
			t.Fatalf("seed %d: плитки сложились в верный ответ без участия пользователя: %v", seed, o.Tiles)
		}
	}
}

func TestBuildOrderDuplicateTokens(t *testing.T) {
	o, ok := BuildOrder("The cat saw the dog today.", nil, 5)
	if !ok {
		t.Fatal("ожидали упражнение")
	}
	if !CheckOrder(o.Answers, "the cat saw the dog today.") {
		t.Fatal("ответ с строчным первым словом должен приниматься")
	}
}

func TestBuildOrderLowerChangesLength(t *testing.T) {
	if _, ok := BuildOrder("İ am here today now.", nil, 1); ok {
		t.Fatal("ожидали отказ: ToLower меняет длину")
	}
}

func TestCheckOrder(t *testing.T) {
	answers := []string{"The cat saw the dog today."}
	if !CheckOrder(answers, "  the  cat saw the dog today. ") {
		t.Error("лишние пробелы и регистр первого слова не должны мешать")
	}
	if !CheckOrder(answers, "The cat saw the dog today") {
		t.Error("завершающая точка не обязательна")
	}
	if !CheckOrder(answers, "The cat saw the dog today?") {
		t.Error("вид завершающего знака не важен")
	}
	if CheckOrder([]string{"Yes, I went home today."}, "Yes I went home today") {
		t.Error("запятая внутри предложения приклеена к слову и сравнивается")
	}
	if CheckOrder(answers, strings.Repeat("a ", 101)) {
		t.Error("длиннее 200 символов — неверно")
	}
	if CheckOrder(answers, "") || CheckOrder(answers, "   ") {
		t.Error("пустой ответ неверен")
	}
}

func TestOrderTopics(t *testing.T) {
	if got := OrderTopics("B1"); !slices.Equal(got, []string{"skill:order", "level:B1"}) {
		t.Fatalf("OrderTopics(B1) = %v", got)
	}
	if got := OrderTopics(""); !slices.Equal(got, []string{"skill:order"}) {
		t.Fatalf("OrderTopics(\"\") = %v", got)
	}
}

func TestBuildOrderRejectsPunctuationHints(t *testing.T) {
	for _, s := range []string{
		`"Come here," she said quietly.`,
		`She said, "Come here now."`,
		`(Please) close the door now.`,
		"Élodie is my best friend.",
		"I don't know what to do…",
		"She said “come here now”.",
		"We [all] went home early.",
	} {
		if _, ok := BuildOrder(s, nil, 1); ok {
			t.Errorf("%q: пунктуация или регистр выдают место слова, ожидали отказ", s)
		}
	}
	for _, s := range []string{"I don't know what to do.", "Well, I went home early.", "I’m here right now."} {
		if _, ok := BuildOrder(s, nil, 1); !ok {
			t.Errorf("%q: обычная запятая и апостроф допустимы", s)
		}
	}
}
