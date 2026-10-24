package words

import (
	"slices"
	"strings"
	"unicode"
)

type token struct {
	word       string // в нижнем регистре
	start, end int    // позиции в символах (рунах) исходного текста, end не включается
}

// tokenizeWithOffsets режет текст на слова так же, как tokenize, и запоминает, где они стоят в тексте.
func tokenizeWithOffsets(text string) []token {
	var out []token
	runes := []rune(text)
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) && !unicode.IsDigit(runes[i]) {
			i++
			continue
		}
		j := i
		for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
			j++
		}
		out = append(out, token{word: strings.ToLower(string(runes[i:j])), start: i, end: j})
		i = j
	}
	return out
}

// FindMatch находит в тексте первое (самое раннее) вхождение любой из форм и возвращает его границы
// в символах (start включительно, end нет). На одной позиции побеждает более длинная форма (фраза).
// Слово ищется целиком: «run» не совпадает с «running». Слова делятся как в полнотекстовом поиске, значит
// «don't» — это «don» и «t»: подсвечивается то же, что нашёл поиск.
func FindMatch(text string, forms [][]string) (start, end int, ok bool) {
	tokens := tokenizeWithOffsets(text)
	for i := range tokens {
		best := 0
		for _, f := range forms {
			if len(f) <= best || i+len(f) > len(tokens) {
				continue
			}
			words := make([]string, len(f))
			for k := range f {
				words[k] = tokens[i+k].word
			}
			if slices.Equal(words, f) {
				best = len(f)
			}
		}
		if best > 0 {
			return tokens[i].start, tokens[i+best-1].end, true
		}
	}
	return 0, 0, false
}
