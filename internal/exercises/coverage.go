package exercises

import (
	"math"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// LemmaPos — слово пользователя (лемма и часть речи) для построения множества известных форм.
type LemmaPos struct {
	Lemma, Pos string
}

// functionWords — служебные слова: их не «изучают» на «Знакомстве» (в блок берутся только существительные, глаголы,
// прилагательные и наречия), поэтому для покрытия они считаются известными всегда.
var functionWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`a an the this that these those i me my mine you your yours he him his she her hers
		it its we us our ours they them their theirs who whom whose which what where when why how and or but if so
		because as than then of in on at to for from with by about into over under up down out off not no yes
		am is are was were be been being do does did done have has had having will would can could shall should may might must
		don't doesn't didn't isn't aren't wasn't weren't won't can't couldn't wouldn't shouldn't haven't hasn't hadn't
		i'm you're he's she's it's we're they're i've you've we've they've i'll you'll he'll she'll we'll they'll i'd you'd
		that's there here very too also just all any some one each every more most much many
		two three four five six seven eight nine ten first second something nothing anything everything someone everyone
		anyone nobody myself yourself himself herself itself ourselves themselves before after without through until while
		since both other another let's what's there's who's where's how's please against between during among within
		upon across behind beside beyond except onto toward towards whether though although unless once`) {
		m[w] = true
	}
	return m
}()

// KnownForms строит множество однословных форм известных слов в нижнем регистре.
func KnownForms(lemmas []LemmaPos) map[string]bool {
	known := make(map[string]bool, len(lemmas)*4)
	for _, l := range lemmas {
		for _, f := range words.Forms(l.Lemma, l.Pos) {
			if len(f) == 1 {
				known[f[0]] = true
			}
		}
	}
	return known
}

// isKnown — слово известно: служебное, среди форм изученных слов, без притяжательного окончания ('s, s') или
// составное через дефис, все части которого известны.
func isKnown(lower string, known map[string]bool) bool {
	if known[lower] || functionWords[lower] {
		return true
	}
	if base, ok := strings.CutSuffix(lower, "'s"); ok && base != "" {
		return isKnown(base, known)
	}
	if base, ok := strings.CutSuffix(lower, "s'"); ok && base != "" {
		return known[base] || known[base+"s"] || functionWords[base]
	}
	if parts := strings.Split(lower, "-"); len(parts) > 1 {
		for _, p := range parts {
			if p == "" || !isKnown(p, known) {
				return false
			}
		}
		return true
	}
	return false
}

// counts возвращает число учитываемых слов и сколько среди них неизвестных. Не учитываются числа и слова с заглавной
// буквы не в начале предложения (имена); служебные слова считаются известными.
func counts(tokens []string, known map[string]bool) (total, unknown int) {
	for i, t := range tokens {
		r := []rune(t)
		if len(r) == 0 || unicode.IsDigit(r[0]) || (i > 0 && unicode.IsUpper(r[0])) {
			continue
		}
		total++
		if !isKnown(strings.ToLower(t), known) {
			unknown++
		}
	}
	return total, unknown
}

// Coverage — доля известных слов среди учитываемых в предложении. tokens — слова в исходном регистре
// (CasedTokens). Нет учитываемых слов — покрытие полное.
func Coverage(tokens []string, known map[string]bool) float64 {
	total, unknown := counts(tokens, known)
	if total == 0 {
		return 1
	}
	return float64(total-unknown) / float64(total)
}

// minLenientWords — с этой длины (учитываемых слов) допускается одно неизвестное слово при любом пороге: предложения
// короткие (4–12 слов), и без этого порог 0,92–0,99 означал бы «все слова известны», а предложение с именем в начале
// (оно считается словом) отсекалось бы всегда.
const minLenientWords = 8

// CoverageOK — предложение проходит порог покрытия: неизвестных слов не больше floor((1 − порог) × n), но при
// n ≥ minLenientWords не меньше одного.
func CoverageOK(tokens []string, known map[string]bool, threshold float64) bool {
	total, unknown := counts(tokens, known)
	allowed := int(math.Floor((1-threshold)*float64(total) + 1e-9))
	if total >= minLenientWords {
		allowed = max(allowed, 1)
	}
	return unknown <= allowed
}

// IsFunctionWord — служебное слово (в нижнем регистре): считается известным всегда.
func IsFunctionWord(lower string) bool { return functionWords[lower] }

// CountKnown возвращает число учитываемых слов предложения и сколько среди них неизвестных (правила как у Coverage).
func CountKnown(tokens []string, known map[string]bool) (total, unknown int) {
	return counts(tokens, known)
}

// UnknownTokens — неизвестные слова предложения в нижнем регистре, в порядке появления.
func UnknownTokens(tokens []string, known map[string]bool) []string {
	var out []string
	for i, t := range tokens {
		r := []rune(t)
		if len(r) == 0 || unicode.IsDigit(r[0]) || (i > 0 && unicode.IsUpper(r[0])) {
			continue
		}
		if lower := strings.ToLower(t); !isKnown(lower, known) {
			out = append(out, lower)
		}
	}
	return out
}
