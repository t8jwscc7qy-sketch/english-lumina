package exercises

import (
	"slices"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// MaxFreeRank — слово без уровня допустимо, если оно в первых 20 000 частотного списка.
const MaxFreeRank = 20000

// WordInfo — что словарь знает о слове. Пустой Level — уровня нет, Rank 0 — слова нет в частотном списке.
type WordInfo struct {
	Level string
	Rank  int
}

// Tokens режет предложение на слова в нижнем регистре. Апострофы и дефисы внутри слова остаются
// (don't, well-known), по краям слова отбрасываются.
func Tokens(text string) []string {
	out := CasedTokens(text)
	for i, t := range out {
		out[i] = strings.ToLower(t)
	}
	return out
}

// CasedTokens — те же слова, что Tokens, но в исходном регистре: по заглавной букве не в начале предложения
// грамматика отличает имена и месяцы (May, Will).
func CasedTokens(text string) []string {
	text = strings.ReplaceAll(text, "’", "'")
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' && r != '-'
	})
	out := fields[:0]
	for _, f := range fields {
		if f = strings.Trim(f, "'-"); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// SentenceLevel — высший уровень среди слов предложения. Слово без уровня допустимо, только если оно среди первых
// MaxFreeRank по частоте. Слова с апострофом (don't, tom's) нейтральны: в списках их нет, они ни дают уровень,
// ни отбраковывают. Если уровня нет ни у одного слова, предложение не берётся (false).
func SentenceLevel(tokens []string, known map[string]WordInfo) (string, bool) {
	best := ""
	for _, t := range tokens {
		if strings.Contains(t, "'") {
			continue
		}
		k := known[t]
		switch {
		case k.Level != "":
			if k.Level > best { // «A1» < … < «C2» и по алфавиту
				best = k.Level
			}
		case k.Rank > 0 && k.Rank <= MaxFreeRank:
		default:
			return "", false
		}
	}
	return best, best != ""
}

// PastSimpleTarget — первое слово, давшее тему past_simple (с тем же контекстом, что в grammar.Topics), в нижнем регистре. Нет — "".
func PastSimpleTarget(tokens []string) string {
	for i := range tokens {
		if slices.Contains(grammar.Topics(tokens[:i+1]), grammar.PastSimple) {
			return strings.ToLower(tokens[i])
		}
	}
	return ""
}

// BaseCandidates — возможные начальные формы правильного глагола на -ed: walked → walk, walke; stopped → stop;
// studied → study. Какая из них настоящая, показывает словарь. Для неправильных глаголов и прочих слов — nil.
func BaseCandidates(past string) []string {
	if _, ok := words.IrregularLemma(past); ok || len(past) <= 4 {
		return nil
	}
	switch {
	case strings.HasSuffix(past, "ied"):
		return []string{past[:len(past)-3] + "y"}
	case strings.HasSuffix(past, "ed"):
		b := past[:len(past)-2]
		out := []string{b, b + "e"}
		if n := len(b); n > 2 && b[n-1] == b[n-2] { // stopped → stop
			out = append(out, b[:n-1])
		}
		return out
	}
	return nil
}

// PastDistractors — другие формы глагола, у которого форма прошедшего времени target. Начальная форма берётся из
// таблицы неправильных глаголов или это единственный из BaseCandidates, известный словарю. При неоднозначности
// (hoped: hop и hope) или неизвестной основе — nil: предложение лучше пропустить, чем ошибиться.
func PastDistractors(target string, known map[string]WordInfo) []string {
	base, ok := words.IrregularLemma(target)
	if !ok {
		var found []string
		for _, c := range BaseCandidates(target) {
			if k := known[c]; k.Level != "" || k.Rank > 0 {
				found = append(found, c)
			}
		}
		if len(found) != 1 {
			return nil
		}
		base = found[0]
	}
	var out []string
	for _, f := range words.Forms(base, "verb") {
		if len(f) == 1 && f[0] != target {
			out = append(out, f[0])
		}
	}
	return out
}
