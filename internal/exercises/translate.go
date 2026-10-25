package exercises

import (
	"strings"
	"unicode/utf8"
)

// Границы упражнения «перевод»: слов в предложении и длина в байтах.
const (
	TranslateMinWords = 4
	TranslateMaxWords = 9
	translateMaxLen   = 80
	// typoMinLen — короче нормализованный вариант опечатку не прощает: на коротких фразах одна буква меняет слово (he ↔ we).
	typoMinLen = 12
	// typoMinWord — опечатку прощаем только в слове не короче этого числа букв (в коротких меняется само слово).
	typoMinWord = 4
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
// предложения. false, если предложение не подходит: 4–9 слов, не длиннее 80 символов, только ASCII-буквы, цифры,
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
		if len(ar) >= typoMinLen && typoOf(gr, ar) {
			return TranslateVerdict{Result: TranslateTypo, Matched: a}
		}
	}
	return TranslateVerdict{}
}

// contractions раскрываются слева направо по тексту; «won't», «can't», «cannot», «shan't» стоят отдельно, чтобы не
// превратиться в «wo not» и «ca not» через общее «n't». «'s» и «'d» двусмысленны
// (is/has, would/had) и остаются как есть: если нужны оба написания, они приходят альтернативами.
var contractions = strings.NewReplacer(
	"won't", "will not", "can't", "can not", "cannot", "can not", "shan't", "shall not",
	"n't", " not", "'m", " am", "'re", " are", "'ve", " have", "'ll", " will",
)

// normTranslate приводит предложение к виду для сравнения: нижний регистр, один вид апострофа, дефис как пробел
// (well-known = well known), раскрытые
// однозначные сокращения, без знаков , . ! ?, одиночные пробелы.
func normTranslate(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("’", "'", "‘", "'", "ʼ", "'", "`", "'", "´", "'", "-", " ").Replace(s)
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

// typoOf: given — опечатка в expected (обе строки нормализованы). Опечатка — одна правка (замена, лишняя или
// пропущенная буква, перестановка соседних) в слове не короче четырёх букв. Грамматику и смысл опечаткой не считаем:
// не прощаются правки на пробеле (слияние и разрыв слов), короткие слова (a/an, he/she, now/not) и «s»/«d» в конце
// слова (число, лицо, прошедшее время).
func typoOf(given, expected []rune) bool {
	a, b := given, expected
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	switch {
	case len(a) == len(b):
		if i == len(a) {
			return false
		}
		if equalRunes(a[i+1:], b[i+1:]) { // замена
			return wordEdit(b, i, a, i) && !spaceOrInflection(a, i) && !spaceOrInflection(b, i)
		}
		if i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && equalRunes(a[i+2:], b[i+2:]) { // перестановка
			return wordEdit(b, i, a, i) && !spaceOrInflection(a, i) && !spaceOrInflection(a, i+1) &&
				!spaceOrInflection(b, i) && !spaceOrInflection(b, i+1)
		}
	case len(a) == len(b)+1: // лишняя буква в ответе
		return equalRunes(a[i+1:], b[i:]) && wordEdit(b, min(i, len(b)-1), a, i) && !spaceOrInflection(a, i)
	case len(b) == len(a)+1: // пропущенная буква
		return equalRunes(a[i:], b[i+1:]) && wordEdit(b, i, a, min(i, len(a)-1)) && !spaceOrInflection(b, i)
	}
	return false
}

func equalRunes(a, b []rune) bool { return string(a) == string(b) }

// wordEdit: правка лежит в слове не короче четырёх букв (смотрим слово ожидаемой фразы и слово ответа).
func wordEdit(b []rune, bi int, a []rune, ai int) bool {
	return wordLen(b, bi) >= typoMinWord && wordLen(a, ai) >= typoMinWord-1
}

// wordLen — длина слова, в котором стоит символ r[i] (если это пробел, то слова слева).
func wordLen(r []rune, i int) int {
	if i >= len(r) {
		i = len(r) - 1
	}
	if i > 0 && r[i] == ' ' {
		i--
	}
	start, end := i, i
	for start > 0 && r[start-1] != ' ' {
		start--
	}
	for end+1 < len(r) && r[end+1] != ' ' {
		end++
	}
	return end - start + 1
}

// spaceOrInflection: символ r[i] — пробел или «s»/«d» на конце слова.
func spaceOrInflection(r []rune, i int) bool {
	if i >= len(r) {
		return false
	}
	if r[i] == ' ' {
		return true
	}
	atEnd := i == len(r)-1 || r[i+1] == ' '
	return atEnd && (r[i] == 's' || r[i] == 'd')
}
