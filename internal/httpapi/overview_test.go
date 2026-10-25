package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const overviewPath = "/api/v1/progress/overview"

// dayEvents — минимальный день (ровно 10 минут) на сутки back суток назад в 10:00 UTC.
func dayEvents(back int) []db.ListActivityEventsRow {
	d := progressNow.AddDate(0, 0, -back)
	return minimumDay(time.Date(d.Year(), d.Month(), d.Day(), 10, 0, 0, 0, time.UTC))
}

func TestBuildOverviewEmpty(t *testing.T) {
	o := buildOverview(nil, progressNow, "")
	if o.StartedOn != nil || o.DaysSinceStart != 0 || o.StudiedMinutes != 0 || o.PaceReady || o.PaceMinutesPerDay != 0 {
		t.Fatalf("пустой журнал: %+v", o)
	}
	if len(o.Daily) != 90 {
		t.Fatalf("daily: %d записей, ожидали 90", len(o.Daily))
	}
	for _, p := range o.Daily {
		if p.Minutes != 0 || p.CumulativeHours != 0 {
			t.Fatalf("в пустом журнале все точки нулевые: %+v", p)
		}
	}
	if o.Daily[89].Day != "2026-10-25" || o.Daily[0].Day != "2026-07-28" {
		t.Fatalf("границы daily: %s .. %s", o.Daily[0].Day, o.Daily[89].Day)
	}
}

func TestBuildOverviewPaceReadyBoundary(t *testing.T) {
	// первое занятие today−13 → прошло 14 суток (включая сегодня) → темп готов
	o := buildOverview(dayEvents(13), progressNow, "")
	if o.DaysSinceStart != 14 || !o.PaceReady || o.StartedOn == nil || *o.StartedOn != "2026-10-12" {
		t.Fatalf("today−13: %+v", o)
	}
	o = buildOverview(dayEvents(12), progressNow, "")
	if o.DaysSinceStart != 13 || o.PaceReady {
		t.Fatalf("today−12: %+v", o)
	}
}

func TestBuildOverviewMinutesAndDaily(t *testing.T) {
	var ev []db.ListActivityEventsRow
	ev = append(ev, dayEvents(2)...)
	ev = append(ev, dayEvents(1)...)
	ev = append(ev, dayEvents(0)...)
	o := buildOverview(ev, progressNow, "")
	if o.StudiedMinutes != 30 || o.DaysSinceStart != 3 {
		t.Fatalf("минуты и сутки: %+v", o)
	}
	// темп за 14 суток: 30 минут / 14 ≈ 2,1
	if o.PaceMinutesPerDay != 2.1 {
		t.Fatalf("темп %v, ожидали 2.1", o.PaceMinutesPerDay)
	}
	last := o.Daily[89]
	if last.Minutes != 10 || last.CumulativeHours != 0.5 {
		t.Fatalf("сегодня: %+v", last)
	}
	for i := 1; i < len(o.Daily); i++ {
		if o.Daily[i].CumulativeHours < o.Daily[i-1].CumulativeHours {
			t.Fatalf("накопленные часы убывают на %d", i)
		}
	}
}

func TestBuildOverviewTimezone(t *testing.T) {
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC) // в Москве 04:00 25 октября
	ev := []db.ListActivityEventsRow{activityRow("exercise.answered", time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC), true)}
	o := buildOverview(ev, now, "Europe/Moscow")
	if o.StartedOn == nil || *o.StartedOn != "2026-10-25" || o.Daily[89].Day != "2026-10-25" || o.Daily[89].Minutes == 0 {
		t.Fatalf("в Москве событие 02:30 25 октября: %+v", o.Daily[89])
	}
	o = buildOverview(ev, now, "")
	if o.StartedOn == nil || *o.StartedOn != "2026-10-24" {
		t.Fatalf("в UTC то же событие — 24 октября: %v", o.StartedOn)
	}
}

func getOverview(t *testing.T, e env) api.ProgressOverview {
	t.Helper()
	token := login(t, e)
	rec := e.do(http.MethodGet, overviewPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.ProgressOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOverviewEndpointEmptyIsNotNull(t *testing.T) {
	e := progressEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, overviewPath, "", withCookie(token))
	if rec.Code != http.StatusOK || !jsonHasArray(rec.Body.String(), "daily") {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func jsonHasArray(body, key string) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &m) != nil {
		return false
	}
	raw := m[key]
	return len(raw) > 0 && raw[0] == '['
}

func TestOverviewEndpointSettingsAndOthers(t *testing.T) {
	e := progressEnv(t)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 25, Timezone: "Europe/Moscow"}
	for _, r := range dayEvents(1) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 1, Row: r})
	}
	for _, r := range dayEvents(0) {
		e.store.activity = append(e.store.activity, fakeActivity{UserID: 2, Row: r}) // чужое
	}
	o := getOverview(t, e)
	if o.StudiedMinutes != 10 || o.DaysSinceStart != 2 {
		t.Fatalf("только свои события: %+v", o)
	}
}

func TestOverviewEndpointUnauthorized(t *testing.T) {
	e := progressEnv(t)
	if rec := e.do(http.MethodGet, overviewPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d", rec.Code)
	}
}

// Накопленные часы округляются до тысячных: до сотых 6,3 минуты превращались в 6,6 («7 мин» вместо «6 мин»).
func TestBuildOverviewCumulativeHoursKeepsMinutes(t *testing.T) {
	start := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	o := buildOverview(burst(start, 7, time.Minute), progressNow, "") // 20 + 6×60 = 380 с = 0,10556 ч
	if got := o.Daily[89].CumulativeHours; got != 0.106 {
		t.Fatalf("накопленные часы %v, ожидали 0.106", got)
	}
}
