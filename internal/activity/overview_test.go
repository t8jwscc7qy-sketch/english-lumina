package activity

import (
	"math"
	"testing"
	"time"
)

var ovToday = Day{2026, 10, 25}

func mins(n int) time.Duration { return time.Duration(n) * time.Minute }

func TestPaceEmpty(t *testing.T) {
	if got := Pace(nil, ovToday, 14); got != 0 {
		t.Fatalf("пусто: %v", got)
	}
}

func TestPaceFullWindow(t *testing.T) {
	m := map[Day]time.Duration{}
	for i := 0; i < 14; i++ {
		m[ovToday.AddDays(-i)] = mins(10)
	}
	if got := Pace(m, ovToday, 14); got != mins(10) {
		t.Fatalf("14 суток по 10 минут: %v", got)
	}
}

func TestPaceCountsEmptyDays(t *testing.T) {
	m := map[Day]time.Duration{}
	for i := 0; i < 7; i++ {
		m[ovToday.AddDays(-i)] = mins(20)
	}
	if got := Pace(m, ovToday, 14); got != mins(10) {
		t.Fatalf("7 из 14 суток по 20 минут: %v, ожидали 10 мин", got)
	}
}

func TestPaceIgnoresOutsideWindow(t *testing.T) {
	m := map[Day]time.Duration{ovToday.AddDays(-14): mins(100), ovToday.AddDays(1): mins(100)}
	if got := Pace(m, ovToday, 14); got != 0 {
		t.Fatalf("вне окна: %v", got)
	}
}

func TestPaceNonPositiveDays(t *testing.T) {
	if got := Pace(map[Day]time.Duration{ovToday: mins(30)}, ovToday, 0); got != 0 {
		t.Fatalf("days=0: %v", got)
	}
}

func TestCumulativeSeriesLength(t *testing.T) {
	s := CumulativeSeries(nil, ovToday, 90)
	if len(s) != 90 || s[0].Day != ovToday.AddDays(-89) || s[89].Day != ovToday {
		t.Fatalf("длина %d, первая %+v, последняя %+v", len(s), s[0].Day, s[len(s)-1].Day)
	}
}

func TestCumulativeSeriesMonotonic(t *testing.T) {
	m := map[Day]time.Duration{ovToday.AddDays(-5): mins(60), ovToday.AddDays(-1): mins(30)}
	s := CumulativeSeries(m, ovToday, 90)
	for i := 1; i < len(s); i++ {
		if s[i].CumulativeHours < s[i-1].CumulativeHours {
			t.Fatalf("убывает на %d: %v < %v", i, s[i].CumulativeHours, s[i-1].CumulativeHours)
		}
	}
	if got := s[89].CumulativeHours; math.Abs(got-1.5) > 1e-9 {
		t.Fatalf("итог: %v ч, ожидали 1,5", got)
	}
	if s[89-5].CumulativeHours != 1 || s[89-5].Minutes != mins(60) {
		t.Fatalf("сутки today-5: %+v", s[89-5])
	}
	if s[89-2].CumulativeHours != 1 || s[89-2].Minutes != 0 {
		t.Fatalf("пустые сутки не уменьшают и не добавляют: %+v", s[89-2])
	}
}

func TestCumulativeSeriesIncludesEarlier(t *testing.T) {
	m := map[Day]time.Duration{ovToday.AddDays(-200): mins(120)}
	s := CumulativeSeries(m, ovToday, 90)
	if s[0].CumulativeHours != 2 {
		t.Fatalf("занятия до окна входят в накопленные часы: %v", s[0].CumulativeHours)
	}
}

func TestStartedOn(t *testing.T) {
	m := map[Day]time.Duration{
		ovToday.AddDays(-10): mins(5),
		ovToday.AddDays(-30): mins(1),
		ovToday.AddDays(-40): 0, // нулевые сутки не считаются
	}
	d, ok := StartedOn(m)
	if !ok || d != ovToday.AddDays(-30) {
		t.Fatalf("старт: %+v %v", d, ok)
	}
	if _, ok := StartedOn(nil); ok {
		t.Fatal("нет занятий — false")
	}
	if _, ok := StartedOn(map[Day]time.Duration{ovToday: 0}); ok {
		t.Fatal("только нулевые сутки — false")
	}
}

func TestDaysSince(t *testing.T) {
	if got := DaysSince(ovToday, ovToday); got != 1 {
		t.Fatalf("сегодня: %d", got)
	}
	if got := DaysSince(ovToday.AddDays(-1), ovToday); got != 2 {
		t.Fatalf("вчера: %d", got)
	}
	if got := DaysSince(Day{2026, 9, 28}, Day{2026, 10, 2}); got != 5 {
		t.Fatalf("через границу месяца: %d", got)
	}
}
