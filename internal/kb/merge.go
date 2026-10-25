package kb

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Оценка перевода-кандидата: стартовые константы, подбираются по контрольному списку (golden.go).
const (
	weightKaikki      = 3.0 // вес источника: Kaikki размечает значения и пометки, словарь FreeDict шумнее
	weightOther       = 1.0
	bonusAgreement    = 2.0 // перевод встречается минимум в двух источниках
	bonusFirstSense   = 1.0
	bonusRankTop      = 2.0 // русское слово среди первых 5000 по частоте
	bonusRankCommon   = 1.0 // среди первых 20000
	penaltyNoRank     = 1.0 // слова нет в первых 20000
	penaltyOld        = 5.0 // archaic, obsolete, dated
	penaltyCoarse     = 5.0 // vulgar, offensive, derogatory
	penaltySlangRare  = 2.0 // slang, rare, dialectal, regional
	penaltyMultiword  = 1.0
	rankTop           = 5000
	rankCommon        = 20000
	maxTranslationLen = 64
	minExtraScore     = 2.0 // добавочный (второй) перевод берётся только с оценкой не ниже
)

var (
	oldTags    = []string{"archaic", "obsolete", "dated", "historical"}
	coarseTags = []string{"vulgar", "offensive", "derogatory"}
	rareTags   = []string{"slang", "rare", "dialectal", "regional"}
)

// Candidate — перевод-кандидат из одного источника. SenseNo — номер значения слова (0, если неизвестен).
type Candidate struct {
	Text    string
	Source  string
	SenseNo int
	Tags    []string
	Sources []string // заполняется при слиянии одинаковых переводов
}

// Scored — кандидат с оценкой и пометками (флаги штрафуемых и информационных тегов).
type Scored struct {
	Candidate
	Score float64
	Flags []string
}

// NormalizeRu приводит русский текст к виду для сравнения и хранения: без знаков ударения, строчными, с одиночными пробелами.
func NormalizeRu(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '́' || r == '̀' {
			return -1
		}
		return r
	}, s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func foldYo(s string) string { return strings.ReplaceAll(s, "ё", "е") }

// validRu: перевод годится — кириллица без латиницы, не длиннее maxTranslationLen символов.
func validRu(s string) bool {
	if s == "" || utf8.RuneCountInString(s) > maxTranslationLen {
		return false
	}
	cyrillic := false
	for _, r := range s {
		switch {
		case r < 0x80 && unicode.IsLetter(r):
			return false
		case unicode.Is(unicode.Cyrillic, r):
			cyrillic = true
		}
	}
	return cyrillic
}

func sourceWeight(name string) float64 {
	if name == "kaikki" {
		return weightKaikki
	}
	return weightOther
}

func hasAny(tags []string, set []string) bool {
	return slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(set, strings.ToLower(t)) })
}

// bestRank — лучшее место русского слова в списке частот; «ё» и «е» в списке стоят отдельно, ищем обе записи.
func bestRank(text string, ranks map[string]int) (int, bool) {
	best, ok := 0, false
	for _, k := range []string{text, foldYo(text)} {
		if r, found := ranks[k]; found && (!ok || r < best) {
			best, ok = r, true
		}
	}
	return best, ok
}

type group struct {
	text    string
	sources map[string]bool
	tags    map[string]bool
	senseNo int // номер значения от Kaikki; нумерация старого словаря с ней несравнима
	otherNo int // номер значения от остальных источников (используется, только если у Kaikki номера нет)
}

// ScoreTranslations оценивает кандидатов: одинаковые переводы из разных источников склеиваются, негодные (латиница,
// не кириллица, пусто) отбрасываются, результат отсортирован по убыванию оценки, затем по тексту (детерминированно).
// ranks — места русских слов в списке частот. Порога нет: слово без обычных переводов остаётся с лучшим из плохих,
// пометки в Flags позволяют потребителю решить.
func ScoreTranslations(cands []Candidate, ranks map[string]int) []Scored {
	groups := map[string]*group{}
	for _, c := range cands {
		text := NormalizeRu(c.Text)
		if !validRu(text) {
			continue
		}
		g := groups[text]
		if g == nil {
			g = &group{text: text, sources: map[string]bool{}, tags: map[string]bool{}}
			groups[text] = g
		}
		g.sources[c.Source] = true
		for _, t := range c.Tags {
			g.tags[strings.ToLower(t)] = true
		}
		if c.SenseNo > 0 {
			if c.Source == "kaikki" {
				if g.senseNo == 0 || c.SenseNo < g.senseNo {
					g.senseNo = c.SenseNo
				}
			} else if g.otherNo == 0 || c.SenseNo < g.otherNo {
				g.otherNo = c.SenseNo
			}
		}
	}
	out := make([]Scored, 0, len(groups))
	for _, g := range groups {
		tags := make([]string, 0, len(g.tags))
		for t := range g.tags {
			tags = append(tags, t)
		}
		slices.Sort(tags)
		sources := make([]string, 0, len(g.sources))
		weight := 0.0
		for s := range g.sources {
			sources = append(sources, s)
			weight = max(weight, sourceWeight(s))
		}
		slices.Sort(sources)

		if g.senseNo == 0 {
			g.senseNo = g.otherNo
		}
		score := weight
		if len(sources) >= 2 {
			score += bonusAgreement
		}
		if g.senseNo == 1 {
			score += bonusFirstSense
		}
		switch r, ok := bestRank(g.text, ranks); {
		case ok && r <= rankTop:
			score += bonusRankTop
		case ok && r <= rankCommon:
			score += bonusRankCommon
		default:
			score -= penaltyNoRank
		}
		var flags []string
		if hasAny(tags, oldTags) {
			score -= penaltyOld
		}
		if hasAny(tags, coarseTags) {
			score -= penaltyCoarse
		}
		if hasAny(tags, rareTags) {
			score -= penaltySlangRare
		}
		for _, t := range tags {
			if slices.Contains(oldTags, t) || slices.Contains(coarseTags, t) || slices.Contains(rareTags, t) || t == "informal" {
				flags = append(flags, t)
			}
		}
		if strings.Contains(g.text, " ") {
			score -= penaltyMultiword
		}
		out = append(out, Scored{
			Candidate: Candidate{Text: g.text, Source: sources[0], SenseNo: g.senseNo, Tags: tags, Sources: sources},
			Score:     score, Flags: flags,
		})
	}
	slices.SortFunc(out, func(a, b Scored) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		}
		return strings.Compare(a.Text, b.Text)
	})
	return out
}

// PickTranslations выбирает до max переводов: первый — лучший, следующие по возможности из других значений
// (если значения известны), иначе по порядку оценок. Вторым и далее не берутся грубые и устаревшие переводы и переводы
// с оценкой ниже minExtraScore: единственный лучший перевод слова берётся даже с пометкой и низкой оценкой, добавочные нет.
func PickTranslations(scored []Scored, max int) []Scored {
	var picked []Scored
	used := make([]bool, len(scored))
	for len(picked) < max {
		next, fresh := -1, -1
		for i, s := range scored {
			if used[i] {
				continue
			}
			if len(picked) > 0 && (s.Score < minExtraScore || hasAny(s.Flags, oldTags) || hasAny(s.Flags, coarseTags)) {
				continue
			}
			if next < 0 {
				next = i
			}
			if fresh < 0 && s.SenseNo > 0 && !senseUsed(picked, s.SenseNo) {
				fresh = i
			}
		}
		if len(picked) > 0 && fresh >= 0 {
			next = fresh
		}
		if next < 0 {
			break
		}
		used[next] = true
		picked = append(picked, scored[next])
	}
	return picked
}

func senseUsed(picked []Scored, senseNo int) bool {
	return slices.ContainsFunc(picked, func(p Scored) bool { return p.SenseNo == senseNo })
}

// Pron — произношение слова: Accent us, uk или пусто, IPA без слэшей, Audio — имя файла Wikimedia Commons.
type Pron struct {
	Accent  string
	IPA     string
	Audio   string
	Primary bool
}

func accentOf(tags []string, audio string) string {
	for _, t := range tags {
		switch strings.ToLower(t) {
		case "us", "general-american", "genam", "american", "ga":
			return "us"
		case "uk", "received-pronunciation", "rp", "british", "gb":
			return "uk"
		}
	}
	a := strings.ToLower(audio)
	switch {
	case strings.Contains(a, "-us-"):
		return "us"
	case strings.Contains(a, "-uk-"), strings.Contains(a, "-gb-"):
		return "uk"
	}
	return ""
}

// PickPronunciations выбирает основное американское и (если отличается) британское произношение; без пометок акцента
// берётся первое. Записи без IPA пропускаются. Звук привязывается по акценту (метка или имя файла).
func PickPronunciations(sounds []Sound) []Pron {
	ipa := map[string]string{}
	audio := map[string]string{}
	for _, s := range sounds {
		acc := accentOf(s.Tags, s.Audio)
		if v := strings.Trim(strings.TrimSpace(s.IPA), "/[]"); v != "" {
			if ipa[acc] == "" {
				ipa[acc] = strings.TrimSpace(v)
			}
		}
		if s.Audio != "" && audio[acc] == "" {
			audio[acc] = s.Audio
		}
	}
	var out []Pron
	if ipa["us"] != "" {
		out = append(out, Pron{Accent: "us", IPA: ipa["us"], Audio: audio["us"]})
	}
	if ipa["uk"] != "" && ipa["uk"] != ipa["us"] {
		out = append(out, Pron{Accent: "uk", IPA: ipa["uk"], Audio: audio["uk"]})
	}
	if len(out) == 0 && ipa[""] != "" {
		out = append(out, Pron{IPA: ipa[""], Audio: audio[""]})
	}
	if len(out) > 0 {
		out[0].Primary = true
		if out[0].Audio == "" {
			out[0].Audio = audio[""]
		}
	}
	return out
}

// NormalizeLemmaVariants разбивает лемму с вариантами через «/» (colour/color) на отдельные слова без пустых и повторов.
func NormalizeLemmaVariants(lemma string) []string {
	var out []string
	for _, part := range strings.Split(lemma, "/") {
		part = strings.TrimSpace(part)
		if part != "" && !slices.Contains(out, part) {
			out = append(out, part)
		}
	}
	return out
}
