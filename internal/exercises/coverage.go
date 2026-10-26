package exercises

import (
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
		that's there here very too also just all any some one each every more most much many`) {
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

// Coverage — доля известных слов среди учитываемых в предложении. tokens — слова в исходном регистре
// (CasedTokens). Не учитываются числа и слова с заглавной буквы не в начале предложения (имена); служебные слова
// считаются известными. Нет учитываемых слов — покрытие полное.
func Coverage(tokens []string, known map[string]bool) float64 {
	total, hit := 0, 0
	for i, t := range tokens {
		r := []rune(t)
		if len(r) == 0 || unicode.IsDigit(r[0]) || (i > 0 && unicode.IsUpper(r[0])) {
			continue
		}
		total++
		lower := strings.ToLower(t)
		if known[lower] || functionWords[lower] {
			hit++
		}
	}
	if total == 0 {
		return 1
	}
	return float64(hit) / float64(total)
}
