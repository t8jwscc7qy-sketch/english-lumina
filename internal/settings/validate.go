// Package settings проверяет настройки обучения, пришедшие от клиента.
package settings

import (
	"slices"
	"time"
)

const (
	minDailyMinutes = 5
	maxDailyMinutes = 240
	maxGoalYears    = 5
)

// levels упорядочены от низшего к высшему.
var levels = []string{"A1", "A2", "B1", "B2", "C1"}

// Input — настройки как их прислал клиент.
type Input struct {
	CurrentLevel string
	TargetLevel  string
	TargetDate   *time.Time // календарный день; время и пояс игнорируются
	DailyMinutes int
	Timezone     string // IANA, например Europe/Moscow
}

// Valid — проверенные настройки в виде, который принимает база.
type Valid struct {
	CurrentLevel string
	TargetLevel  string
	TargetDate   *time.Time // полночь UTC нужного дня
	DailyMinutes int32
	Timezone     string
}

// ValidationError — ошибка ввода; Message на русском, его можно показать пользователю.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(msg string) error { return &ValidationError{Message: msg} }

// Validate проверяет настройки. now — текущий момент; «сегодня» считается в часовом поясе пользователя.
func Validate(in Input, now time.Time) (Valid, error) {
	cur := slices.Index(levels, in.CurrentLevel)
	if cur < 0 {
		return Valid{}, invalid("Выбери текущий уровень от A1 до C1")
	}
	tgt := slices.Index(levels, in.TargetLevel)
	if tgt < 0 {
		return Valid{}, invalid("Выбери целевой уровень от A1 до C1")
	}
	if tgt < cur {
		return Valid{}, invalid("Цель не может быть ниже текущего уровня")
	}
	if in.DailyMinutes < minDailyMinutes || in.DailyMinutes > maxDailyMinutes {
		return Valid{}, invalid("Минут в день должно быть от 5 до 240")
	}
	if in.Timezone == "" {
		return Valid{}, invalid("Не указан часовой пояс")
	}
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		return Valid{}, invalid("Неизвестный часовой пояс")
	}

	out := Valid{
		CurrentLevel: in.CurrentLevel,
		TargetLevel:  in.TargetLevel,
		DailyMinutes: int32(in.DailyMinutes),
		Timezone:     in.Timezone,
	}
	if in.TargetDate != nil {
		y, m, d := now.In(loc).Date()
		today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		ty, tm, td := in.TargetDate.Date()
		date := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
		if date.Before(today) {
			return Valid{}, invalid("Дата цели уже прошла")
		}
		if date.After(today.AddDate(maxGoalYears, 0, 0)) {
			return Valid{}, invalid("Дата цели дальше чем через 5 лет")
		}
		out.TargetDate = &date
	}
	return out, nil
}
