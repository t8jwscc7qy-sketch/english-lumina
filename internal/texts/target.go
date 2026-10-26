// Package texts собирает тексты для чтения и слушания на заданном уровне понимания и подстраивает цель по ответам
// ученика. Чистые функции без базы: входы приходят из обработчика.
package texts

import "github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"

// Границы и шаг цели понимания.
const (
	GoalMin  = 0.80
	GoalMax  = 0.90
	goalStep = 0.02
)

// Ответы «как понял».
const (
	UnderstoodEasy = "easy" // почти всё
	UnderstoodOK   = "ok"   // нормально
	UnderstoodHard = "hard" // трудно
)

func clampGoal(g float64) float64 { return min(GoalMax, max(GoalMin, g)) }

// NextGoal двигает цель понимания: «почти всё» — текст можно сложнее (цель ниже на 2 п. п.), «трудно» — проще
// (выше на 2 п. п.), «нормально» и незнакомый ответ ничего не меняют. Цель остаётся в границах 0,80–0,90.
func NextGoal(current float64, understood string) float64 {
	switch understood {
	case UnderstoodEasy:
		current -= goalStep
	case UnderstoodHard:
		current += goalStep
	}
	g := clampGoal(current)
	// Убираем накопление двоичной погрешности: цель хранится с точностью до сотых.
	return float64(int(g*100+0.5)) / 100
}

// Understanding — доля известных слов среди учитываемых во всех предложениях (токены каждого предложения в
// исходном регистре). Нет учитываемых слов — полное понимание.
func Understanding(tokens [][]string, known map[string]bool) float64 {
	total, unknown := 0, 0
	for _, t := range tokens {
		tt, uu := exercises.CountKnown(t, known)
		total += tt
		unknown += uu
	}
	if total == 0 {
		return 1
	}
	return float64(total-unknown) / float64(total)
}
