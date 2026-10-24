// Package mistakes — расписание повторов ошибок (банк ошибок): чистые функции без обращений к базе.
package mistakes

import "time"

const (
	// MaxRepeatsPerDay — сколько повторов ошибок показываем за местные сутки.
	MaxRepeatsPerDay = 5
	// CloseAtStreak — сколько верных повторов подряд закрывают ошибку.
	CloseAtStreak = 2
	// FirstDelay — через сколько первый повтор (и после неверного ответа).
	FirstDelay = 24 * time.Hour
	// SecondDelay — через сколько повтор после первого верного.
	SecondDelay = 72 * time.Hour
)

// State — состояние ошибки в банке.
type State struct {
	Streak  int // верных повторов подряд
	Closed  bool
	NextDue time.Time
}

// Opened — состояние только что записанной ошибки: первый повтор через сутки.
func Opened(now time.Time) State {
	return State{NextDue: now.Add(FirstDelay)}
}

// Advance применяет ответ на повтор. Неверный ответ обнуляет серию и возвращает повтор через сутки,
// верный увеличивает серию; на CloseAtStreak ошибка закрывается.
func Advance(s State, correct bool, now time.Time) State {
	if !correct {
		return State{NextDue: now.Add(FirstDelay)}
	}
	s.Streak++
	if s.Streak >= CloseAtStreak {
		s.Closed = true
		return s
	}
	s.NextDue = now.Add(SecondDelay)
	return s
}

// ShouldServeRepeat решает, показывать ли повтор: есть наступивший, за сегодня меньше лимита
// и ни одно из двух последних отвеченных упражнений не было повтором.
func ShouldServeRepeat(repeatsToday int, anyOfLastTwoWasRepeat bool, hasDue bool) bool {
	return hasDue && repeatsToday < MaxRepeatsPerDay && !anyOfLastTwoWasRepeat
}

// DayStart возвращает начало местных суток для now в поясе tz, в UTC. Пустой или неизвестный пояс — UTC.
func DayStart(now time.Time, tz string) time.Time {
	loc := time.UTC
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
}
