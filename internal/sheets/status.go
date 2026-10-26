package sheets

import "slices"

// Статусы слова в «умной» таблице.
const (
	WordUnseen       = "unseen"         // не встречал
	WordIntroduced   = "introduced"     // показано на «Знакомстве», ещё не вспомнено хорошо
	WordLearning     = "learning"       // есть карточка, но состояния изучения нет
	WordLearned      = "learned"        // изучено
	WordKnownAtStart = "known_at_start" // отмечено «знаю» в быстром тесте
	WordHard         = "hard"           // трудное: забывал три раза и больше
)

// hardLapses — с этого числа забываний слово считается трудным.
const hardLapses = 3

// WordStatus — статус слова по данным пользователя: состояние изучения (nil — строки нет), есть ли карточка,
// сколько раз забыл. Трудное важнее остальных: оно требует внимания, даже если слово числится изученным.
func WordStatus(learning *string, hasCard bool, lapses int) string {
	if hasCard && lapses >= hardLapses {
		return WordHard
	}
	if learning != nil {
		switch *learning {
		case "learned":
			return WordLearned
		case "known_at_start":
			return WordKnownAtStart
		case "introduced":
			return WordIntroduced
		}
	}
	if hasCard {
		return WordLearning
	}
	return WordUnseen
}

// statusPriority — какой статус показать, если у леммы несколько слов (run: глагол и существительное): самый «живой».
var statusPriority = []string{WordHard, WordLearning, WordIntroduced, WordLearned, WordKnownAtStart, WordUnseen}

// BestStatus выбирает самый «живой» из статусов; пустой список — unseen.
func BestStatus(statuses ...string) string {
	best := len(statusPriority)
	for _, s := range statuses {
		if i := slices.Index(statusPriority, s); i >= 0 && i < best {
			best = i
		}
	}
	if best == len(statusPriority) {
		return WordUnseen
	}
	return statusPriority[best]
}

// TopicWeak — рейтинг темы в нижней трети среди всех тем пользователя: темы с таким рейтингом и ниже целиком
// помещаются в нижнюю треть. Одинаковые рейтинги не делают слабыми всех: у трёх равных тем слабой нет. Тем меньше
// трёх — «слабой» не бывает: сравнивать не с чем.
func TopicWeak(rating float64, all []float64) bool {
	if len(all) < 3 {
		return false
	}
	notAbove := 0
	for _, r := range all {
		if r <= rating {
			notAbove++
		}
	}
	return notAbove <= len(all)/3
}

// MistakeRow — открытая ошибка для шпаргалки «Мои ошибки».
type MistakeRow struct {
	Prompt string // задание
	Given  string // твой ответ
	Answer string // верный ответ
}

// MineMistakes собирает таблицу «Мои ошибки» из открытых ошибок. Нет ошибок — таблица без строк (клиент покажет пояснение).
func MineMistakes(rows []MistakeRow) []Row {
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, Row{Cells: []string{r.Prompt, r.Given, r.Answer}})
	}
	return out
}
