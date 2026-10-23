package srs

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// ordered: «забыл» возвращается раньше всех, «легко» позже всех.
func ordered(p map[Rating]time.Duration) bool {
	return p[Again] < p[Hard] && p[Hard] <= p[Good] && p[Good] < p[Easy]
}

func TestNewCard(t *testing.T) {
	c := New(t0)
	if c.State != StateNew || !c.Due.Equal(t0) || c.Reps != 0 || c.Lapses != 0 || !c.LastReview.IsZero() {
		t.Fatalf("новая карточка: %+v", c)
	}
}

func TestPreviewOnNewCardOrdersIntervals(t *testing.T) {
	p, err := Preview(New(t0), t0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Rating{Again, Hard, Good, Easy} {
		if p[r] <= 0 {
			t.Fatalf("интервал для %d не положительный: %v", r, p[r])
		}
	}
	if !ordered(p) {
		t.Fatalf("порядок интервалов нарушен: again=%v hard=%v good=%v easy=%v", p[Again], p[Hard], p[Good], p[Easy])
	}
}

func TestReviewGoodOnNewCard(t *testing.T) {
	c, err := Review(New(t0), Good, t0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Reps != 1 || !c.Due.After(t0) || !c.LastReview.Equal(t0) || c.State == StateNew {
		t.Fatalf("после «хорошо»: %+v", c)
	}
}

// goodUntilReview повторяет «хорошо» в момент срока, пока карточка не станет зрелой.
func goodUntilReview(t *testing.T) (Card, time.Time) {
	t.Helper()
	c, now := New(t0), t0
	for i := 0; i < 10 && c.State != StateReview; i++ {
		var err error
		if c, err = Review(c, Good, now); err != nil {
			t.Fatal(err)
		}
		now = c.Due
	}
	if c.State != StateReview {
		t.Fatalf("карточка не стала зрелой: %+v", c)
	}
	return c, now
}

func TestGoodSequenceGrowsIntervalsAndStability(t *testing.T) {
	c, now := New(t0), t0
	prevInterval, prevStability := time.Duration(0), 0.0
	for i := 0; i < 6; i++ {
		var err error
		if c, err = Review(c, Good, now); err != nil {
			t.Fatal(err)
		}
		interval := c.Due.Sub(now)
		if interval < prevInterval {
			t.Fatalf("шаг %d: интервал уменьшился %v → %v", i, prevInterval, interval)
		}
		if c.Stability < prevStability {
			t.Fatalf("шаг %d: стабильность упала %v → %v", i, prevStability, c.Stability)
		}
		prevInterval, prevStability = interval, c.Stability
		now = c.Due
	}
	if c.State != StateReview || c.Reps != 6 || prevInterval < 24*time.Hour {
		t.Fatalf("итог: %+v, последний интервал %v", c, prevInterval)
	}
}

func TestAgainOnMatureCardCountsLapse(t *testing.T) {
	c, now := goodUntilReview(t)
	lapsed, err := Review(c, Again, now)
	if err != nil {
		t.Fatal(err)
	}
	if lapsed.Lapses != c.Lapses+1 || lapsed.State != StateRelearning {
		t.Fatalf("после «забыл»: lapses %d→%d, состояние %d", c.Lapses, lapsed.Lapses, lapsed.State)
	}
	if !lapsed.Due.After(now) {
		t.Fatalf("срок после «забыл» не в будущем: %v", lapsed.Due)
	}
	good, err := Review(c, Good, now)
	if err != nil {
		t.Fatal(err)
	}
	if !lapsed.Due.Before(good.Due) {
		t.Fatalf("после «забыл» карточка должна вернуться раньше, чем после «хорошо»: %v и %v", lapsed.Due, good.Due)
	}
}

func TestEasyIsLaterThanHardOnMatureCard(t *testing.T) {
	c, now := goodUntilReview(t)
	p, err := Preview(c, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ordered(p) {
		t.Fatalf("зрелая карточка: again=%v hard=%v good=%v easy=%v", p[Again], p[Hard], p[Good], p[Easy])
	}
}

func TestBadRatingsAreRejected(t *testing.T) {
	c := New(t0)
	for _, r := range []Rating{0, 5, -1, 100} {
		got, err := Review(c, r, t0)
		if !errors.Is(err, ErrBadRating) {
			t.Fatalf("оценка %d: ошибка %v, ожидали ErrBadRating", r, err)
		}
		if got != (Card{}) {
			t.Fatalf("при ошибке должна вернуться пустая карточка: %+v", got)
		}
	}
}

func TestReviewAndPreviewDoNotMutateInput(t *testing.T) {
	c, _ := goodUntilReview(t)
	before := c
	if _, err := Review(c, Again, c.Due); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(c, c.Due); err != nil {
		t.Fatal(err)
	}
	if c != before {
		t.Fatalf("карточка изменилась: %+v → %+v", before, c)
	}
}

func TestInvalidCardIsRejected(t *testing.T) {
	bad := New(t0)
	bad.Reps = -1
	if _, err := Review(bad, Good, t0); err == nil {
		t.Fatal("карточка с отрицательным числом повторений должна отклоняться")
	}
	if _, err := Preview(bad, t0); err == nil {
		t.Fatal("Preview отрицательной карточки должен давать ошибку")
	}
}
