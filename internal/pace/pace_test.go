package pace

import "testing"

func TestNewWordsToday(t *testing.T) {
	auto := func(minutes, due int, acc float64, has bool, today int) Input {
		return Input{DailyMinutes: minutes, Due: due, Accuracy7d: acc, HasAccuracy: has, IntroducedToday: today, Mode: "auto", Cap: 12, Tracked: 500}
	}
	tests := []struct {
		name          string
		in            Input
		limit, remain int
		code          string
	}{
		{"15 минут без данных: база 5", auto(15, 0, 0, false, 0), 5, 5, "base"},
		{"60 минут: потолок 12", auto(60, 0, 0, false, 0), 12, 12, "base"},
		{"5 минут: нижняя граница 3", auto(5, 0, 0, false, 0), 3, 3, "base"},
		{"много просроченных: ноль", auto(15, 36, 0.8, true, 0), 0, 0, "backlog"},
		{"просроченных ровно порог: ещё можно", auto(15, 35, 0.8, true, 0), 5, 5, "base"},
		{"плохая точность: база − 2", auto(15, 0, 0.6, true, 0), 3, 3, "hard"},
		{"плохая точность, малая база: не меньше 1", auto(5, 0, 0.5, true, 0), 1, 1, "hard"},
		{"хорошая точность и мало просроченных: база + 2", auto(15, 5, 0.92, true, 0), 7, 7, "easy"},
		{"хорошая точность, но просроченных больше базы: база", auto(15, 6, 0.92, true, 0), 5, 5, "base"},
		{"хорошая точность у потолка: не выше потолка", auto(60, 0, 0.95, true, 0), 12, 12, "base"},
		{"введено 3 из 5", auto(15, 0, 0.8, true, 3), 5, 2, "base"},
		{"введено всё", auto(15, 0, 0.8, true, 5), 5, 0, "done"},
		{"введено больше лимита", auto(15, 0, 0.8, true, 9), 5, 0, "done"},
		{"ручной режим", Input{Mode: "manual", PerDay: 9, Cap: 12, IntroducedToday: 4}, 9, 5, "manual"},
		{"ручной режим: число приводится к границам", Input{Mode: "manual", PerDay: 99}, 30, 30, "manual"},
		{"ручной режим: ноль приводится к одному", Input{Mode: "manual", PerDay: 0}, 1, 1, "manual"},
		{"ручной режим исчерпан", Input{Mode: "manual", PerDay: 3, IntroducedToday: 3}, 3, 0, "done"},
		{"потолок не задан: 12", Input{DailyMinutes: 120, Mode: "auto", Tracked: 500}, 12, 12, "base"},
		{"неизвестный режим считается авто", Input{DailyMinutes: 15, Mode: "turbo", Cap: 12, Tracked: 500}, 5, 5, "base"},
		{"холодный старт: слов в учёте 0 — не меньше 20", Input{DailyMinutes: 15, Mode: "auto", Cap: 12}, 20, 20, "cold"},
		{"холодный старт: 99 слов в учёте", Input{DailyMinutes: 15, Mode: "auto", Cap: 12, Tracked: 99}, 20, 20, "cold"},
		{"холодный старт кончился на 100 словах", Input{DailyMinutes: 15, Mode: "auto", Cap: 12, Tracked: 100}, 5, 5, "base"},
		{"холодный старт: введено 7 из 20", Input{DailyMinutes: 15, Mode: "auto", Cap: 12, Tracked: 10, IntroducedToday: 7}, 20, 13, "cold"},
		{"холодный старт не отменяет накопившихся повторений", Input{DailyMinutes: 15, Due: 36, Mode: "auto", Cap: 12, Tracked: 10}, 0, 0, "backlog"},
		{"холодный старт не трогает ручной режим", Input{Mode: "manual", PerDay: 5, Tracked: 10}, 5, 5, "manual"},
		{"холодный старт при большой базе не снижает лимит", Input{DailyMinutes: 120, Mode: "auto", Cap: 30, Tracked: 10}, 30, 30, "base"},
	}
	for _, tc := range tests {
		got := NewWordsToday(tc.in)
		if got.Limit != tc.limit || got.Remaining != tc.remain || got.Reason.Code != tc.code {
			t.Errorf("%s: лимит %d, осталось %d, причина %q; ждали %d, %d, %q", tc.name, got.Limit, got.Remaining, got.Reason.Code, tc.limit, tc.remain, tc.code)
		}
		if got.Reason.Text == "" {
			t.Errorf("%s: у причины нет текста", tc.name)
		}
		if got.Remaining < 0 {
			t.Errorf("%s: остаток отрицательный", tc.name)
		}
	}
}

func TestNewWordsTodayIsDeterministic(t *testing.T) {
	in := Input{DailyMinutes: 25, Due: 3, Accuracy7d: 0.88, HasAccuracy: true, IntroducedToday: 1, Mode: "auto", Cap: 12}
	first := NewWordsToday(in)
	if second := NewWordsToday(in); first != second {
		t.Fatal("результат не должен меняться между вызовами")
	}
}
