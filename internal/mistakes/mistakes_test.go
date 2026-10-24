package mistakes

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func TestOpened(t *testing.T) {
	s := Opened(t0)
	if s.Streak != 0 || s.Closed || !s.NextDue.Equal(t0.Add(24*time.Hour)) {
		t.Fatalf("Opened = %+v", s)
	}
}

func TestAdvanceCorrectFirst(t *testing.T) {
	s := Advance(Opened(t0), true, t0)
	if s.Streak != 1 || s.Closed || !s.NextDue.Equal(t0.Add(72*time.Hour)) {
		t.Fatalf("Advance = %+v", s)
	}
}

func TestAdvanceCorrectSecondCloses(t *testing.T) {
	s := Advance(State{Streak: 1, NextDue: t0}, true, t0)
	if s.Streak != 2 || !s.Closed {
		t.Fatalf("Advance = %+v", s)
	}
}

func TestAdvanceWrongResets(t *testing.T) {
	s := Advance(State{Streak: 1, NextDue: t0}, false, t0)
	if s.Streak != 0 || s.Closed || !s.NextDue.Equal(t0.Add(24*time.Hour)) {
		t.Fatalf("Advance = %+v", s)
	}
}

func TestShouldServeRepeat(t *testing.T) {
	tests := []struct {
		name    string
		repeats int
		last2   bool
		hasDue  bool
		want    bool
	}{
		{"нет наступивших", 0, false, false, false},
		{"лимит за сегодня", MaxRepeatsPerDay, false, true, false},
		{"два последних повторы", 1, true, true, false},
		{"можно", 4, false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldServeRepeat(tt.repeats, tt.last2, tt.hasDue); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDayStart(t *testing.T) {
	now := time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		tz   string
		want time.Time
	}{
		{"Москва", "Europe/Moscow", time.Date(2026, 10, 24, 21, 0, 0, 0, time.UTC)},
		{"неизвестный пояс", "Nowhere/Nothing", time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)},
		{"пусто", "", time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DayStart(now, tt.tz)
			if !got.Equal(tt.want) || got.Location() != time.UTC {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
