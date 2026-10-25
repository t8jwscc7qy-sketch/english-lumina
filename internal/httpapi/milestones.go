package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/activity"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/mistakes"
)

// GetMilestones отдаёт вехи занятий и те, что достигнуты сегодня. Ничего не записывает.
func (s *authServer) GetMilestones(w http.ResponseWriter, r *http.Request) {
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
		s.failExercises(w, fmt.Errorf("настройки для вех: %w", err))
		return
	}
	now := s.now()
	since := mistakes.DayStart(now.AddDate(0, 0, -overviewWindowDays), tz)
	events, err := s.store.ListActivityEvents(ctx, db.ListActivityEventsParams{UserID: user.ID, Since: ts(since)})
	if err != nil {
		s.failExercises(w, fmt.Errorf("события занятий для вех: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, buildMilestones(events, now, tz))
}

// buildMilestones собирает ответ из событий; чистая функция (время и пояс приходят снаружи).
func buildMilestones(events []db.ListActivityEventsRow, now time.Time, tz string) api.MilestoneList {
	loc := activity.Location(tz)
	today := activity.DayOf(now, loc)
	var times []time.Time
	in := activity.MilestoneInput{Answers: map[activity.Day]int{}}
	earliest := func(cur **activity.Day, d activity.Day) {
		if *cur == nil || d.Before(**cur) {
			dd := d
			*cur = &dd
		}
	}
	var first *activity.Day // самое раннее загруженное событие любого типа
	for _, e := range events {
		d := activity.DayOf(e.OccurredAt.Time, loc)
		earliest(&first, d)
		switch e.Type {
		case "exercise.answered":
			times = append(times, e.OccurredAt.Time)
			in.Answers[d]++
			earliest(&in.FirstExercise, d)
		case "card.reviewed":
			times = append(times, e.OccurredAt.Time)
			in.Answers[d]++
			earliest(&in.FirstCard, d)
		case "mistake.closed":
			earliest(&in.FirstMistakeClosed, d)
		}
	}
	in.Minutes = activity.DayMinutes(times, loc)

	all, newToday := activity.ComputeMilestones(in, today)
	// История, вероятно, обрезана окном: вехи «до окна» достигаются заново каждый день, вспышку не даём.
	if first != nil && !today.AddDays(-overviewWindowDays+2).Before(*first) {
		newToday = []string{}
	}
	out := api.MilestoneList{Milestones: make([]api.Milestone, 0, len(all)), NewToday: newToday}
	for _, m := range all {
		item := api.Milestone{
			Id: m.ID, Kind: m.Kind, Title: m.Title, Threshold: m.Threshold,
			Progress: api.MilestoneProgress{Value: round(minFloat(m.Value, float64(m.Threshold)), 2), Target: float32(m.Threshold)},
		}
		if m.ReachedOn != nil {
			day := dayString(*m.ReachedOn)
			item.ReachedOn = &day
		}
		out.Milestones = append(out.Milestones, item)
	}
	return out
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
