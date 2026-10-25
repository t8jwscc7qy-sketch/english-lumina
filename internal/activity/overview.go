package activity

import "time"

// DayPoint — точка графика накопленных часов: минуты за сутки и сумма часов с самого начала занятий.
type DayPoint struct {
	Day             Day
	Minutes         time.Duration
	CumulativeHours float64
}

// Pace — средний темп на сутки за последние days суток (от today−days+1 до today), пустые сутки тоже считаются.
// days <= 0 даёт 0.
func Pace(perDay map[Day]time.Duration, today Day, days int) time.Duration {
	if days <= 0 {
		return 0
	}
	var total time.Duration
	for i := 0; i < days; i++ {
		total += perDay[today.AddDays(-i)]
	}
	return total / time.Duration(days)
}

// CumulativeSeries — ровно days точек подряд от today−days+1 до today. Накопленные часы считаются с начала
// всей perDay, а не окна: занятия до окна входят в первое значение, ряд не убывает.
func CumulativeSeries(perDay map[Day]time.Duration, today Day, days int) []DayPoint {
	if days <= 0 {
		return nil
	}
	first := today.AddDays(-days + 1)
	var before time.Duration
	for d, m := range perDay {
		if d.ordinal() < first.ordinal() {
			before += m
		}
	}
	out := make([]DayPoint, 0, days)
	cum := before
	for i := 0; i < days; i++ {
		d := first.AddDays(i)
		m := perDay[d]
		cum += m
		out = append(out, DayPoint{Day: d, Minutes: m, CumulativeHours: cum.Hours()})
	}
	return out
}

// StartedOn — самые ранние сутки с занятиями (нулевые сутки не считаются).
func StartedOn(perDay map[Day]time.Duration) (Day, bool) {
	var best Day
	found := false
	for d, m := range perDay {
		if m <= 0 {
			continue
		}
		if !found || d.ordinal() < best.ordinal() {
			best, found = d, true
		}
	}
	return best, found
}

// DaysSince — сколько суток прошло от start до today включительно: сегодня — 1, вчера — 2.
func DaysSince(start, today Day) int {
	return today.ordinal() - start.ordinal() + 1
}
