package texts

import (
	"math"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
)

// Sentence — предложение текста: номер в Tatoeba (для атрибуции), английский и русский тексты и слова в исходном регистре.
type Sentence struct {
	ID     int64
	En, Ru string
	Tokens []string
}

// Candidate — целевое слово и предложения, где оно есть. Forms — его формы в нижнем регистре (по ним в предложении
// узнают целевое слово).
type Candidate struct {
	Target    string
	Forms     map[string]bool
	Sentences []Sentence
}

// Options — параметры склейки; нули заменяются значениями по умолчанию.
type Options struct {
	Goal            float64 // целевое понимание текста (0,80–0,90)
	Tolerance       float64 // допустимое отклонение от цели (по умолчанию 0,05)
	MinSentences    int     // по умолчанию 6
	MaxSentences    int     // по умолчанию 10
	MinTargets      int     // по умолчанию 3
	MaxTargets      int     // по умолчанию 6
	PerTarget       int     // сколько раз целевое слово встречается (по умолчанию 2)
	MinWords        int     // слов в предложении (по умолчанию 4)
	MaxWords        int     // по умолчанию 12
	MaxExtraUnknown int     // неизвестных слов в предложении кроме целевого (по умолчанию 1)
	Seed            int64
}

func (o Options) withDefaults() Options {
	def := func(v *int, d int) {
		if *v == 0 {
			*v = d
		}
	}
	if o.Goal == 0 {
		o.Goal = 0.85
	}
	if o.Tolerance == 0 {
		o.Tolerance = 0.05
	}
	def(&o.MinSentences, 6)
	def(&o.MaxSentences, 10)
	def(&o.MinTargets, 3)
	def(&o.MaxTargets, 6)
	def(&o.PerTarget, 2)
	def(&o.MinWords, 4)
	def(&o.MaxWords, 12)
	if o.MaxExtraUnknown == 0 {
		o.MaxExtraUnknown = 1
	}
	return o
}

// Text — собранный текст: предложения в порядке чтения, целевые слова, настоящее понимание и связность (доля пар
// соседних предложений с общим содержательным словом).
type Text struct {
	Sentences []Sentence
	Targets   []string
	Coverage  float64
	Cohesion  float64
}

type usable struct {
	s     Sentence
	lower []string
	extra int // неизвестных слов, кроме форм целевого слова
}

// formAt — слово tokens[i] (в нижнем регистре lower) является формой целевого слова. Слово с заглавной буквы не в начале
// предложения — имя (Mark, Will, May), а не целевое слово.
func formAt(forms map[string]bool, tokens, lower []string, i int) bool {
	if !forms[lower[i]] {
		return false
	}
	r := []rune(tokens[i])
	return i == 0 || len(r) == 0 || !unicode.IsUpper(r[0])
}

// usableFor — предложения, подходящие целевому слову: длина по границам, слово есть, лишних неизвестных не больше
// допуска. Предложения с одинаковым номером берутся один раз.
func usableFor(c Candidate, known map[string]bool, o Options) []usable {
	var out []usable
	seen := map[int64]bool{}
	for _, s := range c.Sentences {
		if seen[s.ID] || len(s.Tokens) < o.MinWords || len(s.Tokens) > o.MaxWords {
			continue
		}
		lower := make([]string, len(s.Tokens))
		for i, t := range s.Tokens {
			lower[i] = strings.ToLower(t)
		}
		has := false
		for i := range s.Tokens {
			has = has || formAt(c.Forms, s.Tokens, lower, i)
		}
		if !has {
			continue
		}
		extra := 0
		for _, u := range exercises.UnknownTokens(s.Tokens, known) {
			if !c.Forms[u] {
				extra++
			}
		}
		if extra > o.MaxExtraUnknown {
			continue
		}
		seen[s.ID] = true
		out = append(out, usable{s: s, lower: lower, extra: extra})
	}
	return out
}

// content — содержательные слова предложения для связности: не служебные и не короче трёх букв.
func content(lower []string) map[string]bool {
	m := map[string]bool{}
	for _, w := range lower {
		if len([]rune(w)) >= 3 && !exercises.IsFunctionWord(w) {
			m[w] = true
		}
	}
	return m
}

func shares(a, b map[string]bool) bool {
	for w := range a {
		if b[w] {
			return true
		}
	}
	return false
}

// order расставляет предложения по связности: начинает с самого «целевого» (больше целевых слов), дальше берёт то,
// что делит содержательное слово с предыдущим (иначе — с любым из уже стоящих, иначе — с наименьшим числом лишних
// неизвестных).
func order(items []usable, hits map[int64]int) []usable {
	if len(items) == 0 {
		return nil
	}
	rest := slices.Clone(items)
	sort.SliceStable(rest, func(i, j int) bool {
		if hits[rest[i].s.ID] != hits[rest[j].s.ID] {
			return hits[rest[i].s.ID] > hits[rest[j].s.ID]
		}
		return rest[i].extra < rest[j].extra
	})
	out := []usable{rest[0]}
	rest = rest[1:]
	for len(rest) > 0 {
		prev := content(out[len(out)-1].lower)
		pick := -1
		for i, r := range rest { // сначала общее слово с предыдущим
			if shares(prev, content(r.lower)) {
				pick = i
				break
			}
		}
		if pick < 0 { // потом общее слово с любым из уже стоящих
			for i, r := range rest {
				c := content(r.lower)
				for _, o := range out {
					if shares(c, content(o.lower)) {
						pick = i
						break
					}
				}
				if pick >= 0 {
					break
				}
			}
		}
		if pick < 0 {
			pick = 0
		}
		out = append(out, rest[pick])
		rest = slices.Delete(rest, pick, pick+1)
	}
	return out
}

func cohesion(items []usable) float64 {
	if len(items) < 2 {
		return 0
	}
	n := 0
	for i := 1; i < len(items); i++ {
		if shares(content(items[i-1].lower), content(items[i].lower)) {
			n++
		}
	}
	return float64(n) / float64(len(items)-1)
}

type cand struct {
	c   Candidate
	use []usable
}

// build собирает текст из выбранных целевых слов: каждому по PerTarget предложений (меньше лишних неизвестных —
// раньше), добор до MinSentences, потолок MaxSentences, расстановка по связности.
func build(chosen []cand, known map[string]bool, o Options, rng *rand.Rand) Text {
	picked := map[int64]usable{}
	hits := map[int64]int{}
	count := map[string]int{}
	countHits := func(u usable) {
		for _, ch := range chosen {
			for i := range u.lower {
				if formAt(ch.c.Forms, u.s.Tokens, u.lower, i) {
					count[ch.c.Target]++
					hits[u.s.ID]++
					break
				}
			}
		}
	}
	for _, ch := range chosen {
		use := slices.Clone(ch.use)
		rng.Shuffle(len(use), func(i, j int) { use[i], use[j] = use[j], use[i] })
		sort.SliceStable(use, func(i, j int) bool { return use[i].extra < use[j].extra })
		for _, u := range use {
			if count[ch.c.Target] >= o.PerTarget || len(picked) >= o.MaxSentences {
				break
			}
			if _, dup := picked[u.s.ID]; dup {
				continue
			}
			picked[u.s.ID] = u
			countHits(u)
		}
	}
	// Добор до MinSentences: ещё предложения тех же слов, что связаны с уже выбранными.
	if len(picked) < o.MinSentences {
		var extras []usable
		for _, ch := range chosen {
			for _, u := range ch.use {
				if _, dup := picked[u.s.ID]; !dup {
					extras = append(extras, u)
				}
			}
		}
		rng.Shuffle(len(extras), func(i, j int) { extras[i], extras[j] = extras[j], extras[i] })
		sort.SliceStable(extras, func(i, j int) bool { return extras[i].extra < extras[j].extra })
		for _, u := range extras {
			if len(picked) >= o.MinSentences {
				break
			}
			if _, dup := picked[u.s.ID]; !dup {
				picked[u.s.ID] = u
				countHits(u)
			}
		}
	}
	items := make([]usable, 0, len(picked))
	for _, u := range picked {
		items = append(items, u)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].s.ID < items[j].s.ID }) // порядок не зависит от обхода карты
	ordered := order(items, hits)
	var t Text
	tokens := make([][]string, len(ordered))
	for i, u := range ordered {
		t.Sentences = append(t.Sentences, u.s)
		tokens[i] = u.s.Tokens
	}
	for _, ch := range chosen {
		if count[ch.c.Target] >= o.PerTarget { // слово без двух вхождений целевым не считается
			t.Targets = append(t.Targets, ch.c.Target)
		}
	}
	t.Coverage = Understanding(tokens, known)
	t.Cohesion = cohesion(ordered)
	return t
}

// Stitch собирает текст из предложений-кандидатов (кандидаты идут по убыванию частоты целевого слова). Берёт от
// MinTargets до MaxTargets целевых слов, у каждого не меньше PerTarget предложений, и подгоняет число целевых слов,
// пока понимание не попадёт в цель ± допуск (много неизвестного — убирает слово, слишком легко — добавляет). Если в
// допуск не вышло, возвращает ближайший вариант с честным Coverage. false — собрать нечего.
func Stitch(known map[string]bool, cands []Candidate, opts Options) (Text, bool) {
	o := opts.withDefaults()
	rng := rand.New(rand.NewSource(o.Seed)) //nolint:gosec // воспроизводимая случайность для подбора, не безопасность
	var ok []cand
	for _, c := range cands {
		if use := usableFor(c, known, o); len(use) >= o.PerTarget {
			ok = append(ok, cand{c: c, use: use})
		}
	}
	if len(ok) == 0 {
		return Text{}, false
	}
	// Окно самых частотных подходящих слов; внутри окна выбор случаен, но порядок по частоте сохраняется.
	maxN := min(o.MaxTargets, o.MaxSentences/o.PerTarget) // больше слов, чем можно показать дважды, не берём
	window := min(len(ok), 3*maxN)
	idx := rng.Perm(window)
	n := min((o.MinTargets+maxN)/2, window)
	var best Text
	bestGap := math.MaxFloat64
	seen := map[int]bool{}
	for iter := 0; iter < 8 && n >= 1 && n <= window && !seen[n]; iter++ {
		seen[n] = true
		sel := slices.Clone(idx[:n])
		sort.Ints(sel)
		chosen := make([]cand, n)
		for i, k := range sel {
			chosen[i] = ok[k]
		}
		t := build(chosen, known, o, rng)
		if len(t.Sentences) == 0 {
			break
		}
		if gap := math.Abs(t.Coverage - o.Goal); gap < bestGap {
			best, bestGap = t, gap
		}
		switch {
		case t.Coverage < o.Goal-o.Tolerance && n > min(o.MinTargets, window):
			n--
		case t.Coverage > o.Goal+o.Tolerance && n < min(maxN, window):
			n++
		default:
			iter = 8 // в допуске (или двигаться некуда)
		}
	}
	// Нижние границы: меньше MinSentences предложений или ни одного целевого слова — текст не отдаём.
	return best, len(best.Sentences) >= o.MinSentences && len(best.Targets) >= 1
}
