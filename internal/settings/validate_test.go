package settings

import (
	"errors"
	"testing"
	"time"
)

// 2026-10-23 12:00 UTC: в Pacific/Kiritimati (UTC+14) уже 24 октября.
var now = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func good() Input {
	return Input{CurrentLevel: "B1", TargetLevel: "B2", DailyMinutes: 15, Timezone: "Europe/Moscow"}
}

func TestValidate_accepts(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Input)
	}{
		{"базовый ввод", func(*Input) {}},
		{"цель равна уровню", func(in *Input) { in.TargetLevel = "B1" }},
		{"минимум минут", func(in *Input) { in.DailyMinutes = 5 }},
		{"максимум минут", func(in *Input) { in.DailyMinutes = 240 }},
		{"даты нет", func(in *Input) { in.TargetDate = nil }},
		{"дата сегодня в UTC", func(in *Input) { in.Timezone = "UTC"; in.TargetDate = day(2026, 10, 23) }},
		{"дата ровно через 5 лет", func(in *Input) { in.Timezone = "UTC"; in.TargetDate = day(2031, 10, 23) }},
		{"сегодня в поясе пользователя уже 24-е", func(in *Input) {
			in.Timezone = "Pacific/Kiritimati"
			in.TargetDate = day(2026, 10, 24)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := good()
			tc.mut(&in)
			if _, err := Validate(in, now); err != nil {
				t.Fatalf("ожидали успех, получили %v", err)
			}
		})
	}
}

func TestValidate_rejects(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Input)
	}{
		{"уровень в нижнем регистре", func(in *Input) { in.CurrentLevel = "a1" }},
		{"неизвестный уровень", func(in *Input) { in.CurrentLevel = "D1" }},
		{"пустой уровень", func(in *Input) { in.CurrentLevel = "" }},
		{"неизвестная цель", func(in *Input) { in.TargetLevel = "C2" }},
		{"цель ниже уровня", func(in *Input) { in.CurrentLevel = "B2"; in.TargetLevel = "B1" }},
		{"4 минуты", func(in *Input) { in.DailyMinutes = 4 }},
		{"241 минута", func(in *Input) { in.DailyMinutes = 241 }},
		{"неизвестный пояс", func(in *Input) { in.Timezone = "Mars/Base" }},
		{"пустой пояс", func(in *Input) { in.Timezone = "" }},
		{"вчера в UTC", func(in *Input) { in.Timezone = "UTC"; in.TargetDate = day(2026, 10, 22) }},
		{"23-е в Kiritimati уже вчера", func(in *Input) {
			in.Timezone = "Pacific/Kiritimati"
			in.TargetDate = day(2026, 10, 23)
		}},
		{"через 5 лет и день", func(in *Input) { in.Timezone = "UTC"; in.TargetDate = day(2031, 10, 24) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := good()
			tc.mut(&in)
			_, err := Validate(in, now)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Message == "" {
				t.Fatalf("ожидали *ValidationError с текстом, получили %v", err)
			}
		})
	}
}

func TestValidate_normalizesDate(t *testing.T) {
	in := good()
	in.Timezone = "UTC"
	in.TargetDate = day(2027, 3, 1)
	got, err := Validate(in, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetDate == nil || !got.TargetDate.Equal(*day(2027, 3, 1)) {
		t.Fatalf("дата изменилась: %v", got.TargetDate)
	}
	if got.DailyMinutes != 15 {
		t.Fatalf("минуты: %d", got.DailyMinutes)
	}
}
