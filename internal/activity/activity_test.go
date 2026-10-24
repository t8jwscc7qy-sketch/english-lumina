package activity

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func TestMinutesEmpty(t *testing.T) {
	if got := Minutes(nil); got != 0 {
		t.Fatalf("пустой список: %v", got)
	}
}

func TestMinutesSingleAnswer(t *testing.T) {
	if got := Minutes([]time.Time{t0}); got != BurstStart {
		t.Fatalf("один ответ: %v, ожидали %v", got, BurstStart)
	}
}

func TestMinutesPauseCap(t *testing.T) {
	// 20 с на первый ответ + 30 с + 60 с (ровно 60 засчитывается).
	times := []time.Time{t0, t0.Add(30 * time.Second), t0.Add(90 * time.Second)}
	if got := Minutes(times); got != 110*time.Second {
		t.Fatalf("паузы 30 и 60 с: %v, ожидали 110s", got)
	}
	// Пауза 61 с — новый «заход»: 20 с вместо 61.
	times = []time.Time{t0, t0.Add(61 * time.Second)}
	if got := Minutes(times); got != 40*time.Second {
		t.Fatalf("пауза 61 с: %v, ожидали 40s", got)
	}
}

func TestMinutesUnordered(t *testing.T) {
	a := []time.Time{t0.Add(90 * time.Second), t0, t0.Add(30 * time.Second)}
	if got := Minutes(a); got != 110*time.Second {
		t.Fatalf("порядок на вход не важен: %v", got)
	}
}

func TestDayOfTimezone(t *testing.T) {
	ts := time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC)
	if got := DayOf(ts, Location("Europe/Moscow")); got != (Day{2026, 10, 25}) {
		t.Fatalf("Москва: %+v", got)
	}
	if got := DayOf(ts, Location("Нет/Такого")); got != (Day{2026, 10, 24}) {
		t.Fatalf("неизвестный пояс должен давать UTC: %+v", got)
	}
	if got := DayOf(ts, Location("")); got != (Day{2026, 10, 24}) {
		t.Fatalf("пустой пояс должен давать UTC: %+v", got)
	}
}

func TestDayAddDays(t *testing.T) {
	if got := (Day{2026, 3, 1}).AddDays(-1); got != (Day{2026, 2, 28}) {
		t.Fatalf("через границу месяца: %+v", got)
	}
	if got := (Day{2026, 12, 31}).AddDays(1); got != (Day{2027, 1, 1}) {
		t.Fatalf("через границу года: %+v", got)
	}
}

func TestDayMinutesMidnightNotMerged(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	a := time.Date(2026, 10, 24, 23, 59, 50, 0, loc)
	b := time.Date(2026, 10, 25, 0, 0, 10, 0, loc) // пауза 20 с, но сутки разные
	got := DayMinutes([]time.Time{a, b}, Location("Europe/Moscow"))
	if len(got) != 2 || got[Day{2026, 10, 24}] != BurstStart || got[Day{2026, 10, 25}] != BurstStart {
		t.Fatalf("сутки не должны склеиваться: %+v", got)
	}
}

func done(n ...int) func(today Day) map[Day]time.Duration {
	return func(today Day) map[Day]time.Duration {
		m := map[Day]time.Duration{}
		for _, k := range n {
			m[today.AddDays(-k)] = MinimumDayMinutes * time.Minute
		}
		return m
	}
}

var today = Day{2026, 10, 25}

func TestStreakNoData(t *testing.T) {
	if got := ComputeStreak(nil, today); got.Days != 0 || got.FreezeUsedOn != nil {
		t.Fatalf("нет данных: %+v", got)
	}
}

func TestStreakThreeDays(t *testing.T) {
	// выполнены вчера, позавчера и три дня назад; сегодня ничего нет, но серия не обнуляется
	if got := ComputeStreak(done(1, 2, 3)(today), today); got.Days != 3 || got.FreezeUsedOn != nil {
		t.Fatalf("три дня подряд: %+v", got)
	}
}

func TestStreakTodayAdded(t *testing.T) {
	if got := ComputeStreak(done(0, 1)(today), today); got.Days != 2 {
		t.Fatalf("сегодня + вчера: %+v", got)
	}
}

func TestStreakBelowMinimumNotCounted(t *testing.T) {
	m := done(2)(today)
	m[today.AddDays(-1)] = MinimumDayMinutes*time.Minute - time.Second
	got := ComputeStreak(m, today)
	if got.Days != 1 || got.FreezeUsedOn == nil || *got.FreezeUsedOn != today.AddDays(-1) {
		t.Fatalf("9:59 — не минимум, но один пропуск прощается: %+v", got)
	}
}

func TestStreakFreezeOneMiss(t *testing.T) {
	got := ComputeStreak(done(2, 3)(today), today)
	if got.Days != 2 || got.FreezeUsedOn == nil || *got.FreezeUsedOn != today.AddDays(-1) {
		t.Fatalf("пропуск вчера прощается: %+v", got)
	}
}

func TestStreakTwoMissesBreak(t *testing.T) {
	if got := ComputeStreak(done(3, 4)(today), today); got.Days != 0 || got.FreezeUsedOn != nil {
		t.Fatalf("пропуски вчера и позавчера обрывают: %+v", got)
	}
}

func TestStreakSecondFreezeWithinWeekBreaks(t *testing.T) {
	// выполнены: 1, 3, 4; пропуски 2 и 5 внутри окна 7 суток: второй прощён быть не может
	got := ComputeStreak(done(1, 3, 4)(today), today)
	if got.Days != 3 || got.FreezeUsedOn == nil || *got.FreezeUsedOn != today.AddDays(-2) {
		t.Fatalf("вторая заморозка в окне недели не положена: %+v", got)
	}
}

func TestStreakFreezeRefillsAfterWeek(t *testing.T) {
	// пропуски 1 и 9 (расстояние 8 суток): оба прощаются; выполнены 2..8 и 10
	got := ComputeStreak(done(2, 3, 4, 5, 6, 7, 8, 10)(today), today)
	if got.Days != 8 || got.FreezeUsedOn == nil || *got.FreezeUsedOn != today.AddDays(-1) {
		t.Fatalf("через 8 суток заморозка снова доступна: %+v", got)
	}
}

func TestStreakCap60(t *testing.T) {
	m := map[Day]time.Duration{}
	for i := 1; i <= 100; i++ {
		m[today.AddDays(-i)] = MinimumDayMinutes * time.Minute
	}
	if got := ComputeStreak(m, today); got.Days != HistoryDays {
		t.Fatalf("серия ограничена %d сутками: %+v", HistoryDays, got)
	}
}

// Часовой пояс разбирается один раз на запрос, а не на каждое событие: 18 000 событий (300 ответов в день
// за 60 суток) должны считаться за доли секунды.
func TestDayMinutesManyEventsIsFast(t *testing.T) {
	var times []time.Time
	for d := 0; d < 60; d++ {
		for i := 0; i < 300; i++ {
			times = append(times, t0.AddDate(0, 0, -d).Add(time.Duration(i)*20*time.Second))
		}
	}
	start := time.Now()
	got := DayMinutes(times, Location("Europe/Moscow"))
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("%d событий считались %v, ожидали быстрее 150 мс", len(times), elapsed)
	}
	if len(got) < 60 {
		t.Fatalf("суток %d, ожидали не меньше 60", len(got))
	}
}
