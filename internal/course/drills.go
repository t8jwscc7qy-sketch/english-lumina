package course

import (
	"hash/fnv"
	"math/rand/v2"
	"regexp"
	"strings"
)

// Виды заданий закрепления.
const (
	KindGap   = "gap"   // выбрать слово в пропуск
	KindOrder = "order" // собрать предложение из плиток
	KindMatch = "match" // пары «слово — перевод»
)

// Сколько заданий каждого вида в уроке.
const (
	maxOrder    = 2
	maxOldWords = 3
	maxPairs    = 5
	maxDrills   = 10
	maxNewGaps  = 5
	gapOptions  = 3
)

// Pair — пара для сопоставления.
type Pair struct {
	En string `json:"en"`
	Ru string `json:"ru"`
}

// Drill — одно задание. Ответ отдаётся вместе с заданием: проверка идёт в браузере (личный сервис, урок — тренировка).
type Drill struct {
	Kind   string `json:"kind"`
	Old    bool   `json:"old"`              // слово из прошлых уроков
	Prompt string `json:"prompt,omitempty"` // перевод предложения
	// Gap: Text — предложение с ___ на месте слова, Options — варианты, Answer — верный.
	// Order: Tiles — перемешанные слова, Answer — слова в верном порядке через пробел.
	Text    string   `json:"text,omitempty"`
	Options []string `json:"options,omitempty"`
	Tiles   []string `json:"tiles,omitempty"`
	Answer  string   `json:"answer,omitempty"`
	Pairs   []Pair   `json:"pairs,omitempty"`
}

var sentenceEnd = regexp.MustCompile(`[^.!?]+[.!?]*`)

// splitSentences делит строку на предложения по . ! ?
func splitSentences(s string) []string {
	var out []string
	for _, m := range sentenceEnd.FindAllString(s, -1) {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func seed(id string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	v := h.Sum64()
	return rand.New(rand.NewPCG(v, v^0x9e3779b97f4a7c15))
}

// wordRef — слово курса и номер урока, где оно введено.
type wordRef struct {
	Word
	lesson int
}

// Drills собирает задания урока index (номер в общем порядке курса). Чистая функция: тот же урок даёт те же задания.
// Новые слова урока — пропуски в его же тексте; со второго урока добавляются до трёх слов из прошлых уроков (пропуск
// в тексте этого урока, а если там слова нет — в тексте урока, где оно введено); плитки — короткие предложения урока;
// пары — новые и повторяемые слова.
func Drills(c Course, index int) []Drill {
	all := c.Ordered()
	if index < 0 || index >= len(all) {
		return nil
	}
	l := all[index]
	rng := seed(l.ID)
	var pool []wordRef // все слова до этого урока включительно: источник отвлекающих вариантов
	for i := 0; i <= index; i++ {
		for _, w := range all[i].Words {
			pool = append(pool, wordRef{w, i})
		}
	}
	used := map[int]bool{} // строки урока, уже занятые пропуском
	var fresh []Drill
	for _, w := range l.Words {
		if len(fresh) == maxNewGaps {
			break
		}
		if d, ok := gapFor(l, w, pool, rng, used); ok {
			fresh = append(fresh, d)
		}
	}
	old := oldWords(all, index)
	var repeat []Drill
	for _, w := range old {
		d, ok := gapFor(l, w.Word, pool, rng, used)
		if !ok {
			d, ok = gapFor(all[w.lesson], w.Word, pool, rng, map[int]bool{})
		}
		if ok {
			d.Old = true
			repeat = append(repeat, d)
		}
	}
	order := orderDrills(l, rng)
	if room := maxDrills - 1 - len(fresh) - len(repeat); len(order) > max(room, 1) {
		order = order[:max(room, 1)]
	}
	out := append(append(fresh, order...), repeat...)
	var pairs []Pair
	for _, w := range l.Words {
		pairs = append(pairs, Pair{En: w.Lemma, Ru: w.Ru})
	}
	for _, w := range old {
		pairs = append(pairs, Pair{En: w.Lemma, Ru: w.Ru})
	}
	if len(pairs) > maxPairs {
		pairs = pairs[:maxPairs]
	}
	if len(pairs) >= 3 {
		out = append(out, Drill{Kind: KindMatch, Old: len(old) > 0, Pairs: pairs})
	}
	return out
}

// oldWords — до трёх слов прошлых уроков: сначала из ближайшего прошлого урока, по одному с урока, по кругу.
func oldWords(all []Lesson, index int) []wordRef {
	var out []wordRef
	for round := 0; len(out) < maxOldWords; round++ {
		added := false
		for i := index - 1; i >= 0 && len(out) < maxOldWords; i-- {
			if round < len(all[i].Words) {
				out = append(out, wordRef{all[i].Words[round], i})
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out
}

// gapFor строит пропуск для слова w в тексте урока l: первая свободная строка, где слово встречается.
func gapFor(l Lesson, w Word, pool []wordRef, rng *rand.Rand, used map[int]bool) (Drill, bool) {
	forms := map[string]bool{}
	for _, f := range w.forms() {
		forms[f] = true
	}
	best := -1
	var tok string
	for i, s := range l.Sentences {
		for _, t := range s.Tokens() {
			if forms[lower(t)] {
				if best < 0 || (used[best] && !used[i]) {
					best, tok = i, t
				}
				break
			}
		}
		if best >= 0 && !used[best] {
			break
		}
	}
	if best < 0 {
		return Drill{}, false
	}
	used[best] = true
	s := l.Sentences[best]
	text, ok := blank(s.En, tok)
	if !ok {
		return Drill{}, false
	}
	opts := append([]string{tok}, distractors(w, strings.HasPrefix(text, "___"), pool, rng)...)
	rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
	return Drill{Kind: KindGap, Prompt: s.Ru, Text: text, Options: opts, Answer: tok}, len(opts) >= 2
}

// blank заменяет первое вхождение слова tok (целым словом) на ___.
func blank(en, tok string) (string, bool) {
	re := regexp.MustCompile(`(^|[^\p{L}'])` + regexp.QuoteMeta(tok) + `($|[^\p{L}'])`)
	loc := re.FindStringSubmatchIndex(en)
	if loc == nil {
		return "", false
	}
	start, end := loc[3], loc[4] // конец первой группы и начало второй — границы слова
	return en[:start] + "___" + en[end:], true
}

// distractors — два неверных варианта: сначала слова той же части речи, потом любые; в начале предложения с заглавной.
func distractors(w Word, atStart bool, pool []wordRef, rng *rand.Rand) []string {
	own := map[string]bool{}
	for _, f := range w.forms() {
		own[f] = true
	}
	var same, other []string
	seen := map[string]bool{}
	for _, p := range pool {
		f := lower(p.Lemma)
		if own[f] || seen[f] {
			continue
		}
		seen[f] = true
		if p.Pos == w.Pos {
			same = append(same, p.Lemma)
		} else {
			other = append(other, p.Lemma)
		}
	}
	rng.Shuffle(len(same), func(i, j int) { same[i], same[j] = same[j], same[i] })
	rng.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
	cands := append(same, other...)
	if len(cands) > gapOptions-1 {
		cands = cands[:gapOptions-1]
	}
	for i, c := range cands {
		switch {
		case c == "I":
		case atStart:
			cands[i] = strings.ToUpper(c[:1]) + c[1:]
		default:
			cands[i] = strings.ToLower(c[:1]) + c[1:]
		}
	}
	return cands
}

// orderDrills — до двух заданий «собери предложение»: предложения урока из 3–6 слов, у которых есть пара в переводе.
func orderDrills(l Lesson, rng *rand.Rand) []Drill {
	var out []Drill
	for _, s := range l.Sentences {
		en, ru := splitSentences(s.En), splitSentences(s.Ru)
		if len(en) != len(ru) {
			continue
		}
		for i, e := range en {
			toks := Sentence{En: e}.Tokens()
			if len(toks) < 3 || len(toks) > 6 {
				continue
			}
			tiles := append([]string(nil), toks...)
			for range 5 {
				rng.Shuffle(len(tiles), func(a, b int) { tiles[a], tiles[b] = tiles[b], tiles[a] })
				if strings.Join(tiles, " ") != strings.Join(toks, " ") {
					break
				}
			}
			out = append(out, Drill{Kind: KindOrder, Prompt: ru[i], Tiles: tiles, Answer: strings.Join(toks, " ")})
			if len(out) == maxOrder {
				return out
			}
			break // одно предложение со строки: задания из разных мест текста
		}
	}
	return out
}

// Glossary — переводы всех форм слов, введённых до урока index включительно (для подсказки по нажатию на слово).
func Glossary(c Course, index int) map[string]string {
	out := map[string]string{}
	all := c.Ordered()
	for i := 0; i <= index && i < len(all); i++ {
		for _, w := range all[i].Words {
			for _, f := range w.forms() {
				if _, ok := out[f]; !ok {
					out[f] = w.Ru
				}
			}
		}
	}
	return out
}
