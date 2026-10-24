package exercises

import (
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
)

func TestBuildGapArticles(t *testing.T) {
	g, ok := BuildGap("I have a dog.", grammar.Articles, []string{"i", "have", "a", "dog"}, 1)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	if g.Prompt != "I have _____ dog." {
		t.Errorf("Prompt = %q", g.Prompt)
	}
	if !slices.Equal(g.Answers, []string{"a"}) {
		t.Errorf("Answers = %v", g.Answers)
	}
	if len(g.Choices) != 3 {
		t.Fatalf("Choices = %v", g.Choices)
	}
	seen := map[string]bool{}
	for _, c := range g.Choices {
		seen[c] = true
	}
	if len(seen) != 3 || !seen["a"] || !seen["an"] || !seen["the"] {
		t.Errorf("Choices = %v", g.Choices)
	}
}

func TestBuildGapKeepsPunctuationAndApostrophe(t *testing.T) {
	g, ok := BuildGap("I'm in the garden.", grammar.Prepositions, []string{"i'm", "in", "the", "garden"}, 2)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	if g.Prompt != "I'm _____ the garden." {
		t.Errorf("Prompt = %q", g.Prompt)
	}
	if !slices.Contains(g.Choices, "in") || len(g.Choices) != 4 {
		t.Errorf("Choices = %v", g.Choices)
	}
}

func TestBuildGapCapitalized(t *testing.T) {
	g, ok := BuildGap("The dog runs.", grammar.Articles, []string{"the", "dog", "runs"}, 3)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	if g.Prompt != "_____ dog runs." || !slices.Equal(g.Answers, []string{"The"}) {
		t.Errorf("%+v", g)
	}
	for _, c := range g.Choices {
		if c[0] < 'A' || c[0] > 'Z' {
			t.Errorf("вариант без заглавной: %q", c)
		}
	}
	if !Check(g.Answers, "The") {
		t.Error("Check должен принимать The")
	}
}

func TestBuildGapNoWord(t *testing.T) {
	if _, ok := BuildGap("Dogs run.", grammar.Articles, []string{"dogs", "run"}, 1); ok {
		t.Error("слова нет — ожидали false")
	}
	if _, ok := BuildGap("He left.", grammar.PastSimple, []string{"he", "left"}, 1); ok {
		t.Error("past_simple идёт через BuildGapForWord")
	}
}

func TestBuildGapFutureHasFourChoices(t *testing.T) {
	g, ok := BuildGap("It will rain.", grammar.Future, []string{"it", "will", "rain"}, 1)
	if !ok || len(g.Choices) != 4 || !slices.Contains(g.Choices, "will") {
		t.Errorf("%+v %v", g, ok)
	}
}

func TestBuildGapForWord(t *testing.T) {
	g, ok := BuildGapForWord("He walked home.", "walked", []string{"walk", "walks", "walking", "walked"}, 5)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	if g.Prompt != "He _____ home." || len(g.Choices) != 4 || !slices.Contains(g.Choices, "walked") {
		t.Errorf("%+v", g)
	}
	if _, ok := BuildGapForWord("He walked home.", "ran", []string{"a", "b", "c"}, 5); ok {
		t.Error("слова нет — false")
	}
	if _, ok := BuildGapForWord("He walked home.", "walked", []string{"walk"}, 5); ok {
		t.Error("мало отвлекающих — false")
	}
}

func TestBuildGapWholeWordOnly(t *testing.T) {
	g, ok := BuildGap("The other cat is a pet.", grammar.Articles, []string{"the", "other", "cat", "is", "a", "pet"}, 1)
	if !ok || !strings.HasPrefix(g.Prompt, "_____ other") {
		t.Errorf("%+v", g)
	}
}

func TestBuildGapDeterministic(t *testing.T) {
	tok := []string{"i", "have", "a", "dog"}
	a, _ := BuildGap("I have a dog.", grammar.Articles, tok, 42)
	b, _ := BuildGap("I have a dog.", grammar.Articles, tok, 42)
	if !slices.Equal(a.Choices, b.Choices) {
		t.Errorf("%v != %v", a.Choices, b.Choices)
	}
}

func TestCheck(t *testing.T) {
	if !Check([]string{"the"}, " The ") {
		t.Error("ожидали true")
	}
	if Check([]string{"the"}, "") || Check([]string{"the"}, "   ") {
		t.Error("пусто — false")
	}
	if Check([]string{"the"}, strings.Repeat("a", 201)) {
		t.Error("201 символ — false")
	}
	if Check([]string{"the"}, "a") {
		t.Error("неверный ответ")
	}
}

func TestPickTopic(t *testing.T) {
	if _, ok := PickTopic(nil, nil); ok {
		t.Error("пусто — false")
	}
	rs := []TopicRating{
		{Topic: "a", Mu: 1400, Phi: 100, Attempts: 5},
		{Topic: "b", Mu: 1600, Phi: 100, Attempts: 5},
	}
	if got, _ := PickTopic(rs, []string{"b", "a"}); got != "a" {
		t.Errorf("слабая раньше сильной: %q", got)
	}
	if got, _ := PickTopic(rs, []string{"b", "a", "new"}); got != "new" {
		t.Errorf("новая раньше: %q", got)
	}
	if got, _ := PickTopic(rs, []string{"b", "n1", "n2"}); got != "n1" {
		t.Errorf("первая новая по порядку: %q", got)
	}
	eq := []TopicRating{
		{Topic: "a", Mu: 1500, Phi: 80, Attempts: 3},
		{Topic: "b", Mu: 1500, Phi: 200, Attempts: 3},
	}
	if got, _ := PickTopic(eq, []string{"a", "b"}); got != "b" {
		t.Errorf("больше Phi: %q", got)
	}
}

func TestBuildGapAmbiguousOccurrence(t *testing.T) {
	if _, ok := BuildGap("The will is long, and it will help.", grammar.Future,
		[]string{"the", "will", "is", "long", "and", "it", "will", "help"}, 1); ok {
		t.Error("два will — ожидали false")
	}
	if _, ok := BuildGapForWord("He walked and walked.", "walked", []string{"walk", "walks", "walking"}, 1); ok {
		t.Error("повтор слова — ожидали false")
	}
}

func TestBuildGapLetterInsideWord(t *testing.T) {
	g, ok := BuildGap("Have a nice day.", grammar.Articles, []string{"have", "a", "nice", "day"}, 1)
	if !ok || g.Prompt != "Have _____ nice day." {
		t.Errorf("%+v %v", g, ok)
	}
}

func TestBuildGapForWordApostropheAndHyphen(t *testing.T) {
	g, ok := BuildGapForWord("I'm here.", "i'm", []string{"you're", "he's", "we're"}, 1)
	if !ok || g.Prompt != "_____ here." || !slices.Contains(g.Choices, "I'm") {
		t.Errorf("%+v %v", g, ok)
	}
	g, ok = BuildGapForWord("He is well-known.", "well-known", []string{"famous", "wellknown", "known"}, 1)
	if !ok || g.Prompt != "He is _____." || !slices.Contains(g.Choices, "well-known") {
		t.Errorf("%+v %v", g, ok)
	}
}

func TestBuildGapLowerChangesLength(t *testing.T) {
	if _, ok := BuildGapForWord("İ will go.", "will", []string{"would", "can", "must"}, 1); ok {
		t.Error("ожидали false")
	}
}

func TestCheckMultipleAnswers(t *testing.T) {
	a := []string{"color", "colour"}
	if !Check(a, "Colour") || !Check(a, " color") || Check(a, "colr") {
		t.Error("несколько ответов")
	}
}

// Ответ, который сервер отдаёт клиенту (Answers[0]), обязан быть среди вариантов дословно:
// фронт сравнивает строго и подставляет его в предложение.
func TestBuildGapAnswerIsAmongChoices(t *testing.T) {
	cases := []struct {
		sentence, topic string
		tokens          []string
		wantAnswer      string
	}{
		{"The dog runs.", grammar.Articles, []string{"the", "dog", "runs"}, "The"},
		{"In May we swim.", grammar.Prepositions, []string{"in", "may", "we", "swim"}, "In"},
		{"I have a dog.", grammar.Articles, []string{"i", "have", "a", "dog"}, "a"},
	}
	for _, c := range cases {
		g, ok := BuildGap(c.sentence, c.topic, c.tokens, 5)
		if !ok {
			t.Fatalf("%q: пропуск не собран", c.sentence)
		}
		if !slices.Contains(g.Choices, g.Answers[0]) {
			t.Errorf("%q: ответ %q не среди вариантов %v", c.sentence, g.Answers[0], g.Choices)
		}
		if g.Answers[0] != c.wantAnswer {
			t.Errorf("%q: Answers[0] = %q, ждали %q", c.sentence, g.Answers[0], c.wantAnswer)
		}
		if !Check(g.Answers, strings.ToLower(c.wantAnswer)) || !Check(g.Answers, c.wantAnswer) {
			t.Errorf("%q: Check должен принимать оба регистра", c.sentence)
		}
	}
}

// Отвлекающие не должны быть верными ответами: ни «this» вместо артикля, ни «shall»/«would» вместо will,
// ни взаимозаменяемые модальные и предлоги.
func TestBuildGapNoCorrectDistractors(t *testing.T) {
	cases := []struct {
		sentence, topic string
		tokens          []string
		banned          []string
	}{
		{"I have a dog.", grammar.Articles, []string{"i", "have", "a", "dog"}, []string{"this", "that", "my"}},
		{"It will rain.", grammar.Future, []string{"it", "will", "rain"}, []string{"shall", "would", "can", "may"}},
		{"We may go.", grammar.Modals, []string{"we", "may", "go"}, []string{"might", "could", "can"}},
		{"We could go.", grammar.Modals, []string{"we", "could", "go"}, []string{"can", "may", "might", "would"}},
		{"The bird is above the house.", grammar.Prepositions, []string{"the", "bird", "is", "above", "the", "house"}, []string{"over"}},
		{"The cat is under it.", grammar.Prepositions, []string{"the", "cat", "is", "under", "it"}, []string{"below"}},
		{"Choose between us.", grammar.Prepositions, []string{"choose", "between", "us"}, []string{"among"}},
	}
	for _, c := range cases {
		for seed := int64(0); seed < 20; seed++ {
			g, ok := BuildGap(c.sentence, c.topic, c.tokens, seed)
			if !ok {
				t.Fatalf("%q: пропуск не собран", c.sentence)
			}
			if len(g.Choices) < 3 {
				t.Errorf("%q: вариантов меньше трёх: %v", c.sentence, g.Choices)
			}
			for _, ch := range g.Choices {
				if ch != g.Answers[0] && slices.Contains(c.banned, strings.ToLower(ch)) {
					t.Errorf("%q: верный по смыслу вариант %q среди отвлекающих %v", c.sentence, ch, g.Choices)
				}
			}
		}
	}
}

func TestBuildGapForWordArticleAgreement(t *testing.T) {
	pool := []string{"table", "apple", "orange", "dog", "cat", "egg", "house", "ear", "ice", "bed"}
	// «an _____»: отвлекающие тоже на гласную, иначе артикль выдаёт ответ.
	g, ok := BuildGapForWord("She ate an apple.", "apple", pool, 1)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	for _, c := range g.Choices {
		if !strings.ContainsRune("aeiou", rune(c[0])) {
			t.Errorf("вариант %q не на гласную после an: %v", c, g.Choices)
		}
	}
	// «a _____» — на согласную.
	g, ok = BuildGapForWord("She has a dog.", "dog", pool, 1)
	if !ok {
		t.Fatal("ожидали пропуск")
	}
	for _, c := range g.Choices {
		if strings.ContainsRune("aeiou", rune(c[0])) {
			t.Errorf("вариант %q на гласную после a: %v", c, g.Choices)
		}
	}
	// Подходящих меньше трёх (гласных всего две: orange, egg) — предложение пропускаем.
	if _, ok := BuildGapForWord("She ate an apple.", "apple", []string{"orange", "egg", "table", "dog"}, 1); ok {
		t.Error("гласных отвлекающих меньше трёх — ждали false")
	}
	// Без артикля фильтра нет.
	if g, ok := BuildGapForWord("She likes apple pie.", "apple", pool, 1); !ok || len(g.Choices) != 4 {
		t.Errorf("без артикля: %+v %v", g, ok)
	}
}

// Месяц May не должен становиться целью пропуска для modals; цель — настоящий can.
func TestBuildGapSkipsCapitalizedMay(t *testing.T) {
	s := "We met in May and we can swim."
	g, ok := BuildGap(s, grammar.Modals, CasedTokens(s), 1)
	if !ok || g.Prompt != "We met in May and we _____ swim." {
		t.Fatalf("%+v %v", g, ok)
	}
	if _, ok := BuildGap("We met in May.", grammar.Modals, CasedTokens("We met in May."), 1); ok {
		t.Error("May — месяц, пропуска быть не должно")
	}
}

// «Mr.» с точкой внутри предложения не мешает собрать пропуск.
func TestBuildGapWithMrDot(t *testing.T) {
	s := "Mr. Smith has a dog."
	g, ok := BuildGap(s, grammar.Articles, CasedTokens(s), 1)
	if !ok || g.Prompt != "Mr. Smith has _____ dog." || g.Answers[0] != "a" {
		t.Fatalf("%+v %v", g, ok)
	}
}
