package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const milestonesPath = "/api/v1/milestones"

func milestone(t *testing.T, l api.MilestoneList, id string) api.Milestone {
	t.Helper()
	for _, m := range l.Milestones {
		if m.Id == id {
			return m
		}
	}
	t.Fatalf("нет вехи %s", id)
	return api.Milestone{}
}

func TestBuildMilestonesEmpty(t *testing.T) {
	l := buildMilestones(nil, progressNow, "")
	if len(l.Milestones) != 19 || l.NewToday == nil || len(l.NewToday) != 0 {
		t.Fatalf("вех %d, newToday %#v", len(l.Milestones), l.NewToday)
	}
	for _, m := range l.Milestones {
		if m.ReachedOn != nil || m.Progress.Value != 0 || m.Progress.Target <= 0 {
			t.Fatalf("пусто: %+v", m)
		}
	}
}

func TestBuildMilestonesFirstEvents(t *testing.T) {
	ev := []db.ListActivityEventsRow{
		activityRow("card.reviewed", time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC), false),
		activityRow("exercise.answered", time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC), true),
		activityRow("mistake.closed", time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC), false),
		activityRow("exercise.answered", time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC), true), // раньше первого по порядку в списке
	}
	l := buildMilestones(ev, progressNow, "")
	for id, want := range map[string]string{"first-card": "2026-10-20", "first-exercise": "2026-10-19", "first-mistake-closed": "2026-10-22"} {
		m := milestone(t, l, id)
		if m.ReachedOn == nil || *m.ReachedOn != want || m.Progress.Value != 1 || m.Progress.Target != 1 {
			t.Fatalf("%s: %+v, ожидали %s", id, m, want)
		}
	}
}

func TestBuildMilestonesHoursAndAnswers(t *testing.T) {
	start := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	// 61 ответ с паузой 1 минута: 20 + 60×60 = 3620 с — час занятий достигнут сегодня; ответов 61 (до сотни не дошло)
	l := buildMilestones(burst(start, 61, time.Minute), progressNow, "")
	h := milestone(t, l, "hours-1")
	if h.ReachedOn == nil || *h.ReachedOn != "2026-10-25" {
		t.Fatalf("hours-1: %+v", h)
	}
	if a := milestone(t, l, "answers-100"); a.ReachedOn != nil || a.Progress.Value != 61 {
		t.Fatalf("answers-100: %+v", a)
	}
	if !slices.Contains(l.NewToday, "hours-1") || !slices.Contains(l.NewToday, "first-exercise") {
		t.Fatalf("newToday %v", l.NewToday)
	}
	// на пороге ответов
	l = buildMilestones(burst(start, 100, time.Second), progressNow, "")
	if a := milestone(t, l, "answers-100"); a.ReachedOn == nil || a.Progress.Value != 100 {
		t.Fatalf("100 ответов: %+v", a)
	}
}

func TestBuildMilestonesStreakAndNewToday(t *testing.T) {
	var ev []db.ListActivityEventsRow
	for _, back := range []int{3, 2, 1, 0} { // 4 суток подряд по 10 минут, включая сегодня
		ev = append(ev, dayEvents(back)...)
	}
	l := buildMilestones(ev, progressNow, "")
	s3 := milestone(t, l, "streak-3")
	if s3.ReachedOn == nil || *s3.ReachedOn != "2026-10-24" {
		t.Fatalf("streak-3 достигнута вчера (третьи сутки): %+v", s3)
	}
	if slices.Contains(l.NewToday, "streak-3") {
		t.Fatalf("вчерашняя веха не новая: %v", l.NewToday)
	}
	if milestone(t, l, "streak-7").Progress.Value != 4 {
		t.Fatalf("прогресс до 7 дней: %+v", milestone(t, l, "streak-7").Progress)
	}
}

func TestBuildMilestonesTimezone(t *testing.T) {
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC) // в Москве 04:00 25 октября
	ev := []db.ListActivityEventsRow{activityRow("exercise.answered", time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC), true)}
	l := buildMilestones(ev, now, "Europe/Moscow")
	if m := milestone(t, l, "first-exercise"); m.ReachedOn == nil || *m.ReachedOn != "2026-10-25" || !slices.Contains(l.NewToday, "first-exercise") {
		t.Fatalf("в Москве событие 02:30 25 октября — сегодня: %+v %v", m, l.NewToday)
	}
	l = buildMilestones(ev, now, "")
	if m := milestone(t, l, "first-exercise"); m.ReachedOn == nil || *m.ReachedOn != "2026-10-24" || len(l.NewToday) != 0 {
		t.Fatalf("в UTC то же событие вчера: %+v %v", m, l.NewToday)
	}
}

func getMilestones(t *testing.T, e env) (api.MilestoneList, string) {
	t.Helper()
	token := login(t, e)
	rec := e.do(http.MethodGet, milestonesPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.MilestoneList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out, rec.Body.String()
}

func TestMilestonesEndpoint(t *testing.T) {
	e := progressEnv(t)
	for _, r := range dayEvents(0) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 1, Row: r})
	}
	for _, r := range dayEvents(0) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 2, Row: r}) // чужое
	}
	l, body := getMilestones(t, e)
	if !strings.Contains(body, `"newToday":[`) || strings.Contains(body, `"newToday":null`) {
		t.Fatalf("newToday должен быть массивом: %s", body)
	}
	if a := milestone(t, l, "answers-100"); a.Progress.Value != 11 {
		t.Fatalf("только свои 11 ответов: %+v", a)
	}
}

func TestMilestonesEndpointUnauthorized(t *testing.T) {
	e := progressEnv(t)
	if rec := e.do(http.MethodGet, milestonesPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d", rec.Code)
	}
}

// 365 суток по 60 ответов (около 22 тысяч событий): расчёт вех должен укладываться в долю секунды.
func TestBuildMilestonesManyEventsIsFast(t *testing.T) {
	var ev []db.ListActivityEventsRow
	for d := 0; d < 365; d++ {
		day := progressNow.AddDate(0, 0, -d)
		start := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, time.UTC)
		ev = append(ev, burst(start, 60, 20*time.Second)...)
	}
	begin := time.Now()
	l := buildMilestones(ev, progressNow, "Europe/Moscow")
	if elapsed := time.Since(begin); elapsed > 500*time.Millisecond {
		t.Fatalf("%d событий считались %v, ожидали быстрее 500 мс", len(ev), elapsed)
	}
	if m := milestone(t, l, "streak-365"); m.ReachedOn == nil {
		t.Fatalf("365 суток подряд по 20 минут: серия 365 достигнута: %+v", m)
	}
}

// Обрезанная история (самое раннее событие на первых сутках окна): вехи до окна «достигаются» заново каждый день,
// поэтому «новых сегодня» не отдаём — вспышки не будет.
func TestBuildMilestonesTruncatedHistoryHasNoNewToday(t *testing.T) {
	day := func(back int) time.Time {
		d := progressNow.AddDate(0, 0, -back)
		return time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, time.UTC)
	}
	today := burst(day(0), 100, time.Second) // сегодня 100 ответов: answers-100 достигается сегодня
	truncated := append([]db.ListActivityEventsRow{activityRow("exercise.answered", day(365), true)}, today...)
	l := buildMilestones(truncated, progressNow, "")
	if len(l.NewToday) != 0 {
		t.Fatalf("история обрезана: newToday должен быть пустым, получили %v", l.NewToday)
	}
	// контроль: то же самое, но история началась 10 суток назад — вехи сегодня новые
	recent := append([]db.ListActivityEventsRow{activityRow("exercise.answered", day(10), true)}, today...)
	l = buildMilestones(recent, progressNow, "")
	if !slices.Contains(l.NewToday, "answers-100") {
		t.Fatalf("необрезанная история: ждём answers-100 в newToday, получили %v", l.NewToday)
	}
}
