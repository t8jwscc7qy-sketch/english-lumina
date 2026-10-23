// Package grammar определяет грамматические темы предложения по его словам.
// Темы нужны упражнению «пропуск» и рейтингу по темам. Функции чистые.
package grammar

import (
	"slices"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// Ключи тем в порядке перечисления.
const (
	Articles     = "grammar:articles"
	Prepositions = "grammar:prepositions"
	Future       = "grammar:future"
	Modals       = "grammar:modals"
	PastSimple   = "grammar:past_simple"
)

var topicWords = map[string][]string{
	Articles: {"a", "an", "the"},
	Prepositions: {"in", "on", "at", "to", "from", "with", "by", "for", "of", "about", "into", "over", "under",
		"between", "through", "after", "before", "during", "without", "against", "among", "behind", "near",
		"across", "around", "above", "below", "off", "up", "down"},
	Future: {"will", "going"},
	Modals: {"can", "could", "must", "should", "may", "might", "would"},
}

// order задаёт порядок тем в результате Topics.
var order = []string{Articles, Prepositions, Future, Modals, PastSimple}

// Words возвращает слова-кандидаты для пропуска в теме (нижний регистр).
// У past_simple закрытого списка нет, как и у неизвестной темы, — вернётся nil.
func Words(topic string) []string {
	return slices.Clone(topicWords[topic])
}

// Topics возвращает темы, встретившиеся среди слов, без повторов, в порядке перечисления.
// past_simple: неправильная форма из таблицы или слово длиннее 4 букв на «ed».
func Topics(tokens []string) []string {
	found := map[string]bool{}
	for _, tok := range tokens {
		w := strings.ToLower(tok)
		for topic, list := range topicWords {
			if slices.Contains(list, w) {
				found[topic] = true
			}
		}
		if words.IrregularPast(w) || (len(w) > 4 && strings.HasSuffix(w, "ed")) {
			found[PastSimple] = true
		}
	}
	var out []string
	for _, topic := range order {
		if found[topic] {
			out = append(out, topic)
		}
	}
	return out
}
