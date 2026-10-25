package activity

import (
	"slices"
	"testing"
	"time"
)

var msToday = Day{2026, 10, 25}

func find(t *testing.T, all []Milestone, id string) Milestone {
	t.Helper()
	for _, m := range all {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("нет вехи %s", id)
	return Milestone{}
}

func TestMilestonesEmpty(t *testing.T) {
	all, newToday := ComputeMilestones(MilestoneInput{}, msToday)
	if len(all) != 19 {
		t.Fatalf("вех %d, ожидали 19 (6 серия + 7 часы + 3 ответы + 3 первое)", len(all))
	}
	for _, m := range all {
		if m.ReachedOn != nil || m.Value != 0 {
			t.Fatalf("пустая история: %+v", m)
		}
	}
	if newToday == nil || len(newToday) != 0 {
		t.Fatalf("newToday должен быть пустым срезом, не nil: %#v", newToday)
	}
}

func TestMilestoneHoursThreshold(t *testing.T) {
	// 30 минут вчера и 30 минут сегодня: 1 час накоплен сегодня (на пороге), а 59 минут — ещё нет
	in := MilestoneInput{Minutes: map[Day]time.Duration{msToday.AddDays(-1): 30 * time.Minute, msToday: 30 * time.Minute}}
	all, newToday := ComputeMilestones(in, msToday)
	h1 := find(t, all, "hours-1")
	if h1.ReachedOn == nil || *h1.ReachedOn != msToday || h1.Value != 1 {
		t.Fatalf("hours-1: %+v", h1)
	}
	if !slices.Contains(newToday, "hours-1") {
		t.Fatalf("newToday %v", newToday)
	}
	in = MilestoneInput{Minutes: map[Day]time.Duration{msToday.AddDays(-1): 30 * time.Minute, msToday: 29 * time.Minute}}
	all, _ = ComputeMilestones(in, msToday)
	if m := find(t, all, "hours-1"); m.ReachedOn != nil {
		t.Fatalf("59 минут: не достигнута: %+v", m)
	}
}

func TestMilestoneAnswersThreshold(t *testing.T) {
	in := MilestoneInput{Answers: map[Day]int{msToday.AddDays(-2): 60, msToday.AddDays(-1): 40}}
	all, _ := ComputeMilestones(in, msToday)
	a := find(t, all, "answers-100")
	if a.ReachedOn == nil || *a.ReachedOn != msToday.AddDays(-1) || a.Value != 100 {
		t.Fatalf("answers-100 на пороге: %+v", a)
	}
	in = MilestoneInput{Answers: map[Day]int{msToday.AddDays(-2): 60, msToday.AddDays(-1): 39}}
	all, _ = ComputeMilestones(in, msToday)
	if a := find(t, all, "answers-100"); a.ReachedOn != nil || a.Value != 99 {
		t.Fatalf("99 ответов: %+v", a)
	}
}

func TestMilestoneStreak(t *testing.T) {
	in := MilestoneInput{Minutes: map[Day]time.Duration{}}
	for i := 7; i >= 1; i-- {
		in.Minutes[msToday.AddDays(-i)] = MinimumDayMinutes * time.Minute
	}
	all, newToday := ComputeMilestones(in, msToday)
	s7 := find(t, all, "streak-7")
	if s7.ReachedOn == nil || *s7.ReachedOn != msToday.AddDays(-1) || s7.Value != 7 {
		t.Fatalf("streak-7 достигнута в седьмые сутки: %+v", s7)
	}
	if s3 := find(t, all, "streak-3"); s3.ReachedOn == nil || *s3.ReachedOn != msToday.AddDays(-5) {
		t.Fatalf("streak-3: %+v", s3)
	}
	if m := find(t, all, "streak-14"); m.ReachedOn != nil || m.Value != 7 {
		t.Fatalf("streak-14: %+v", m)
	}
	if len(newToday) != 0 {
		t.Fatalf("вчерашние вехи не новые: %v", newToday)
	}
}

func TestMilestoneFirst(t *testing.T) {
	d := msToday.AddDays(-3)
	in := MilestoneInput{FirstCard: &d}
	all, _ := ComputeMilestones(in, msToday)
	if m := find(t, all, "first-card"); m.ReachedOn == nil || *m.ReachedOn != d || m.Value != 1 {
		t.Fatalf("first-card: %+v", m)
	}
	if m := find(t, all, "first-exercise"); m.ReachedOn != nil || m.Value != 0 {
		t.Fatalf("first-exercise: %+v", m)
	}
}

func TestMilestoneNewTodayOrder(t *testing.T) {
	// сегодня сразу 5 часов занятий, 100 ответов и первая карточка: серии нет (один день), порядок часы, ответы, первое
	card := msToday
	in := MilestoneInput{
		Minutes:   map[Day]time.Duration{msToday: 5 * time.Hour},
		Answers:   map[Day]int{msToday: 100},
		FirstCard: &card,
	}
	_, newToday := ComputeMilestones(in, msToday)
	want := []string{"hours-5", "hours-1", "answers-100", "first-card"}
	if !slices.Equal(newToday, want) {
		t.Fatalf("newToday %v, ожидали %v (серия, часы, ответы, первое; внутри линейки больший порог первым)", newToday, want)
	}
}

func TestMilestoneNewTodayStreak(t *testing.T) {
	// 7 суток подряд по 10 минут: серия 7 достигнута сегодня, серия 3 раньше, час накоплен вчера (60 минут к шестым суткам)
	in := MilestoneInput{Minutes: map[Day]time.Duration{}}
	for i := 6; i >= 0; i-- {
		in.Minutes[msToday.AddDays(-i)] = MinimumDayMinutes * time.Minute
	}
	_, newToday := ComputeMilestones(in, msToday)
	if !slices.Equal(newToday, []string{"streak-7"}) {
		t.Fatalf("newToday %v, ожидали только streak-7", newToday)
	}
}

func TestMilestoneIDsAndTitles(t *testing.T) {
	all, _ := ComputeMilestones(MilestoneInput{}, msToday)
	want := map[string]string{
		"streak-3": "Серия 3 дня", "streak-7": "Серия 7 дней", "streak-365": "Серия 365 дней",
		"hours-1": "1 час занятий", "hours-5": "5 часов занятий", "hours-10": "10 часов занятий", "hours-250": "250 часов занятий",
		"answers-100": "100 ответов", "answers-1000": "1000 ответов",
		"first-card": "Первая карточка", "first-exercise": "Первое упражнение", "first-mistake-closed": "Первая закрытая ошибка",
	}
	for id, title := range want {
		if m := find(t, all, id); m.Title != title {
			t.Errorf("%s: %q, ожидали %q", id, m.Title, title)
		}
	}
	kinds := []string{}
	for _, m := range all {
		if len(kinds) == 0 || kinds[len(kinds)-1] != m.Kind {
			kinds = append(kinds, m.Kind)
		}
	}
	if !slices.Equal(kinds, []string{"streak", "hours", "answers", "first"}) {
		t.Fatalf("порядок линеек %v", kinds)
	}
}
