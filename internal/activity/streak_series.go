package activity

import "time"

// seriesDepth — глубина просмотра серии для вех (окно истории).
const seriesDepth = 400

// StreakSeries — длина серии на конец каждых суток от from до to. Для каждых суток считается обратным
// подсчётом ComputeStreakN по событиям до этих суток (то, что видел бы пользователь в тот день): так серия вех
// совпадает с серией «Сегодня» по построению. Сутки без минимума, если это не прошедший день, не обнуляют серию.
func StreakSeries(minutes map[Day]time.Duration, from, to Day) map[Day]int {
	out := make(map[Day]int)
	for d := from; d.ordinal() <= to.ordinal(); d = d.AddDays(1) {
		out[d] = ComputeStreakN(minutes, d, seriesDepth).Days
	}
	return out
}
