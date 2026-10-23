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
	if len(g.Choices) != 4 {
		t.Fatalf("Choices = %v", g.Choices)
	}
	seen := map[string]bool{}
	for _, c := range g.Choices {
		seen[c] = true
	}
	if len(seen) != 4 || !seen["a"] || !seen["an"] || !seen["the"] {
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
	if g.Prompt != "_____ dog runs." || !slices.Equal(g.Answers, []string{"the"}) {
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
