package rating

import (
	"math"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	r := New()
	if r.Mu != 1500 || r.Phi != 350 || r.Sigma != 0.06 {
		t.Fatalf("New() = %+v", r)
	}
}

func TestUpdateWinRaisesRating(t *testing.T) {
	r := Update(Rating{1500, 200, 0.06}, Rating{Mu: 1400, Phi: 30, Sigma: 0.06}, 1)
	if r.Mu <= 1500 || r.Phi >= 200 {
		t.Fatalf("после победы: %+v", r)
	}
}

func TestUpdateLossLowersRating(t *testing.T) {
	r := Update(Rating{1500, 200, 0.06}, Rating{Mu: 1700, Phi: 300, Sigma: 0.06}, 0)
	if r.Mu >= 1500 {
		t.Fatalf("после поражения: %+v", r)
	}
}

func TestUpdateKeepsBounds(t *testing.T) {
	r := New()
	for i := range 200 {
		r = Update(r, Rating{Mu: 1500, Phi: 50, Sigma: 0.06}, float64(i%2))
		if r.Phi > 350 || r.Sigma <= 0 || math.IsNaN(r.Mu) {
			t.Fatalf("шаг %d: %+v", i, r)
		}
	}
}

// Пример из статьи Глика: три соперника за один период.
func TestUpdatePeriodGlickExample(t *testing.T) {
	r := updatePeriod(Rating{1500, 200, 0.06}, []match{
		{Rating{Mu: 1400, Phi: 30}, 1},
		{Rating{Mu: 1550, Phi: 100}, 0},
		{Rating{Mu: 1700, Phi: 300}, 0},
	})
	if math.Abs(r.Mu-1464.06) > 0.01 || math.Abs(r.Phi-151.52) > 0.01 || math.Abs(r.Sigma-0.05999) > 0.00001 {
		t.Fatalf("пример Глика: %+v", r)
	}
}
