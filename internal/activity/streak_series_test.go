package activity

import (
	"math/rand/v2"
	"testing"
	"time"
)

var ssToday = Day{2026, 10, 25}

func doneDays(today Day, back ...int) map[Day]time.Duration {
	m := map[Day]time.Duration{}
	for _, k := range back {
		m[today.AddDays(-k)] = MinimumDayMinutes * time.Minute
	}
	return m
}

func TestComputeStreakNMatchesComputeStreak(t *testing.T) {
	cases := [][]int{{}, {1, 2, 3}, {0, 1}, {2, 3}, {3, 4}, {1, 3, 4}, {2, 3, 4, 5, 6, 7, 8, 10}}
	for _, c := range cases {
		m := doneDays(ssToday, c...)
		if got, want := ComputeStreakN(m, ssToday, HistoryDays), ComputeStreak(m, ssToday); got.Days != want.Days || (got.FreezeUsedOn == nil) != (want.FreezeUsedOn == nil) {
			t.Fatalf("%v: ComputeStreakN=%+v, ComputeStreak=%+v", c, got, want)
		}
	}
}

func TestComputeStreakNLongCap(t *testing.T) {
	back := make([]int, 0, 100)
	for i := 1; i <= 100; i++ {
		back = append(back, i)
	}
	m := doneDays(ssToday, back...)
	if got := ComputeStreakN(m, ssToday, 365).Days; got != 100 {
		t.Fatalf("потолок 365: %d, ожидали 100", got)
	}
	if got := ComputeStreak(m, ssToday).Days; got != HistoryDays {
		t.Fatalf("ComputeStreak ограничен %d: %d", HistoryDays, got)
	}
}

func TestStreakSeriesBasic(t *testing.T) {
	m := doneDays(ssToday, 2, 1, 0)
	s := StreakSeries(m, ssToday.AddDays(-2), ssToday)
	if s[ssToday.AddDays(-2)] != 1 || s[ssToday.AddDays(-1)] != 2 || s[ssToday] != 3 {
		t.Fatalf("серия по дням: %+v", s)
	}
}

func TestStreakSeriesFreeze(t *testing.T) {
	// выполнены 5, 4, пропуск 3, выполнены 2, 1: один пропуск прощён
	m := doneDays(ssToday, 5, 4, 2, 1)
	s := StreakSeries(m, ssToday.AddDays(-5), ssToday.AddDays(-1))
	if s[ssToday.AddDays(-1)] != 4 {
		t.Fatalf("пропуск прощён, серия 4: %+v", s)
	}
	// второй пропуск в 7 суток обрывает: выполнены 6, пропуск 5, выполнен 4, пропуск 3, выполнены 2, 1
	m = doneDays(ssToday, 6, 4, 2, 1)
	s = StreakSeries(m, ssToday.AddDays(-6), ssToday.AddDays(-1))
	if got := s[ssToday.AddDays(-1)]; got != 3 {
		t.Fatalf("прощается более поздний пропуск (−3), ранний (−5) обрывает: серия −1, −2, −4 = 3, получили %d", got)
	}
}

func TestStreakSeriesLastDayNotDoneDoesNotBreak(t *testing.T) {
	m := doneDays(ssToday, 3, 2, 1)
	s := StreakSeries(m, ssToday.AddDays(-3), ssToday)
	if s[ssToday] != 3 {
		t.Fatalf("сегодня без минимума серию не обнуляет: %d", s[ssToday])
	}
}

func TestStreakSeriesEqualsBackwardOnRandomHistories(t *testing.T) {
	for seed := uint64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 1))
		n := 5 + rng.IntN(116)
		p := 0.3 + rng.Float64()*0.65
		from := ssToday.AddDays(-n + 1)
		m := map[Day]time.Duration{}
		for i := 0; i < n; i++ {
			if rng.Float64() < p {
				m[from.AddDays(i)] = MinimumDayMinutes * time.Minute
			}
		}
		series := StreakSeries(m, from, ssToday)
		for i := 0; i < n; i++ {
			e := from.AddDays(i)
			upTo := map[Day]time.Duration{}
			for d, v := range m {
				if d.ordinal() <= e.ordinal() {
					upTo[d] = v
				}
			}
			if got, want := series[e], ComputeStreakN(upTo, e, 400).Days; got != want {
				t.Fatalf("seed %d, сутки %d: ряд %d, обратный счёт %d", seed, i, got, want)
			}
		}
	}
}
