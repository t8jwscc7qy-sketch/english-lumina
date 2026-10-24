package exercises

// TopicRating — рейтинг пользователя по теме и число попыток.
type TopicRating struct {
	Topic    string
	Mu, Phi  float64
	Attempts int
}

// PickTopic выбирает тему для следующего упражнения из доступных.
// Тема без попыток идёт первой (по порядку available); иначе берётся тема с наименьшим Mu,
// а при равенстве — с наибольшим Phi (о ней известно меньше всего). Пустой available — false.
func PickTopic(ratings []TopicRating, available []string) (string, bool) {
	byTopic := make(map[string]TopicRating, len(ratings))
	for _, r := range ratings {
		byTopic[r.Topic] = r
	}
	best, found := "", false
	var bestR TopicRating
	for _, t := range available {
		r, ok := byTopic[t]
		if !ok || r.Attempts == 0 {
			return t, true
		}
		if !found || r.Mu < bestR.Mu || (r.Mu == bestR.Mu && r.Phi > bestR.Phi) {
			best, bestR, found = t, r, true
		}
	}
	return best, found
}
