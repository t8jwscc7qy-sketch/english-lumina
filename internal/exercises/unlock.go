package exercises

// Gate — условие допуска к умению: рейтинг другой темы не ниже MinRating
// при не менее чем MinAttempts попыток.
type Gate struct {
	Requires    string
	MinRating   float64
	MinAttempts int
}

// gates — таблица допусков. Пока пуста: до первого закрытого умения всё доступно.
var gates = map[string]Gate{}

// Unlocked говорит, открыта ли тема пользователю.
func Unlocked(topic string, ratings []TopicRating) bool {
	return unlocked(topic, ratings, gates)
}

func unlocked(topic string, ratings []TopicRating, gates map[string]Gate) bool {
	g, ok := gates[topic]
	if !ok {
		return true
	}
	for _, r := range ratings {
		if r.Topic == g.Requires {
			return r.Mu >= g.MinRating && r.Attempts >= g.MinAttempts
		}
	}
	return false
}
