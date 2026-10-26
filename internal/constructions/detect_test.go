package constructions

import (
	"slices"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
)

func ids(text string) []string {
	var out []string
	for _, m := range Detect([]Sentence{{Text: text, Tokens: exercises.CasedTokens(text)}}) {
		out = append(out, m.ID)
	}
	return out
}

func TestDetectPositive(t *testing.T) {
	cases := []struct{ text, want string }{
		{"I have seen this film.", PresentPerfect},
		{"She has never been to Paris.", PresentPerfect},
		{"We've already finished the work.", PresentPerfect},
		{"They haven't called me yet.", PresentPerfect},
		{"I walked home yesterday.", PastSimple},
		{"He went to school.", PastSimple},
		{"You can swim well.", Modals},
		{"There is a cat on the roof.", ThereIs},
		{"There's no milk.", ThereIs},
		{"This box is bigger than that one.", Comparative},
		{"She is more careful than him.", Comparative},
		{"It is better than nothing.", Comparative},
		{"If it rains, we will stay home.", FirstConditional},
		{"If you come, I'll wait.", FirstConditional},
		{"If I were rich, I would travel.", SecondConditional},
		{"Do you like tea?", QuestionDo},
		{"Did he call you?", QuestionDo},
	}
	for _, c := range cases {
		if got := ids(c.text); !slices.Contains(got, c.want) {
			t.Errorf("%q: нашли %v, ждали %s", c.text, got, c.want)
		}
	}
}

func TestDetectNegative(t *testing.T) {
	cases := []struct{ text, not string }{
		{"I have a dog.", PresentPerfect},
		{"We have got a car.", PresentPerfect}, // have got — не Present Perfect
		{"He's a teacher.", PresentPerfect},
		{"It's raining now.", PresentPerfect},
		{"She is tired.", PastSimple},
		{"The boy is happy today.", PastSimple},
		{"Rather than wait, he left.", Comparative},
		{"I know no other way than this.", Comparative},
		{"The answer than came.", Comparative},
		{"Do it now.", QuestionDo},       // не вопрос
		{"Do you like tea.", QuestionDo}, // нет знака вопроса
		{"There he goes.", ThereIs},
		{"If it rains, we would stay home.", FirstConditional},
		{"If it rains, we will stay home.", SecondConditional},
		{"She can run.", ThereIs},
		{"He may have seen it.", PresentPerfect},        // модальный глагол + have
		{"I wonder if he will come.", FirstConditional}, // «ли» в косвенном вопросе
		{"I asked Tom if he would go.", SecondConditional},
		{"If I had known, I would have come.", SecondConditional}, // третье условное
	}
	for _, c := range cases {
		if got := ids(c.text); slices.Contains(got, c.not) {
			t.Errorf("%q: ложное срабатывание %s (%v)", c.text, c.not, got)
		}
	}
}

func TestPresentPerfectIsNotAlsoPastSimple(t *testing.T) {
	if got := ids("I have walked home."); slices.Contains(got, PastSimple) || !slices.Contains(got, PresentPerfect) {
		t.Fatalf("walked после have — Present Perfect, не Past Simple: %v", got)
	}
}

func TestDetectCollectsSentenceIndexesAndKeepsOrder(t *testing.T) {
	mk := func(text string) Sentence { return Sentence{Text: text, Tokens: exercises.CasedTokens(text)} }
	got := Detect([]Sentence{mk("I can swim."), mk("Do you like tea?"), mk("She can read."), mk("There is a cat.")})
	want := []Match{{ID: Modals, Sentences: []int{0, 2}}, {ID: ThereIs, Sentences: []int{3}}, {ID: QuestionDo, Sentences: []int{1}}}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i].ID != want[i].ID || !slices.Equal(got[i].Sentences, want[i].Sentences) {
			t.Fatalf("%d: %+v, ждали %+v", i, got[i], want[i])
		}
	}
	if len(Detect(nil)) != 0 {
		t.Fatal("без предложений ничего не находится")
	}
}
