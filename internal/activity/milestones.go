package activity

import (
	"fmt"
	"sort"
	"time"
)

// Пороги вех (зашиты в код: смена порогов меняет прошлые вехи задним числом).
var (
	streakThresholds  = []int{3, 7, 14, 30, 100, 365}
	hoursThresholds   = []int{1, 5, 10, 25, 50, 100, 250}
	answersThresholds = []int{100, 500, 1000}
)

// Milestone — веха и её состояние.
type Milestone struct {
	ID, Kind, Title string
	Threshold       int
	ReachedOn       *Day    // первые сутки, когда порог достигнут; nil — не достигнута
	Value           float64 // текущее значение линейки в единицах порога (дни, часы, ответы; для «первое» 0 или 1)
}

// MilestoneInput — факты из журнала событий (в окне истории).
type MilestoneInput struct {
	Minutes                                      map[Day]time.Duration // минуты занятий по суткам
	Answers                                      map[Day]int           // ответы на упражнения и оценки карточек по суткам
	FirstCard, FirstExercise, FirstMistakeClosed *Day
}

func pluralRu(n int, one, few, many string) string {
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

// ComputeMilestones считает все вехи и те, что достигнуты именно сегодня (newToday: серия, затем часы, затем ответы,
// затем первое; внутри линейки больший порог первым). Список вех всегда полный и в том же порядке.
func ComputeMilestones(in MilestoneInput, today Day) (all []Milestone, newToday []string) {
	from := today
	extend := func(d Day) {
		if d.ordinal() < from.ordinal() {
			from = d
		}
	}
	for d := range in.Minutes {
		extend(d)
	}
	for d := range in.Answers {
		extend(d)
	}

	// накопление по суткам
	streak := StreakSeries(in.Minutes, from, today)
	var cumMinutes time.Duration
	cumAnswers := 0
	reachedStreak := map[int]Day{}
	reachedHours := map[int]Day{}
	reachedAnswers := map[int]Day{}
	for d := from; d.ordinal() <= today.ordinal(); d = d.AddDays(1) {
		cumMinutes += in.Minutes[d]
		cumAnswers += in.Answers[d]
		for _, t := range streakThresholds {
			if _, ok := reachedStreak[t]; !ok && streak[d] >= t {
				reachedStreak[t] = d
			}
		}
		for _, t := range hoursThresholds {
			if _, ok := reachedHours[t]; !ok && cumMinutes >= time.Duration(t)*time.Hour {
				reachedHours[t] = d
			}
		}
		for _, t := range answersThresholds {
			if _, ok := reachedAnswers[t]; !ok && cumAnswers >= t {
				reachedAnswers[t] = d
			}
		}
	}
	hoursNow := cumMinutes.Hours()
	streakNow := float64(streak[today])

	add := func(m Milestone, reached map[int]Day) {
		if d, ok := reached[m.Threshold]; ok {
			dd := d
			m.ReachedOn = &dd
		}
		all = append(all, m)
	}
	for _, t := range streakThresholds {
		add(Milestone{ID: fmt.Sprintf("streak-%d", t), Kind: "streak", Threshold: t, Value: streakNow,
			Title: fmt.Sprintf("Серия %d %s", t, pluralRu(t, "день", "дня", "дней"))}, reachedStreak)
	}
	for _, t := range hoursThresholds {
		add(Milestone{ID: fmt.Sprintf("hours-%d", t), Kind: "hours", Threshold: t, Value: hoursNow,
			Title: fmt.Sprintf("%d %s занятий", t, pluralRu(t, "час", "часа", "часов"))}, reachedHours)
	}
	for _, t := range answersThresholds {
		add(Milestone{ID: fmt.Sprintf("answers-%d", t), Kind: "answers", Threshold: t, Value: float64(cumAnswers),
			Title: fmt.Sprintf("%d %s", t, pluralRu(t, "ответ", "ответа", "ответов"))}, reachedAnswers)
	}
	first := func(id, title string, d *Day) {
		m := Milestone{ID: id, Kind: "first", Title: title, Threshold: 1}
		if d != nil {
			dd := *d
			m.ReachedOn, m.Value = &dd, 1
		}
		all = append(all, m)
	}
	first("first-card", "Первая карточка", in.FirstCard)
	first("first-exercise", "Первое упражнение", in.FirstExercise)
	first("first-mistake-closed", "Первая закрытая ошибка", in.FirstMistakeClosed)

	newToday = []string{}
	var fresh []Milestone
	for _, m := range all {
		if m.ReachedOn != nil && *m.ReachedOn == today {
			fresh = append(fresh, m)
		}
	}
	rank := map[string]int{"streak": 0, "hours": 1, "answers": 2, "first": 3}
	sort.SliceStable(fresh, func(i, j int) bool {
		if rank[fresh[i].Kind] != rank[fresh[j].Kind] {
			return rank[fresh[i].Kind] < rank[fresh[j].Kind]
		}
		return fresh[i].Threshold > fresh[j].Threshold
	})
	for _, m := range fresh {
		newToday = append(newToday, m.ID)
	}
	return all, newToday
}
