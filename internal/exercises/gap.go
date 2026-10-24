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
	Choices []string // 3–4 уникальных варианта в перемешанном порядке
	Answers []string // допустимые написания ответа; Answers[0] совпадает с вариантом дословно (в начале предложения — с заглавной)
}

// extras добивают варианты, если в теме мало слов. Только заведомо неверные: после will стоит глагол
// в начальной форме, поэтому «is», «been», «to» там невозможны. Для артиклей добора нет
// («this», «my» бывают верны), вариантов будет три.
var extras = map[string][]string{
	grammar.Future: {"is", "been", "to"},
}

// minDistractors — сколько отвлекающих нужно минимум: у тем с закрытым списком вариантов бывает три (артикли).
const (
	minDistractors     = 3 // для BuildGapForWord
	minGrammarDistract = 2 // для BuildGap
)

// BuildGap собирает пропуск для грамматической темы с закрытым списком слов
// (grammar.Words). Токены — в исходном регистре (см. grammar.IsTopicWord). Слово-цель — первое слово предложения из этого списка.
// Для past_simple и тем level:/pos: нужен BuildGapForWord. false, если собрать нельзя.
func BuildGap(sentence, topic string, tokens []string, seed int64) (Gap, bool) {
	pool := grammar.Words(topic)
	if len(pool) == 0 {
		return Gap{}, false
	}
	target := ""
	for i, tok := range tokens {
		w := strings.ToLower(tok)
		if !slices.Contains(pool, w) || !grammar.IsTopicWord(topic, tokens, i) {
			continue
		}
		target = w
		break
	}
	if target == "" {
		return Gap{}, false
	}
	rest := slices.DeleteFunc(slices.Clone(pool), func(w string) bool { return w == target || grammar.Interchangeable(w, target) })
	rest = append(rest, extras[topic]...)
	return build(sentence, target, rest, minGrammarDistract, seed)
}

// BuildGapForWord собирает пропуск на месте слова target; отвлекающие (формы слова,
// однотипные слова) передаёт вызывающий. Нужно минимум 3 разных, не равных target. false, если
// слова нет в предложении или отвлекающих не хватает.
func BuildGapForWord(sentence, target string, distractors []string, seed int64) (Gap, bool) {
	return build(sentence, strings.ToLower(target), distractors, minDistractors, seed)
}

func build(sentence, target string, distractors []string, minDistr int, seed int64) (Gap, bool) {
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
	uniq = agreeWithArticle(sentence, start, target, uniq)
	if len(uniq) < minDistr {
		return Gap{}, false
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
	rng.Shuffle(len(uniq), func(i, j int) { uniq[i], uniq[j] = uniq[j], uniq[i] })
	choices := append([]string{target}, uniq[:min(3, len(uniq))]...)
	rng.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })

	answer := target
	if r, _ := utf8.DecodeRuneInString(sentence[start:end]); unicode.IsUpper(r) {
		for i, c := range choices {
			choices[i] = capitalize(c)
		}
		answer = capitalize(target)
	}
	return Gap{
		Prompt:  sentence[:start] + blank + sentence[end:],
		Choices: choices,
		Answers: []string{answer},
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

// agreeWithArticle: если перед словом стоит a/an, оставляет отвлекающие, начинающиеся на тот же звук
// (по первой букве: гласная или согласная), иначе артикль выдаёт ответ. Эвристика грубая («an hour»), но
// ошибка только сужает выбор. Не хватит отвлекающих — build откажется от предложения.
func agreeWithArticle(sentence string, start int, target string, distractors []string) []string {
	fields := strings.Fields(sentence[:start])
	if len(fields) == 0 {
		return distractors
	}
	switch strings.ToLower(strings.Trim(fields[len(fields)-1], ".,;:!?\"()")) {
	case "a", "an":
	default:
		return distractors
	}
	vowel := func(w string) bool {
		r, _ := utf8.DecodeRuneInString(w)
		return strings.ContainsRune("aeiou", r)
	}
	want := vowel(target)
	return slices.DeleteFunc(slices.Clone(distractors), func(d string) bool { return vowel(d) != want })
}
