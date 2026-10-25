// Package levelsuggest решает, предложить ли пользователю сменить уровень. Только предлагает: уровень меняет сам пользователь.
package levelsuggest

import "time"

const (
	// Window — сколько последних ответов на упражнения своего уровня смотрим; меньше — предложения нет.
	Window = 30
	// UpCorrect — верных из Window и больше: предложить уровень выше.
	UpCorrect = 27 // 90 %
	// DownCorrect — верных из Window и меньше: предложить уровень ниже.
	DownCorrect = 12 // 40 %
	// HistoryDays — ответы старше не учитываются (их отбирает запрос, пакет знает число для описания в API).
	HistoryDays = 60
	// DismissFor — после «Пока нет» предложение прячется на столько.
	DismissFor = 14 * 24 * time.Hour
)

// Направление предложения.
const (
	Up   = "up"
	Down = "down"
)

// Levels — шкала уровней сервиса по возрастанию.
var Levels = []string{"A1", "A2", "B1", "B2", "C1"}

// Suggestion — предложение: откуда, куда и по какой выборке.
type Suggestion struct {
	Direction string
	From, To  string
	Correct   int
	Total     int
}

// Shift возвращает уровень на delta ступеней от level; false, если level не на шкале или выходит за её края.
func Shift(level string, delta int) (string, bool) {
	for i, l := range Levels {
		if l == level {
			j := i + delta
			if j < 0 || j >= len(Levels) {
				return "", false
			}
			return Levels[j], true
		}
	}
	return "", false
}

// Suggest смотрит на последние Window ответов (порядок от старых к новым; лишние старые отбрасываются) на упражнения
// уровня level. dismissedAt — когда пользователь в последний раз ответил «Пока нет» (nil — не отвечал): в течение
// DismissFor любое предложение скрыто. Нет предложения — nil.
func Suggest(level string, answers []bool, dismissedAt *time.Time, now time.Time) *Suggestion {
	if len(answers) < Window {
		return nil
	}
	if dismissedAt != nil && now.Sub(*dismissedAt) < DismissFor {
		return nil
	}
	last := answers[len(answers)-Window:]
	correct := 0
	for _, ok := range last {
		if ok {
			correct++
		}
	}
	switch {
	case correct >= UpCorrect:
		if to, ok := Shift(level, 1); ok {
			return &Suggestion{Direction: Up, From: level, To: to, Correct: correct, Total: Window}
		}
	case correct <= DownCorrect:
		if to, ok := Shift(level, -1); ok {
			return &Suggestion{Direction: Down, From: level, To: to, Correct: correct, Total: Window}
		}
	}
	return nil
}
