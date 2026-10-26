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

// defaultGoalMinutes — цель дня, пока настройки не заданы.
const defaultGoalMinutes = 15

// GetProgressToday отдаёт минуты, серию и счётчики дня из журнала событий. Ничего не записывает.
func (s *authServer) GetProgressToday(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	tz, goal := "", defaultGoalMinutes
	st, err := s.store.GetSettings(ctx, user.ID)
	switch {
	case err == nil:
		tz, goal = st.Timezone, int(st.DailyMinMinutes)
	case !errors.Is(err, pgx.ErrNoRows):
		s.failExercises(w, fmt.Errorf("настройки для прогресса: %w", err))
		return
	}
	now := s.now()
	since := mistakes.DayStart(now.AddDate(0, 0, -activity.HistoryDays), tz)
	events, err := s.store.ListActivityEvents(ctx, db.ListActivityEventsParams{
		UserID: user.ID, Since: ts(since),
	})
	if err != nil {
		s.failExercises(w, fmt.Errorf("события занятий: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, buildProgress(events, now, tz, goal))
}

// buildProgress собирает ответ из событий; чистая функция (время и пояс приходят снаружи).
// studyMinutes — минуты занятий по суткам: ответы и оценки карточек считаются по паузам между ними, урок «Пути» —
// своим временем (payload.seconds), прибавленным к суткам, когда урок завершён.
func studyMinutes(events []db.ListActivityEventsRow, loc *time.Location) map[activity.Day]time.Duration {
	var times []time.Time
	for _, e := range events {
		if e.Type == "exercise.answered" || e.Type == "card.reviewed" {
			times = append(times, e.OccurredAt.Time)
		}
	}
	perDay := activity.DayMinutes(times, loc)
	for _, e := range events {
		if e.Type == "lesson.completed" && e.Seconds > 0 {
			perDay[activity.DayOf(e.OccurredAt.Time, loc)] += time.Duration(e.Seconds) * time.Second
		}
	}
	return perDay
}

func buildProgress(events []db.ListActivityEventsRow, now time.Time, tz string, goal int) api.ProgressToday {
	loc := activity.Location(tz) // пояс разбираем один раз: на тысячах событий LoadLocation на каждое слишком дорог
	perDay := studyMinutes(events, loc)
	today := activity.DayOf(now, loc)
	todayDur := perDay[today]

	var counts api.TodayCounts
	for _, e := range events {
		if activity.DayOf(e.OccurredAt.Time, loc) != today {
			continue
		}
		switch e.Type {
		case "exercise.answered":
			counts.Answers++
			if e.Correct { // correct осмысленно только у ответов на упражнения
				counts.Correct++
			}
		case "card.reviewed":
			counts.CardsReviewed++
		case "mistake.opened":
			counts.MistakesOpened++
		case "mistake.closed":
			counts.MistakesClosed++
		}
	}

	streak := activity.ComputeStreak(perDay, today)
	out := api.ProgressToday{
		Minutes:     float32(math.Round(todayDur.Minutes()*10) / 10),
		MinimumDay:  activity.MinimumDayMinutes,
		GoalMinutes: goal,
		DoneToday:   todayDur >= activity.MinimumDayMinutes*time.Minute,
		StreakDays:  streak.Days,
		Today:       counts,
	}
	if streak.FreezeUsedOn != nil {
		f := fmt.Sprintf("%04d-%02d-%02d", streak.FreezeUsedOn.Y, streak.FreezeUsedOn.M, streak.FreezeUsedOn.D)
		out.FreezeUsedOn = &f
	}
	return out
}
