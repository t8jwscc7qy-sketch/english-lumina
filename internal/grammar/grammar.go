// Package grammar определяет грамматические темы предложения по его словам.
// Темы нужны упражнению «пропуск» и рейтингу по темам. Функции чистые.
// Спорные случаи пропускаем, а не угадываем: лучше не дать тему, чем научить неверному.
package grammar

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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

// interchangeable — группы слов, которые в большинстве предложений подходят друг другу по смыслу.
// Упражнение не ставит слова одной группы друг другу в отвлекающие: иначе верный ответ выдаётся за неверный.
// Группы нарочно широкие: лучше потерять вариант, чем засчитать верное слово ошибкой.
// Известный риск: a/the по переводу не различаются («I saw a dog» и «I saw the dog» переводятся одинаково),
// поэтому в артиклях полагаться можно только на контекст предложения.
var interchangeable = [][]string{
	{"can", "could", "may"}, {"may", "might", "could"}, {"would", "could", "might"}, {"should", "must"}, {"should", "would"},
	{"over", "above"}, {"under", "below"}, {"among", "between"}, {"in", "into"}, {"around", "about"},
	{"near", "by"}, {"for", "during"}, {"across", "through"}, {"at", "in"},
}

// Interchangeable сообщает, что слова из одной группы взаимозаменяемых (регистр не важен).
func Interchangeable(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for _, g := range interchangeable {
		if slices.Contains(g, a) && slices.Contains(g, b) {
			return true
		}
	}
	return false
}

// order задаёт порядок тем в результате Topics.
var order = []string{Articles, Prepositions, Future, Modals, PastSimple}

// beforeWill — слова, после которых will — существительное («free will», «my will»).
var beforeWill = []string{"a", "the", "my", "his", "her", "your", "our", "their", "free"}

// IsTopicWord сообщает, что tokens[i] в закрытом списке слов темы и действительно относится к ней.
// Токены передаются как в тексте, с исходным регистром: слово с заглавной не в начале предложения
// («in May», «Call Will») для future и modals — имя или месяц. Дальше отсекаются существительные:
// will после «a/the/my/free», can и may после тех же слов и can перед «of» («a can of soup»).
func IsTopicWord(topic string, tokens []string, i int) bool {
	if i < 0 || i >= len(tokens) {
		return false
	}
	tok := tokens[i]
	w := strings.ToLower(tok)
	if !slices.Contains(topicWords[topic], w) {
		return false
	}
	if topic != Future && topic != Modals {
		return true
	}
	if r, _ := utf8.DecodeRuneInString(tok); i > 0 && unicode.IsUpper(r) {
		return false
	}
	prev, next := "", ""
	if i > 0 {
		prev = strings.ToLower(tokens[i-1])
	}
	if i+1 < len(tokens) {
		next = strings.ToLower(tokens[i+1])
	}
	switch w {
	case "will", "may":
		return !slices.Contains(beforeWill, prev)
	case "can":
		return !slices.Contains(beforeWill, prev) && next != "of"
	}
	return true
}

// beforeParticiple — слова, после которых форма на -ed или неправильная форма —
// причастие или прилагательное («have gone», «am tired»), а не прошедшее время.
var beforeParticiple = []string{"have", "has", "had", "is", "am", "are", "was", "were", "be", "been", "get", "got", "very",
	// сокращения со вспомогательным глаголом: «I've walked», «he's gone»
	"i'm", "i've", "he's", "she's", "it's", "that's", "you're", "we're", "they're", "you've", "we've", "they've"}

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
// Токены — слова без пунктуации в исходном регистре (по нему отличаются имена: «in May», «Call Will»).
// past_simple: неправильная форма прошедшего времени или слово длиннее 4 букв на «ed»,
// если оно не из списка исключений и перед ним нет вспомогательного глагола.
func Topics(tokens []string) []string {
	found := map[string]bool{}
	for i, tok := range tokens {
		w := normQuote(tok)
		prev, next := "", ""
		if i > 0 {
			prev = normQuote(tokens[i-1])
		}
		if i+1 < len(tokens) {
			next = normQuote(tokens[i+1])
		}
		for topic := range topicWords {
			if IsTopicWord(topic, tokens, i) {
				found[topic] = true
			}
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

// normQuote — слово в нижнем регистре с обычным апострофом.
func normQuote(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "’", "'")
}
