package httpapi

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/activity"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/mistakes"
)

const (
	overviewWindowDays = 365 // за сколько суток читаем события
	overviewDailyDays  = 90  // сколько суток отдаём для графика
	paceDays           = 14  // окно темпа и порог готовности темпа
)

// GetProgressOverview отдаёт факты для страницы «Прогресс»; прогноз срока считает клиент. Ничего не записывает.
func (s *authServer) GetProgressOverview(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	tz := ""
	st, err := s.store.GetSettings(ctx, user.ID)
	switch {
	case err == nil:
		tz = st.Timezone
	case !errors.Is(err, pgx.ErrNoRows):
		s.failExercises(w, fmt.Errorf("настройки для обзора прогресса: %w", err))
		return
	}
	now := s.now()
	since := mistakes.DayStart(now.AddDate(0, 0, -overviewWindowDays), tz)
	events, err := s.store.ListActivityEvents(ctx, db.ListActivityEventsParams{UserID: user.ID, Since: ts(since)})
	if err != nil {
		s.failExercises(w, fmt.Errorf("события занятий для обзора: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, buildOverview(events, now, tz))
}

func round(v float64, places int) float32 {
	k := math.Pow(10, float64(places))
	return float32(math.Round(v*k) / k)
}

// buildOverview собирает ответ из событий; чистая функция (время и пояс приходят снаружи).
func buildOverview(events []db.ListActivityEventsRow, now time.Time, tz string) api.ProgressOverview {
	loc := activity.Location(tz)
	perDay := studyMinutes(events, loc)
	today := activity.DayOf(now, loc)

	var total time.Duration
	for _, m := range perDay {
		total += m
	}
	out := api.ProgressOverview{
		StudiedMinutes:    round(total.Minutes(), 1),
		PaceMinutesPerDay: round(activity.Pace(perDay, today, paceDays).Minutes(), 1),
		Daily:             make([]api.DailyPoint, 0, overviewDailyDays),
	}
	if start, ok := activity.StartedOn(perDay); ok {
		out.DaysSinceStart = activity.DaysSince(start, today)
		day := dayString(start)
		out.StartedOn = &day
	}
	out.PaceReady = out.DaysSinceStart >= paceDays
	for _, p := range activity.CumulativeSeries(perDay, today, overviewDailyDays) {
		out.Daily = append(out.Daily, api.DailyPoint{Day: dayString(p.Day), Minutes: round(p.Minutes.Minutes(), 1), CumulativeHours: round(p.CumulativeHours, 3)})
	}
	return out
}

func dayString(d activity.Day) string { return fmt.Sprintf("%04d-%02d-%02d", d.Y, d.M, d.D) }
