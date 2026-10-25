package exercises

import (
	"strings"
	"unicode/utf8"
)

// Границы упражнения «перевод»: слов в предложении и длина в байтах.
const (
	TranslateMinWords = 3
	TranslateMaxWords = 9
	translateMaxLen   = 80
	// typoMinLen — короче нормализованный вариант опечатку не прощает: на коротких фразах одна буква меняет слово (he ↔ we).
	typoMinLen = 12
)

// Translate — упражнение «перевод»: русское предложение переводят по-английски вводом.
type Translate struct {
	Answers []string // допустимые английские предложения; Answers[0] — исходное
}

// TranslateResult — итог проверки ответа.
type TranslateResult int

const (
	TranslateWrong TranslateResult = iota
	TranslateTypo                  // совпало с точностью до одной буквы
	TranslateExact
)

// TranslateVerdict — итог проверки и допустимый вариант, с которым совпал ответ (пусто при TranslateWrong).
type TranslateVerdict struct {
	Result  TranslateResult
	Matched string
}

// BuildTranslate собирает упражнение из английского предложения и других английских переводов того же русского
// предложения. false, если предложение не подходит: 3–9 слов, не длиннее 80 символов, только ASCII-буквы, цифры,
// пробел и знаки , . ! ? ' ’ - (кавычки, скобки, многоточие, ; и : отбрасывают предложение). Негодные альтернативы
// и повторы после приведения пропускаются.
func BuildTranslate(sentence string, alternatives []string) (Translate, bool) {
	sentence = strings.TrimSpace(sentence)
	if !translatable(sentence) {
		return Translate{}, false
	}
	answers := []string{sentence}
	seen := map[string]bool{normTranslate(sentence): true}
	for _, alt := range alternatives {
		alt = strings.TrimSpace(alt)
		n := normTranslate(alt)
		if seen[n] || !translatable(alt) {
			continue
		}
		seen[n] = true
		answers = append(answers, alt)
	}
	return Translate{Answers: answers}, true
}

// CheckTranslate сверяет ответ с допустимыми: сначала точное совпадение после приведения (регистр, пунктуация,
// пробелы, однозначные сокращения), затем опечатка в одну букву. Пустой ответ и ответ длиннее 200 символов неверны.
func CheckTranslate(answers []string, given string) TranslateVerdict {
	given = strings.TrimSpace(given)
	if given == "" || utf8.RuneCountInString(given) > maxAnswerLen {
		return TranslateVerdict{}
	}
	g := normTranslate(given)
	if g == "" {
		return TranslateVerdict{}
	}
	for _, a := range answers {
		if normTranslate(a) == g {
			return TranslateVerdict{Result: TranslateExact, Matched: a}
		}
	}
	gr := []rune(g)
	for _, a := range answers {
		ar := []rune(normTranslate(a))
		if len(ar) >= typoMinLen && oneEdit(gr, ar) {
			return TranslateVerdict{Result: TranslateTypo, Matched: a}
		}
	}
	return TranslateVerdict{}
}

// TranslateTopics — ключи тем упражнения «перевод»: навык и уровень предложения (если он известен).
func TranslateTopics(level string) []string {
	if level == "" {
		return []string{"skill:translate"}
	}
	return []string{"skill:translate", "level:" + level}
}

// contractions раскрываются в таком порядке: частные случаи раньше общего «n't». «'s» и «'d» двусмысленны
// (is/has, would/had) и остаются как есть: если нужны оба написания, они приходят альтернативами.
var contractions = strings.NewReplacer(
	"won't", "will not", "can't", "can not", "cannot", "can not", "shan't", "shall not",
	"n't", " not", "'m", " am", "'re", " are", "'ve", " have", "'ll", " will",
)

// normTranslate приводит предложение к виду для сравнения: нижний регистр, один вид апострофа, раскрытые
// однозначные сокращения, без знаков , . ! ?, одиночные пробелы.
func normTranslate(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("’", "'", "‘", "'").Replace(s)
	s = contractions.Replace(s)
	s = strings.NewReplacer(",", "", ".", "", "!", "", "?", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// translatable: предложение годится для упражнения (см. BuildTranslate).
func translatable(s string) bool {
	if s == "" || len(s) > translateMaxLen {
		return false
	}
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune(" ,.!?'’-", r)
		if !ok {
			return false
		}
	}
	n := len(strings.Fields(s))
	return n >= TranslateMinWords && n <= TranslateMaxWords
}

// oneEdit: строки отличаются ровно одной вставкой, удалением, заменой или перестановкой соседних букв.
func oneEdit(a, b []rune) bool {
	if len(a) < len(b) {
		a, b = b, a
	}
	switch len(a) - len(b) {
	case 0:
		diff := -1
		n := 0
		for i := range a {
			if a[i] != b[i] {
				if n == 0 {
					diff = i
				}
				n++
			}
		}
		switch n {
		case 1:
			return true
		case 2:
			return diff+1 < len(a) && a[diff] == b[diff+1] && a[diff+1] == b[diff]
		}
		return false
	case 1:
		i := 0
		for i < len(b) && a[i] == b[i] {
			i++
		}
		for ; i < len(b); i++ {
			if a[i+1] != b[i] {
				return false
			}
		}
		return true
	}
	return false
}
