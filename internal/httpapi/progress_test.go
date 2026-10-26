package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const progressPath = "/api/v1/progress/today"

var progressNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func activityRow(typ string, at time.Time, correct bool) db.ListActivityEventsRow {
	return db.ListActivityEventsRow{Type: typ, OccurredAt: pgtype.Timestamptz{Time: at, Valid: true}, Correct: correct}
}

// burst — n ответов на упражнения подряд с паузой gap, начиная с start (первый ответ даёт 20 с).
func burst(start time.Time, n int, gap time.Duration) []db.ListActivityEventsRow {
	var out []db.ListActivityEventsRow
	for i := range n {
		out = append(out, activityRow("exercise.answered", start.Add(time.Duration(i)*gap), true))
	}
	return out
}

// minimumDay — события, которые дают ровно минимальный день (20 + 9×60 + 40 = 600 с).
func minimumDay(start time.Time) []db.ListActivityEventsRow {
	out := burst(start, 10, time.Minute)
	return append(out, activityRow("exercise.answered", start.Add(9*time.Minute+40*time.Second), true))
}

func TestBuildProgressEmpty(t *testing.T) {
	p := buildProgress(nil, progressNow, "", 15)
	if p.Minutes != 0 || p.MinimumDay != 10 || p.GoalMinutes != 15 || p.DoneToday || p.StreakDays != 0 || p.FreezeUsedOn != nil || p.Today != (api.TodayCounts{}) {
		t.Fatalf("пустой журнал: %+v", p)
	}
}

func TestBuildProgressMinutesAndCounts(t *testing.T) {
	base := progressNow.Add(-time.Hour)
	events := []db.ListActivityEventsRow{
		activityRow("exercise.answered", base, true),
		activityRow("card.reviewed", base.Add(30*time.Second), false),
		activityRow("exercise.answered", base.Add(60*time.Second), false),
		activityRow("card.reviewed", base.Add(90*time.Second), false),
		activityRow("exercise.answered", base.Add(120*time.Second), true),
		activityRow("mistake.opened", base.Add(61*time.Second), false),
		activityRow("mistake.closed", base.Add(121*time.Second), false),
	}
	p := buildProgress(events, progressNow, "", 15)
	// минуты только по ответам и карточкам: 20 + 30×4 = 140 с ≈ 2,3 мин
	if p.Minutes != 2.3 {
		t.Errorf("минуты %v, ожидали 2.3", p.Minutes)
	}
	want := api.TodayCounts{Answers: 3, Correct: 2, CardsReviewed: 2, MistakesOpened: 1, MistakesClosed: 1}
	if p.Today != want {
		t.Errorf("счётчики %+v, ожидали %+v", p.Today, want)
	}
}

func TestBuildProgressDoneTodayBoundary(t *testing.T) {
	start := progressNow.Add(-time.Hour)
	events := minimumDay(start) // ровно 600 с
	if p := buildProgress(events, progressNow, "", 15); !p.DoneToday || p.Minutes != 10 {
		t.Fatalf("ровно 10 минут: %+v", p)
	}
	short := minimumDay(start)
	short[len(short)-1].OccurredAt = pgtype.Timestamptz{Time: start.Add(9*time.Minute + 39*time.Second), Valid: true} // 599 с
	if p := buildProgress(short, progressNow, "", 15); p.DoneToday || p.Minutes != 10 {
		// 599 с округляются до 10,0 для показа, но минимум ещё не выполнен
		t.Fatalf("599 с: doneToday=%v минуты=%v", p.DoneToday, p.Minutes)
	}
}

func TestBuildProgressStreakAndFreeze(t *testing.T) {
	day := func(back int) time.Time {
		return progressNow.Add(-time.Duration(back) * 24 * time.Hour).Add(-2 * time.Hour)
	}
	var events []db.ListActivityEventsRow
	events = append(events, minimumDay(day(2))...)
	events = append(events, minimumDay(day(3))...) // вчера (back=1) пропущено
	p := buildProgress(events, progressNow, "", 15)
	if p.StreakDays != 2 || p.FreezeUsedOn == nil || *p.FreezeUsedOn != "2026-10-24" {
		t.Fatalf("серия %d, заморозка %v", p.StreakDays, p.FreezeUsedOn)
	}
	// сегодня минимум тоже выполнен → серия 3
	events = append(events, minimumDay(progressNow.Add(-3*time.Hour))...)
	if p := buildProgress(events, progressNow, "", 15); p.StreakDays != 3 || !p.DoneToday {
		t.Fatalf("с сегодняшним днём: %+v", p)
	}
}

func TestBuildProgressTimezone(t *testing.T) {
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC) // в Москве 04:00 25 октября
	ev := []db.ListActivityEventsRow{activityRow("exercise.answered", time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC), true)}
	if p := buildProgress(ev, now, "Europe/Moscow", 15); p.Today.Answers != 1 {
		t.Errorf("в Москве событие 02:30 25 октября — сегодня: %+v", p.Today)
	}
	if p := buildProgress(ev, now, "", 15); p.Today.Answers != 0 {
		t.Errorf("в UTC то же событие — вчера: %+v", p.Today)
	}
}

func progressEnv(t *testing.T) env {
	t.Helper()
	store := newFakeStore(t)
	srv := newAuthServer(store, auth.NewThrottle(5, time.Minute, time.Now))
	srv.now = func() time.Time { return progressNow }
	h := api.HandlerWithOptions(srv, api.StdHTTPServerOptions{BaseRouter: http.NewServeMux(), Middlewares: []api.MiddlewareFunc{checkOrigin}})
	return env{h: h, store: store}
}

func getProgress(t *testing.T, e env) api.ProgressToday {
	t.Helper()
	token := login(t, e)
	rec := e.do(http.MethodGet, progressPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.ProgressToday
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestProgressEndpointNoSettingsDefaults(t *testing.T) {
	e := progressEnv(t)
	p := getProgress(t, e)
	if p.GoalMinutes != 15 || p.MinimumDay != 10 || p.Minutes != 0 || p.DoneToday {
		t.Fatalf("без настроек: %+v", p)
	}
}

func TestProgressEndpointUsesSettingsAndHidesOthers(t *testing.T) {
	e := progressEnv(t)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 25, Timezone: "Europe/Moscow"}
	for _, r := range minimumDay(progressNow.Add(-time.Hour)) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 1, Row: r})
	}
	for _, r := range minimumDay(progressNow.Add(-2 * time.Hour)) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 2, Row: r}) // чужое
	}
	p := getProgress(t, e)
	if p.GoalMinutes != 25 || !p.DoneToday || p.Minutes != 10 || p.Today.Answers != 11 {
		t.Fatalf("настройки и чужие события: %+v", p)
	}
}

func TestProgressEndpointUnauthorized(t *testing.T) {
	e := progressEnv(t)
	rec := e.do(http.MethodGet, progressPath, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d", rec.Code)
	}
}

// Урок «Пути» засчитывается своим временем: 7 минут урока и 3 минуты ответов дают минимум дня, серию и часы.
func TestLessonTimeCountsTowardsMinutes(t *testing.T) {
	lesson := activityRow("lesson.completed", progressNow.Add(-2*time.Hour), false)
	lesson.Seconds = 420
	yesterday := activityRow("lesson.completed", progressNow.Add(-24*time.Hour), false)
	yesterday.Seconds = 900
	events := append(minimumDay(progressNow.Add(-time.Hour))[:10], lesson, yesterday)
	p := buildProgress(events, progressNow, "", 15)
	if !p.DoneToday || p.StreakDays != 2 {
		t.Fatalf("урок должен входить в минуты дня и серию: %+v", p)
	}
	o := buildOverview([]db.ListActivityEventsRow{yesterday}, progressNow, "")
	if o.StudiedMinutes != 15 {
		t.Fatalf("часы занятий без урока: %+v", o.StudiedMinutes)
	}
	m := buildMilestones([]db.ListActivityEventsRow{yesterday}, progressNow, "")
	for _, x := range m.Milestones {
		if x.Id == "hours-1" && x.Progress.Value != 0.25 {
			t.Fatalf("веха часов без урока: %+v", x.Progress)
		}
	}
}
