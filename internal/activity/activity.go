// Package activity считает минуты занятий и серию по журналу событий. Все функции чистые: времена и сутки
// приходят снаружи, ничего не читается из базы.
package activity

import (
	"slices"
	"time"
)

const (
	// MinimumDayMinutes — «минимальный день»: столько минут активности нужно за сутки.
	MinimumDayMinutes = 10
	// PauseCap — пауза между ответами, которая ещё засчитывается целиком.
	PauseCap = 60 * time.Second
	// BurstStart — сколько секунд даёт первый ответ «захода»: первый ответ дня или ответ после длинной паузы.
	BurstStart = 20 * time.Second
	// HistoryDays — на сколько суток назад смотрит серия (и её предел).
	HistoryDays = 60
)

// Day — календарные сутки в часовом поясе пользователя.
type Day struct{ Y, M, D int }

// DayOf возвращает сутки момента t в поясе tz; пустой или неизвестный пояс — UTC.
func DayOf(t time.Time, tz string) Day {
	y, m, d := t.In(location(tz)).Date()
	return Day{y, int(m), d}
}

// AddDays сдвигает сутки на n (может быть отрицательным).
func (d Day) AddDays(n int) Day {
	y, m, dd := time.Date(d.Y, time.Month(d.M), d.D+n, 12, 0, 0, 0, time.UTC).Date()
	return Day{y, int(m), dd}
}

func (d Day) ordinal() int {
	return int(time.Date(d.Y, time.Month(d.M), d.D, 12, 0, 0, 0, time.UTC).Unix() / 86400)
}

func location(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Minutes считает активное время по моментам ответов одних суток (в любом порядке). Ответ добавляет паузу
// с предыдущим, если она не больше PauseCap; иначе (и для первого ответа) добавляется BurstStart.
func Minutes(times []time.Time) time.Duration {
	sorted := slices.Clone(times)
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })
	var total time.Duration
	for i, t := range sorted {
		if i > 0 {
			if gap := t.Sub(sorted[i-1]); gap <= PauseCap {
				total += gap
				continue
			}
		}
		total += BurstStart
	}
	return total
}

// DayMinutes раскладывает ответы по суткам в поясе tz и считает минуты каждых суток отдельно:
// пауза через полночь не склеивает сутки.
func DayMinutes(times []time.Time, tz string) map[Day]time.Duration {
	byDay := map[Day][]time.Time{}
	for _, t := range times {
		d := DayOf(t, tz)
		byDay[d] = append(byDay[d], t)
	}
	out := make(map[Day]time.Duration, len(byDay))
	for d, ts := range byDay {
		out[d] = Minutes(ts)
	}
	return out
}

// Streak — серия: число суток подряд с выполненным минимумом и последний прощённый пропуск.
type Streak struct {
	Days         int
	FreezeUsedOn *Day // последний пропуск, который «заморозка» простила (он лежит внутри серии)
}

// ComputeStreak считает серию назад от вчера: сегодня добавляется, если минимум уже выполнен, и не обрывает серию,
// пока он не выполнен. Пропуск прощается, если ближайший к нему по времени прощённый пропуск дальше шести суток
// (то есть в любом окне из семи суток прощается не больше одного пропуска). Смотрит на HistoryDays суток назад.
func ComputeStreak(minutes map[Day]time.Duration, today Day) Streak {
	isDone := func(d Day) bool { return minutes[d] >= MinimumDayMinutes*time.Minute }
	var s Streak
	if isDone(today) {
		s.Days = 1
	}
	var forgiven *Day // последний (самый поздний) прощённый пропуск в проходе
	var pending *Day  // пропуск, прощённый, но ещё не «мостом» к выполненным суткам
	for i := 1; i <= HistoryDays && s.Days < HistoryDays; i++ {
		d := today.AddDays(-i)
		if isDone(d) {
			s.Days++
			if pending != nil {
				if s.FreezeUsedOn == nil {
					s.FreezeUsedOn = pending
				}
				pending = nil
			}
			continue
		}
		if forgiven != nil && forgiven.ordinal()-d.ordinal() < 7 {
			break
		}
		dd := d
		forgiven, pending = &dd, &dd
	}
	return s
}
