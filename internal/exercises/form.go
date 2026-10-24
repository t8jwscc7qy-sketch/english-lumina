package exercises

import (
	"slices"
	"strings"
)

// timeMarkers — слова, по которым время глагола видно из предложения; без них форма спорна.
var timeMarkers = []string{
	"yesterday", "ago", "last", "tomorrow", "next", "now", "today", "tonight",
	"every", "always", "usually", "often", "never", "already", "yet", "just",
}

// ambiguousWords — слова с совпадающими формами: по предложению не понять, какая форма имеется в виду.
// Проверяется по формам слова, потому что леммы BuildForm не получает.
var ambiguousWords = []string{
	"read", "lead", "saw", "put", "set", "cut", "hit", "let", "shut", "hurt",
	"cost", "spread", "quit", "bet", "beat",
}

// BuildForm собирает «выбор формы»: пропуск на месте единственной формы слова в предложении,
// варианты — остальные формы. forms — однословные формы в нижнем регистре; answer — найденная
// форма в написании предложения. false, если у слова меньше трёх разных форм, оно неоднозначно,
// в предложении не ровно одно вхождение форм слова или у глагола нет маркера времени.
func BuildForm(sentence, pos string, forms []string, seed int64) (gap Gap, answer string, ok bool) {
	var uniq []string
	for _, f := range forms {
		if f = strings.ToLower(strings.TrimSpace(f)); f != "" && !slices.Contains(uniq, f) {
			uniq = append(uniq, f)
		}
	}
	if len(uniq) < 3 {
		return Gap{}, "", false
	}
	for _, f := range uniq {
		if slices.Contains(ambiguousWords, f) {
			return Gap{}, "", false
		}
	}
	target, found := "", 0
	for _, f := range uniq {
		if n := len(findWords(sentence, f)); n > 0 {
			target, found = f, found+n
		}
	}
	if found != 1 {
		return Gap{}, "", false
	}
	if pos == "verb" && !hasTimeMarker(sentence) {
		return Gap{}, "", false
	}
	g, ok := build(sentence, target, uniq, 2, seed)
	if !ok {
		return Gap{}, "", false
	}
	return g, g.Answers[0], true
}

func hasTimeMarker(sentence string) bool {
	words := strings.FieldsFunc(strings.ToLower(sentence), func(r rune) bool { return !wordRune(r) })
	return slices.ContainsFunc(words, func(w string) bool { return slices.Contains(timeMarkers, w) })
}
