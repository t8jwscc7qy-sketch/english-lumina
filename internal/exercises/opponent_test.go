package exercises

import "testing"

func TestOpponent(t *testing.T) {
	tests := []struct {
		topics []string
		mu     float64
	}{
		{[]string{"grammar:articles", "level:A1"}, 800},
		{[]string{"level:B2"}, 1400},
		{[]string{"level:C2", "level:A1"}, 1800},
		{[]string{"grammar:articles"}, 1500},
		{[]string{"level:Z9"}, 1500},
		{nil, 1500},
	}
	for _, tt := range tests {
		got := Opponent(tt.topics)
		if got.Mu != tt.mu || got.Phi != 100 || got.Sigma != 0.06 {
			t.Errorf("Opponent(%v) = %+v, ждали Mu %v", tt.topics, got, tt.mu)
		}
	}
}
