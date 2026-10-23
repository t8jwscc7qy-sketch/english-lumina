// Package exercises собирает и проверяет упражнения. Функции чистые:
// случайность задаётся числом seed, поэтому результат повторяем.
package exercises

import (
	"math/rand/v2"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
)

// blank — пропуск в тексте упражнения.
const blank = "_____"

// maxAnswerLen — длиннее этого ответ не принимается.
const maxAnswerLen = 200

// Gap — упражнение «пропуск».
type Gap struct {
	Prompt  string   // предложение с blank на месте слова
	Choices []string // 4 уникальных варианта в перемешанном порядке
	Answers []string // допустимые написания правильного ответа (нижний регистр)
}

// extras добивают варианты, если в теме меньше четырёх слов.
var extras = map[string][]string{
	grammar.Articles: {"this"},
	grammar.Future:   {"would", "shall", "can"},
}

// BuildGap собирает пропуск для грамматической темы с закрытым списком слов
// (grammar.Words). Слово-цель — первое слово предложения из этого списка.
// Для past_simple и тем level:/pos: нужен BuildGapForWord. false, если собрать нельзя.
func BuildGap(sentence, topic string, tokens []string, seed int64) (Gap, bool) {
	pool := grammar.Words(topic)
	if len(pool) == 0 {
		return Gap{}, false
	}
	willNoun := grammar.WillNounPrecursors()
	target := ""
	for i, tok := range tokens {
		w := strings.ToLower(tok)
		if !slices.Contains(pool, w) {
			continue
		}
		if topic == grammar.Future && i > 0 && slices.Contains(willNoun, strings.ToLower(tokens[i-1])) {
			continue
		}
		target = w
		break
	}
	if target == "" {
		return Gap{}, false
	}
	rest := slices.DeleteFunc(slices.Clone(pool), func(w string) bool { return w == target })
	rest = append(rest, extras[topic]...)
	return build(sentence, target, rest, seed)
}

// BuildGapForWord собирает пропуск на месте слова target; отвлекающие (формы слова,
// однотипные слова) передаёт вызывающий. Нужно минимум 3 разных, не равных target. false, если
// слова нет в предложении или отвлекающих не хватает.
func BuildGapForWord(sentence, target string, distractors []string, seed int64) (Gap, bool) {
	return build(sentence, strings.ToLower(target), distractors, seed)
}

func build(sentence, target string, distractors []string, seed int64) (Gap, bool) {
	// Слово должно встречаться ровно один раз: иначе выбор вхождения спорный
	// или второе вхождение подсказывает ответ.
	spans := findWords(sentence, target)
	if len(spans) != 1 {
		return Gap{}, false
	}
	start, end := spans[0][0], spans[0][1]
	var uniq []string
	for _, d := range distractors {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && d != target && !slices.Contains(uniq, d) {
			uniq = append(uniq, d)
		}
	}
	if len(uniq) < 3 {
		return Gap{}, false
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
	rng.Shuffle(len(uniq), func(i, j int) { uniq[i], uniq[j] = uniq[j], uniq[i] })
	choices := append([]string{target}, uniq[:3]...)
	rng.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })

	if r, _ := utf8.DecodeRuneInString(sentence[start:end]); unicode.IsUpper(r) {
		for i, c := range choices {
			choices[i] = capitalize(c)
		}
	}
	return Gap{
		Prompt:  sentence[:start] + blank + sentence[end:],
		Choices: choices,
		Answers: []string{target},
	}, true
}

// findWords ищет все вхождения слова целиком без учёта регистра и возвращает границы в байтах.
// Апострофы и дефисы внутри слова границей не считаются (I'm, well-known).
func findWords(s, word string) [][2]int {
	lower := strings.ToLower(s)
	// ToLower может менять длину в байтах (например «İ»): тогда границы не совпадут.
	if word == "" || len(lower) != len(s) {
		return nil
	}
	var out [][2]int
	for from := 0; from < len(lower); {
		i := strings.Index(lower[from:], word)
		if i < 0 {
			break
		}
		i += from
		j := i + len(word)
		before, _ := utf8.DecodeLastRuneInString(lower[:i])
		after, _ := utf8.DecodeRuneInString(lower[j:])
		if (i == 0 || !wordRune(before)) && (j == len(lower) || !wordRune(after)) {
			out = append(out, [2]int{i, j})
			from = j
		} else {
			from = i + 1
		}
	}
	return out
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '’' || r == '-'
}

func capitalize(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// Check сравнивает ответ с допустимыми без учёта регистра и пробелов по краям.
// Пустой ответ и ответ длиннее 200 символов неверны.
func Check(answers []string, given string) bool {
	given = strings.ToLower(strings.TrimSpace(given))
	if given == "" || utf8.RuneCountInString(given) > maxAnswerLen {
		return false
	}
	for _, a := range answers {
		if strings.ToLower(strings.TrimSpace(a)) == given {
			return true
		}
	}
	return false
}
