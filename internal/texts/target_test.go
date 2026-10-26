package texts

import (
	"math"
	"testing"
)

func TestNextGoal(t *testing.T) {
	cases := []struct {
		cur        float64
		understood string
		want       float64
	}{
		{0.85, UnderstoodEasy, 0.83}, {0.85, UnderstoodHard, 0.87}, {0.85, UnderstoodOK, 0.85},
		{0.80, UnderstoodEasy, 0.80}, {0.90, UnderstoodHard, 0.90}, {0.81, UnderstoodEasy, 0.80},
		{0.89, UnderstoodHard, 0.90}, {0.85, "bogus", 0.85}, {0.5, UnderstoodOK, 0.80}, {0.99, UnderstoodOK, 0.90},
	}
	for _, c := range cases {
		if got := NextGoal(c.cur, c.understood); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("NextGoal(%v, %q) = %v, ждали %v", c.cur, c.understood, got, c.want)
		}
	}
}

func TestUnderstanding(t *testing.T) {
	known := map[string]bool{"cat": true, "dog": true}
	toks := [][]string{{"the", "cat", "sleeps"}, {"a", "dog", "runs", "fast"}} // учитываются cat, sleeps, dog, runs, fast; известных 2 + служебные the, a
	got := Understanding(toks, known)
	// the и a — служебные (известные); cat, dog известны; sleeps, runs, fast — нет: 4 из 7
	if math.Abs(got-4.0/7) > 1e-9 {
		t.Fatalf("понимание %v, ждали 4/7", got)
	}
	if Understanding(nil, known) != 1 {
		t.Fatal("пустой текст — полное понимание")
	}
}
