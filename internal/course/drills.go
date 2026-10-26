package course

import (
	"hash/fnv"
	"math/rand/v2"
	"regexp"
	"sort"
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
	// Сначала слова, которым подходит меньше предложений: иначе частое слово (is) займёт единственное предложение
	// редкого (friend). Порядок заданий потом возвращается к порядку слов урока.
	order := make([]int, len(l.Words))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return candidates(l, l.Words[order[a]]) < candidates(l, l.Words[order[b]]) })
	byWord := map[int]Drill{}
	for _, i := range order {
		if len(byWord) == maxNewGaps {
			break
		}
		if d, ok := gapFor(l, l.Words[i], pool, rng, used); ok {
			byWord[i] = d
		}
	}
	var fresh []Drill
	for i := range l.Words {
		if d, ok := byWord[i]; ok {
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
	tiles := orderDrills(l, rng)
	if room := maxDrills - 1 - len(fresh) - len(repeat); len(tiles) > max(room, 1) {
		tiles = tiles[:max(room, 1)]
	}
	out := append(append(fresh, tiles...), repeat...)
	var pairs []Pair
	seenEn, seenRu := map[string]bool{}, map[string]bool{}
	addPair := func(w Word) { // одинаковое слово или перевод дали бы две неразличимые плитки, и пары не собрать
		if len(pairs) == maxPairs || seenEn[lower(w.Lemma)] || seenRu[w.Ru] {
			return
		}
		seenEn[lower(w.Lemma)], seenRu[w.Ru] = true, true
		pairs = append(pairs, Pair{En: w.Lemma, Ru: w.Ru})
	}
	for _, w := range l.Words {
		addPair(w)
	}
	for _, w := range old {
		addPair(w.Word)
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

// candidates — в скольких предложениях урока встречается слово.
func candidates(l Lesson, w Word) int {
	forms := map[string]bool{}
	for _, f := range w.forms() {
		forms[f] = true
	}
	n := 0
	for _, s := range units(l) {
		for _, t := range s.Tokens() {
			if forms[lower(t)] {
				n++
				break
			}
		}
	}
	return n
}

// units — предложения урока по отдельности: реплика «Hello, Anna! How are you?» даёт два предложения, если перевод
// делится на столько же частей; иначе реплика остаётся целиком.
func units(l Lesson) []Sentence {
	var out []Sentence
	for _, s := range l.Sentences {
		en, ru := splitSentences(s.En), splitSentences(s.Ru)
		if len(en) < 2 || len(en) != len(ru) {
			out = append(out, s)
			continue
		}
		for i := range en {
			out = append(out, Sentence{Speaker: s.Speaker, En: en[i], Ru: ru[i]})
		}
	}
	return out
}

// gapFor строит пропуск для слова w: первое свободное предложение урока l, где слово встречается. Одно предложение —
// один пропуск: иначе второе задание подсказывало бы ответ первого.
func gapFor(l Lesson, w Word, pool []wordRef, rng *rand.Rand, used map[int]bool) (Drill, bool) {
	forms := map[string]bool{}
	for _, f := range w.forms() {
		forms[f] = true
	}
	for i, s := range units(l) {
		if used[i] {
			continue
		}
		for _, tok := range s.Tokens() {
			if !forms[lower(tok)] {
				continue
			}
			text, ok := blank(s.En, tok)
			if !ok {
				return Drill{}, false
			}
			used[i] = true
			opts := append([]string{tok}, distractors(w, capitalized(tok), pool, rng)...)
			rng.Shuffle(len(opts), func(a, b int) { opts[a], opts[b] = opts[b], opts[a] })
			return Drill{Kind: KindGap, Prompt: s.Ru, Text: text, Options: opts, Answer: tok}, len(opts) >= 2
		}
	}
	return Drill{}, false
}

// blank заменяет первое вхождение слова tok (целым словом, как режет токенизатор: апостроф и дефис — часть слова,
// прямой и типографский апостроф равны) на ___.
func blank(en, tok string) (string, bool) {
	word := strings.ReplaceAll(regexp.QuoteMeta(strings.ReplaceAll(tok, "’", "'")), "'", "['’]")
	re := regexp.MustCompile(`(^|[^\p{L}\p{N}'’-])(` + word + `)($|[^\p{L}\p{N}'’-])`)
	loc := re.FindStringSubmatchIndex(en)
	if loc == nil {
		return "", false
	}
	return en[:loc[4]] + "___" + en[loc[5]:], true
}

// capitalized — ответ пишется с заглавной (начало предложения), кроме местоимения I, которое заглавное всегда.
func capitalized(tok string) bool {
	return tok != "I" && tok != "" && strings.ToUpper(tok[:1]) == tok[:1] && strings.ToLower(tok[:1]) != tok[:1]
}

// distractors — два неверных варианта: сначала слова той же части речи, потом любые; регистр первой буквы как у
// ответа, иначе ответ угадывается по заглавной букве.
func distractors(w Word, capital bool, pool []wordRef, rng *rand.Rand) []string {
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
		case capital:
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
