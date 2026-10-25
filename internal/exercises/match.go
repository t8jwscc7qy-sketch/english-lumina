package exercises

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MatchPairs — сколько пар в упражнении «сопоставление».
const MatchPairs = 5

const (
	matchMaxRuLen = 24 // перевод длиннее для плитки не годится
	matchMinRuLen = 3  // двухбуквенные сокращения словаря («ма») не годятся
)

var matchWordRe = regexp.MustCompile(`^[A-Za-z]{2,20}$`)

// MatchCandidate — слово и значения из словаря по порядку; в каждом значении русские варианты.
type MatchCandidate struct {
	Word   string
	Senses [][]string
}

// MatchPair — английское слово и его русский перевод.
type MatchPair struct {
	Word, Ru string
}

// Match — собранный набор: слова в порядке кандидатов, переводы вперемешку, верные пары по словам.
type Match struct {
	Words, Options []string
	Answers        []MatchPair
}

// MatchError — ответ нельзя разобрать или он не из этого упражнения; Message показывается пользователю по-русски.
type MatchError struct{ Message string }

func (e *MatchError) Error() string { return e.Message }

func matchErr(format string, a ...any) error { return &MatchError{Message: fmt.Sprintf(format, a...)} }

// PairResult — разбор одной пары ответа.
type PairResult struct {
	Word, Given, Answer string
	Correct             bool
}

// normalizeRu приводит перевод к виду для сравнения: без знаков ударения, строчными, с одиночными пробелами.
func normalizeRu(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '́' || r == '̀' {
			return -1
		}
		return r
	}, s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// validRu — перевод одним словом до matchMaxRuLen символов, с кириллицей, без разделителей пар.
func validRu(s string) bool {
	if n := utf8.RuneCountInString(s); n < matchMinRuLen || n > matchMaxRuLen || strings.ContainsAny(s, " ;=") {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) >= 0
}

// PickTranslation выбирает основной перевод слова среди его значений: вариант, который встречается в наибольшем
// числе значений; при равенстве выигрывает стоящий раньше (по значению, затем по месту в значении). Годятся только
// однословные переводы. Если значений несколько, а ни один вариант не повторяется, перевод неоднозначен: false.
func PickTranslation(senses [][]string) (string, bool) {
	type stat struct {
		count, firstSense, firstPos int
	}
	stats := map[string]*stat{}
	usable := 0
	for si, sense := range senses {
		seen := map[string]bool{}
		any := false
		for pi, raw := range sense {
			ru := normalizeRu(raw)
			if !validRu(ru) {
				continue
			}
			any = true
			if seen[ru] {
				continue
			}
			seen[ru] = true
			if st, ok := stats[ru]; ok {
				st.count++
			} else {
				stats[ru] = &stat{count: 1, firstSense: si, firstPos: pi}
			}
		}
		if any {
			usable++
		}
	}
	if len(stats) == 0 {
		return "", false
	}
	best, bestStat := "", (*stat)(nil)
	for ru, st := range stats {
		switch {
		case bestStat == nil, st.count > bestStat.count,
			st.count == bestStat.count && (st.firstSense < bestStat.firstSense ||
				st.firstSense == bestStat.firstSense && (st.firstPos < bestStat.firstPos ||
					st.firstPos == bestStat.firstPos && ru < best)):
			best, bestStat = ru, st
		}
	}
	if usable > 1 && bestStat.count < 2 {
		return "", false
	}
	return best, true
}

// BuildMatch собирает набор из первых подходящих кандидатов: слово из латинских букв (2–20), однозначный перевод,
// ни слово, ни перевод не повторяются. Меньше MatchPairs подходящих — false. Порядок переводов задаёт seed.
func BuildMatch(cands []MatchCandidate, seed int64) (Match, bool) {
	var m Match
	words, rus := map[string]bool{}, map[string]bool{}
	for _, c := range cands {
		if len(m.Answers) == MatchPairs {
			break
		}
		if !matchWordRe.MatchString(c.Word) || words[strings.ToLower(c.Word)] {
			continue
		}
		ru, ok := PickTranslation(c.Senses)
		if !ok || rus[ru] {
			continue
		}
		words[strings.ToLower(c.Word)], rus[ru] = true, true
		m.Words = append(m.Words, c.Word)
		m.Answers = append(m.Answers, MatchPair{Word: c.Word, Ru: ru})
	}
	if len(m.Answers) < MatchPairs {
		return Match{}, false
	}
	m.Options = make([]string, MatchPairs)
	for i, p := range m.Answers {
		m.Options[i] = p.Ru
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x6d617463)) //nolint:gosec // перемешивание упражнения, не защита
	rng.Shuffle(len(m.Options), func(i, j int) { m.Options[i], m.Options[j] = m.Options[j], m.Options[i] })
	return m, true
}

// CheckMatch сверяет ответ «слово=перевод;слово=перевод;…» с верными парами. Порядок пар в ответе любой, регистр,
// пробелы и ударения не важны. Результат идёт в порядке слов упражнения; второе значение — число верных пар.
// Неверный формат или чужие слова и переводы — ошибка с понятным русским текстом.
func CheckMatch(answers []MatchPair, given string) ([]PairResult, int, error) {
	n := len(answers)
	byWord := make(map[string]int, n)
	options := make(map[string]bool, n)
	for i, a := range answers {
		byWord[strings.ToLower(a.Word)] = i
		options[normalizeRu(a.Ru)] = true
	}
	parts := strings.Split(strings.TrimSpace(given), ";")
	for len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1] // хвостовой «;» не считается парой
	}
	if len(parts) != n {
		return nil, 0, matchErr("Нужно соединить все %d слов, а в ответе пар: %d", n, len(parts))
	}
	got := make([]string, n)
	filled := make([]bool, n)
	usedRu := make(map[string]bool, n)
	for _, part := range parts {
		w, ru, ok := strings.Cut(part, "=")
		if !ok {
			return nil, 0, matchErr("Не удалось разобрать ответ: каждая пара должна быть записана как «слово=перевод»")
		}
		word, ruNorm := strings.ToLower(strings.TrimSpace(w)), normalizeRu(ru)
		i, known := byWord[word]
		if !known {
			return nil, 0, matchErr("Слово «%s» не из этого упражнения", strings.TrimSpace(w))
		}
		if filled[i] {
			return nil, 0, matchErr("Слово «%s» указано дважды", strings.TrimSpace(w))
		}
		if !options[ruNorm] {
			return nil, 0, matchErr("Перевод «%s» не из этого упражнения", strings.TrimSpace(ru))
		}
		if usedRu[ruNorm] {
			return nil, 0, matchErr("Перевод «%s» указан дважды", strings.TrimSpace(ru))
		}
		usedRu[ruNorm], filled[i], got[i] = true, true, ruNorm
	}
	res := make([]PairResult, n)
	right := 0
	for i, a := range answers {
		ok := got[i] == normalizeRu(a.Ru)
		if ok {
			right++
		}
		res[i] = PairResult{Word: a.Word, Given: got[i], Answer: a.Ru, Correct: ok}
	}
	return res, right, nil
}
