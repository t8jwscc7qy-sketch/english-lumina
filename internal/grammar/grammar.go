// Package grammar определяет грамматические темы предложения по его словам.
// Темы нужны упражнению «пропуск» и рейтингу по темам. Функции чистые.
// Спорные случаи пропускаем, а не угадываем: лучше не дать тему, чем научить неверному.
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

// topicWords — слова-кандидаты пропуска. «going» сюда не входит: оно значит будущее
// только в «going to», а «to» и частицы фразовых глаголов — не предлоги.
var topicWords = map[string][]string{
	Articles: {"a", "an", "the"},
	Prepositions: {"in", "on", "at", "from", "with", "by", "for", "of", "about", "into", "over", "under",
		"between", "through", "after", "before", "during", "without", "against", "among", "behind", "near",
		"across", "around", "above", "below"},
	Future: {"will"},
	Modals: {"can", "could", "must", "should", "may", "might", "would"},
}

// order задаёт порядок тем в результате Topics.
var order = []string{Articles, Prepositions, Future, Modals, PastSimple}

// beforeWill — слова, после которых will — существительное («free will», «my will»).
var beforeWill = []string{"a", "the", "my", "his", "her", "your", "our", "their", "free"}

// WillNounPrecursors возвращает слова, после которых will — существительное, а не будущее время.
func WillNounPrecursors() []string {
	return slices.Clone(beforeWill)
}

// beforeParticiple — слова, после которых форма на -ed или неправильная форма —
// причастие или прилагательное («have gone», «am tired»), а не прошедшее время.
var beforeParticiple = []string{"have", "has", "had", "is", "am", "are", "was", "were", "be", "been", "get", "got", "very"}

// notPast — слова на «ed», которые не глаголы в прошедшем времени (в том числе прилагательные).
var notPast = []string{"speed", "hundred", "indeed", "breed", "greed", "kindred", "hatred", "sacred", "naked",
	"wicked", "need", "seed", "feed", "bleed", "proceed", "succeed", "exceed",
	// чаще прилагательные, чем глаголы: «a bit tired», «he got married»
	"tired", "married", "excited", "bored", "interested", "worried", "surprised", "scared"}

// Words возвращает слова-кандидаты для пропуска в теме (нижний регистр).
// У past_simple закрытого списка нет, как и у неизвестной темы, — вернётся nil.
func Words(topic string) []string {
	return slices.Clone(topicWords[topic])
}

// Topics возвращает темы, встретившиеся среди слов, без повторов, в порядке перечисления.
// Токены должны быть без пунктуации и в нижнем регистре (регистр всё же не важен).
// past_simple: неправильная форма прошедшего времени или слово длиннее 4 букв на «ed»,
// если оно не из списка исключений и перед ним нет вспомогательного глагола.
func Topics(tokens []string) []string {
	found := map[string]bool{}
	for i, tok := range tokens {
		w := strings.ToLower(tok)
		prev, next := "", ""
		if i > 0 {
			prev = strings.ToLower(tokens[i-1])
		}
		if i+1 < len(tokens) {
			next = strings.ToLower(tokens[i+1])
		}
		for topic, list := range topicWords {
			if !slices.Contains(list, w) || (topic == Future && slices.Contains(beforeWill, prev)) {
				continue
			}
			found[topic] = true
		}
		if w == "going" && next == "to" {
			found[Future] = true
		}
		isPast := words.IrregularPast(w) ||
			(len(w) > 4 && strings.HasSuffix(w, "ed") && !slices.Contains(notPast, w))
		if isPast && !slices.Contains(beforeParticiple, prev) {
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
