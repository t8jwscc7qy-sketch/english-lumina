// Package pace считает, сколько новых слов дать пользователю сегодня: в авто-режиме по его минутам в день,
// накопившимся повторениям и точности последних ответов, в ручном — столько, сколько он выбрал.
// Пороги и формулы — заявленная эвристика, не откалиброванная по данным.
package pace

import "math"

const (
	minBase         = 3
	maxPerDay       = 30
	defaultCap      = 12
	backlogFactor   = 7    // просрочено больше backlogFactor × база — новых слов нет
	hardAccuracy    = 0.70 // точность ниже — лимит меньше
	easyAccuracy    = 0.90 // точность не ниже (и мало просроченных) — лимит больше
	defaultAccuracy = 0.8  // точность, если данных за неделю нет
	adjust          = 2
	coldStartWords  = 100 // пока слов в учёте меньше, действует «холодный старт»
	coldStartFloor  = 20  // и новых слов в день не меньше (иначе до первых текстов недели)
)

// Input — исходные данные. Mode — «auto» или «manual» (всё остальное считается авто); Cap — потолок авто-режима.
type Input struct {
	DailyMinutes    int
	Due             int // просроченных карточек сейчас
	Accuracy7d      float64
	HasAccuracy     bool
	IntroducedToday int
	Mode            string
	PerDay          int
	Cap             int
	// Tracked — слов уже в учёте (введено, изучено, отмечено «знаю»): определяет «холодный старт».
	Tracked int
}

// Reason — причина лимита: код для клиента и текст по-русски.
type Reason struct {
	Code string
	Text string
}

// Result — лимит на сутки и сколько слов ещё можно ввести.
type Result struct {
	Limit     int
	Remaining int
	Reason    Reason
}

var (
	reasonManual  = Reason{"manual", "Ты выбрал число вручную"}
	reasonBacklog = Reason{"backlog", "Сначала повторим накопившееся"}
	reasonHard    = Reason{"hard", "Последние повторения шли трудно"}
	reasonEasy    = Reason{"easy", "Повторения идут легко"}
	reasonBase    = Reason{"base", "Столько слов поместится в твои минуты"}
	reasonDone    = Reason{"done", "На сегодня всё"}
	reasonCold    = Reason{"cold", "Начало пути: сначала накопим слов для первых текстов"}
)

func clamp(v, lo, hi int) int { return min(hi, max(lo, v)) }

// NewWordsToday возвращает лимит новых слов на сутки. Введённые сегодня слова вычитаются из остатка; исчерпанный
// положительный лимит даёт причину «На сегодня всё».
func NewWordsToday(in Input) Result {
	var limit int
	var reason Reason
	if in.Mode == "manual" {
		limit, reason = clamp(in.PerDay, 1, maxPerDay), reasonManual
	} else {
		cap := in.Cap
		if cap <= 0 {
			cap = defaultCap
		}
		cap = clamp(cap, 1, maxPerDay)
		base := clamp(int(math.Round(float64(in.DailyMinutes)/3)), minBase, cap)
		acc := defaultAccuracy
		if in.HasAccuracy {
			acc = in.Accuracy7d
		}
		switch {
		case in.Due > backlogFactor*base:
			limit, reason = 0, reasonBacklog
		case acc < hardAccuracy:
			limit, reason = max(1, base-adjust), reasonHard
		case acc >= easyAccuracy && in.Due <= base:
			limit, reason = min(cap, base+adjust), reasonEasy
		default:
			limit, reason = base, reasonBase
		}
		// Холодный старт: пока слов мало, лимит не ниже coldStartFloor (кроме случая накопившихся повторений).
		if in.Tracked < coldStartWords && limit > 0 && limit < coldStartFloor {
			limit, reason = coldStartFloor, reasonCold
		}
		// «Повторения идут легко» без прибавки (потолок) — это обычный лимит.
		if reason == reasonEasy && limit == base {
			reason = reasonBase
		}
	}
	remaining := max(0, limit-in.IntroducedToday)
	if limit > 0 && remaining == 0 {
		reason = reasonDone
	}
	return Result{Limit: limit, Remaining: remaining, Reason: reason}
}
