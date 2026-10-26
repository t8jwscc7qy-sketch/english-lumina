package kb

import (
	"slices"
	"testing"
)

func texts(in []Scored) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = s.Text
	}
	return out
}

func TestScoreTranslationsDropsArchaicWhenCommonExists(t *testing.T) {
	cands := []Candidate{
		{Text: "батю́шка", Source: "kaikki", SenseNo: 1, Tags: []string{"archaic"}},
		{Text: "па́па", Source: "kaikki", SenseNo: 1},
		{Text: "папа", Source: "freedict", SenseNo: 1},
		{Text: "бать", Source: "kaikki", SenseNo: 1, Tags: []string{"colloquial"}},
	}
	// у «батюшки» хорошее место в списке частот: без штрафа за archaic оно обогнало бы «бать»
	ranks := map[string]int{"папа": 274, "батюшка": 4000, "бать": 25000}
	got := ScoreTranslations(cands, ranks)
	if got[0].Text != "папа" || len(got[0].Sources) != 2 {
		t.Fatalf("первым ждали «папа» из двух источников: %+v", got[0])
	}
	last := got[len(got)-1]
	if last.Text != "батюшка" || !slices.Contains(last.Flags, "archaic") {
		t.Fatalf("последним ждали «батюшка» с пометкой archaic: %+v", last)
	}
}

func TestScoreTranslationsVulgarPenalty(t *testing.T) {
	cands := []Candidate{
		{Text: "соса́ть", Source: "kaikki", SenseNo: 1, Tags: []string{"vulgar", "slang"}},
		{Text: "дуть", Source: "kaikki", SenseNo: 2},
	}
	ranks := map[string]int{"сосать": 3000, "дуть": 4000}
	got := ScoreTranslations(cands, ranks)
	if got[0].Text != "дуть" {
		t.Fatalf("грубое значение не должно побеждать: %v", texts(got))
	}
	if !slices.Contains(got[1].Flags, "vulgar") {
		t.Fatalf("пометка vulgar потеряна: %+v", got[1])
	}
}

func TestScoreTranslationsAgreementBeatsSingleSource(t *testing.T) {
	cands := []Candidate{
		{Text: "ребёнок", Source: "kaikki", SenseNo: 1},
		{Text: "ребёнок", Source: "freedict", SenseNo: 1},
		{Text: "козлёнок", Source: "kaikki", SenseNo: 1},
	}
	got := ScoreTranslations(cands, map[string]int{"ребенок": 1297, "козленок": 1500})
	if got[0].Text != "ребёнок" || len(got[0].Sources) != 2 {
		t.Fatalf("согласие источников должно выигрывать: %v", texts(got))
	}
}

func TestScoreTranslationsFoldsYoInRankLookup(t *testing.T) {
	cands := []Candidate{
		{Text: "тёплый", Source: "kaikki", SenseNo: 1},
		{Text: "прохладный", Source: "kaikki", SenseNo: 1},
	}
	// в списке частот «ё» и «е» стоят отдельно: тёплый там как «теплый»
	got := ScoreTranslations(cands, map[string]int{"теплый": 16976})
	if got[0].Text != "тёплый" {
		t.Fatalf("место слова с «ё» ищется и под «е»: %v", texts(got))
	}
}

func TestScoreTranslationsRejectsLatinAndNonCyrillic(t *testing.T) {
	cands := []Candidate{
		{Text: "dad", Source: "kaikki"},
		{Text: "123", Source: "kaikki"},
		{Text: "чтo", Source: "kaikki"}, // кириллица с латинской «o»
		{Text: "папа", Source: "kaikki"},
		{Text: "", Source: "kaikki"},
	}
	got := ScoreTranslations(cands, nil)
	if !slices.Equal(texts(got), []string{"папа"}) {
		t.Fatalf("остаться должен только «папа»: %v", texts(got))
	}
}

func TestScoreTranslationsOnlyRareKeepsBest(t *testing.T) {
	got := ScoreTranslations([]Candidate{{Text: "батюшка", Source: "kaikki", Tags: []string{"archaic"}}}, map[string]int{"батюшка": 30608})
	if len(got) != 1 || got[0].Text != "батюшка" {
		t.Fatalf("слово без обычных переводов не остаётся пустым: %+v", got)
	}
}

func TestScoreTranslationsMultiwordPenalty(t *testing.T) {
	cands := []Candidate{
		{Text: "красная рыба", Source: "kaikki", SenseNo: 1},
		{Text: "лосось", Source: "kaikki", SenseNo: 1},
	}
	got := ScoreTranslations(cands, nil)
	if got[0].Text != "лосось" {
		t.Fatalf("однословный перевод предпочтительнее: %v", texts(got))
	}
}

func TestScoreTranslationsDeterministic(t *testing.T) {
	a := []Candidate{{Text: "ааа", Source: "kaikki"}, {Text: "ббб", Source: "kaikki"}, {Text: "ввв", Source: "kaikki"}}
	b := []Candidate{a[2], a[0], a[1]}
	got1, got2 := texts(ScoreTranslations(a, nil)), texts(ScoreTranslations(b, nil))
	if !slices.Equal(got1, got2) || !slices.Equal(got1, []string{"ааа", "ббб", "ввв"}) {
		t.Fatalf("равные оценки идут по тексту независимо от входа: %v / %v", got1, got2)
	}
}

func TestPickTranslationsPrefersDifferentSenses(t *testing.T) {
	scored := []Scored{
		{Candidate: Candidate{Text: "ааа"}, KaikkiSense: 1, Score: 9},
		{Candidate: Candidate{Text: "ббб"}, KaikkiSense: 1, Score: 8},
		{Candidate: Candidate{Text: "ввв"}, KaikkiSense: 2, Score: 7},
	}
	if got := texts(PickTranslations(scored, 2)); !slices.Equal(got, []string{"ааа", "ввв"}) {
		t.Fatalf("второй перевод из другого значения: %v", got)
	}
	same := []Scored{scored[0], scored[1]}
	if got := texts(PickTranslations(same, 2)); !slices.Equal(got, []string{"ааа", "ббб"}) {
		t.Fatalf("все из одного значения: берём следующий по оценке: %v", got)
	}
	unknown := []Scored{{Candidate: Candidate{Text: "ааа"}, Score: 9}, {Candidate: Candidate{Text: "ббб"}, Score: 8}}
	if got := texts(PickTranslations(unknown, 2)); !slices.Equal(got, []string{"ааа", "ббб"}) {
		t.Fatalf("значение неизвестно: %v", got)
	}
	if got := PickTranslations(scored, 1); len(got) != 1 || got[0].Text != "ааа" {
		t.Fatalf("max 1: %v", texts(got))
	}
	if got := PickTranslations(nil, 2); len(got) != 0 {
		t.Fatalf("пусто: %v", got)
	}
}

func TestNormalizeRu(t *testing.T) {
	for in, want := range map[string]string{"Па́па ": "папа", "  оде́жда": "одежда", "тёплый": "тёплый", "Красная  рыба": "красная рыба"} {
		if got := NormalizeRu(in); got != want {
			t.Errorf("%q: %q, ждали %q", in, got, want)
		}
	}
}

func TestPickPronunciations(t *testing.T) {
	sounds := []Sound{
		{IPA: "/dæd/", Tags: []string{"US", "General-American"}},
		{IPA: "/dad/", Tags: []string{"UK", "Received-Pronunciation"}},
		{Audio: "En-us-dad.ogg"},
		{IPA: ""},
	}
	got := PickPronunciations(sounds)
	if len(got) != 2 || got[0] != (Pron{Accent: "us", IPA: "dæd", Audio: "En-us-dad.ogg", Primary: true}) || got[1] != (Pron{Accent: "uk", IPA: "dad"}) {
		t.Fatalf("произношения: %+v", got)
	}
}

func TestPickPronunciationsUKOnlyAndUntagged(t *testing.T) {
	if got := PickPronunciations([]Sound{{IPA: "[kləʊðz]", Tags: []string{"UK"}}}); len(got) != 1 || got[0].Accent != "uk" || !got[0].Primary || got[0].IPA != "kləʊðz" {
		t.Fatalf("только британское — основное: %+v", got)
	}
	got := PickPronunciations([]Sound{{IPA: "/θɪn/"}})
	if len(got) != 1 || got[0].Accent != "" || !got[0].Primary || got[0].IPA != "θɪn" {
		t.Fatalf("без пометки акцента: %+v", got)
	}
	if got := PickPronunciations(nil); len(got) != 0 {
		t.Fatalf("пусто: %+v", got)
	}
}

func TestPickPronunciationsSameIPAKeepsOne(t *testing.T) {
	got := PickPronunciations([]Sound{{IPA: "/kæt/", Tags: []string{"US"}}, {IPA: "/kæt/", Tags: []string{"UK"}}})
	if len(got) != 1 || got[0].Accent != "us" {
		t.Fatalf("одинаковое IPA не дублируется: %+v", got)
	}
}

func TestNormalizeLemmaVariants(t *testing.T) {
	for in, want := range map[string][]string{
		"colour/color": {"colour", "color"},
		"a.m.":         {"a.m."},
		" a / b /a ":   {"a", "b"},
		"":             nil,
		"//":           nil,
		"ice cream":    {"ice cream"},
	} {
		if got := NormalizeLemmaVariants(in); !slices.Equal(got, want) {
			t.Errorf("%q: %v, ждали %v", in, got, want)
		}
	}
}

func TestGoldenListWellFormed(t *testing.T) {
	if len(Golden) != 40 {
		t.Fatalf("в контрольном списке %d слов, ждали 40", len(Golden))
	}
	seen := map[string]bool{}
	for _, g := range Golden {
		key := g.Lemma + "/" + g.Pos
		if seen[key] {
			t.Errorf("повтор %s", key)
		}
		seen[key] = true
		if g.Lemma == "" || g.Pos == "" || len(g.Accept) == 0 {
			t.Errorf("неполная запись %+v", g)
		}
		for _, a := range g.Accept {
			if a != NormalizeRu(a) {
				t.Errorf("%s: допустимый перевод %q не нормализован (%q)", key, a, NormalizeRu(a))
			}
		}
	}
	for _, must := range []string{"dad/noun", "kid/noun", "thin/adjective", "clothes/noun", "blow/verb"} {
		if !seen[must] {
			t.Errorf("в списке нет проблемного слова %s", must)
		}
	}
}

func TestPickTranslationsSkipsBadSecond(t *testing.T) {
	scored := []Scored{
		{Candidate: Candidate{Text: "дуть"}, KaikkiSense: 1, Score: 8},
		{Candidate: Candidate{Text: "соса́ть"}, KaikkiSense: 2, Score: 7, Flags: []string{"vulgar"}},
		{Candidate: Candidate{Text: "батюшка"}, KaikkiSense: 3, Score: 6, Flags: []string{"archaic"}},
		{Candidate: Candidate{Text: "веять"}, KaikkiSense: 4, Score: 2, Flags: []string{"slang"}},
	}
	got := texts(PickTranslations(scored, 2))
	if !slices.Equal(got, []string{"дуть", "веять"}) {
		t.Fatalf("грубое и устаревшее вторым не берём: %v", got)
	}
	bad := []Scored{{Candidate: Candidate{Text: "батюшка"}, Score: 1, Flags: []string{"archaic"}}}
	if got := texts(PickTranslations(bad, 2)); !slices.Equal(got, []string{"батюшка"}) {
		t.Fatalf("единственный (первый) перевод берётся даже с пометкой: %v", got)
	}
}

func TestPickTranslationsExtraNeedsMinScore(t *testing.T) {
	weak := []Scored{{Candidate: Candidate{Text: "ааа"}, Score: 9}, {Candidate: Candidate{Text: "ббб"}, Score: 1.5}}
	if got := texts(PickTranslations(weak, 2)); !slices.Equal(got, []string{"ааа"}) {
		t.Fatalf("слабый добавочный перевод не берётся: %v", got)
	}
	ok := []Scored{{Candidate: Candidate{Text: "ааа"}, Score: 9}, {Candidate: Candidate{Text: "ббб"}, Score: 2}}
	if got := texts(PickTranslations(ok, 2)); !slices.Equal(got, []string{"ааа", "ббб"}) {
		t.Fatalf("оценка 2 проходит: %v", got)
	}
	if got := texts(PickTranslations([]Scored{{Candidate: Candidate{Text: "ааа"}, Score: -3}}, 2)); !slices.Equal(got, []string{"ааа"}) {
		t.Fatalf("первый перевод берётся при любой оценке: %v", got)
	}
}

func TestScoreTranslationsKaikkiSenseBeatsLegacySense(t *testing.T) {
	// у «редкий» в старом словаре номер значения 1, у Kaikki — 2; у «тонкий» в Kaikki — 1: значение 1 решает по Kaikki
	cands := []Candidate{
		{Text: "тонкий", Source: "kaikki", SenseNo: 1},
		{Text: "редкий", Source: "kaikki", SenseNo: 2, Tags: []string{"rare"}},
		{Text: "редкий", Source: "freedict", SenseNo: 1},
	}
	got := ScoreTranslations(cands, map[string]int{"тонкий": 16560, "редкий": 12922})
	if got[0].Text != "тонкий" {
		t.Fatalf("номер значения от Kaikki важнее номера старого словаря: %v", texts(got))
	}
}

func TestScoreTranslationsPrefersFirstGroupAndPosition(t *testing.T) {
	// как в Викисловаре: основное значение — первая группа, внутри группы первое слово самое обычное
	cands := []Candidate{
		{Text: "ночь", Source: "kaikki", GroupNo: 2, GroupPos: 1},
		{Text: "ночь", Source: "freedict", SenseNo: 1},
		{Text: "утро", Source: "kaikki", GroupNo: 1, GroupPos: 1},
		{Text: "утро", Source: "freedict", SenseNo: 1},
	}
	got := ScoreTranslations(cands, map[string]int{"ночь": 400, "утро": 600})
	if got[0].Text != "утро" {
		t.Fatalf("первая группа Викисловаря должна побеждать: %v", texts(got))
	}
	pos := ScoreTranslations([]Candidate{
		{Text: "ааа", Source: "kaikki", GroupNo: 1, GroupPos: 3},
		{Text: "ббб", Source: "kaikki", GroupNo: 1, GroupPos: 1},
	}, nil)
	if pos[0].Text != "ббб" {
		t.Fatalf("первое слово группы лучше третьего: %v", texts(pos))
	}
}

func TestScoreTranslationsRejectsMarkup(t *testing.T) {
	cands := []Candidate{
		{Text: "[[время]] до [[полдень|полудня]]", Source: "freedict"},
		{Text: "время (до полудня)", Source: "freedict"},
		{Text: "утро", Source: "freedict"},
	}
	if got := texts(ScoreTranslations(cands, nil)); !slices.Equal(got, []string{"утро"}) {
		t.Fatalf("вики-разметка и скобки отбрасываются: %v", got)
	}
}

func TestPickTranslationsNeverChoosesCoarseFirst(t *testing.T) {
	coarse := Scored{Candidate: Candidate{Text: "ааа"}, Score: 9, Flags: []string{"vulgar", "slang"}}
	plain := Scored{Candidate: Candidate{Text: "ббб"}, Score: 3}
	if got := texts(PickTranslations([]Scored{coarse, plain}, 2)); !slices.Equal(got, []string{"ббб"}) {
		t.Fatalf("грубый перевод не выбирается и первым: %v", got)
	}
	if got := PickTranslations([]Scored{coarse}, 2); len(got) != 0 {
		t.Fatalf("у слова только грубый перевод: не выбираем ничего (строка остаётся запасной): %v", texts(got))
	}
	old := Scored{Candidate: Candidate{Text: "ввв"}, Score: 1, Flags: []string{"archaic"}}
	if got := texts(PickTranslations([]Scored{old}, 2)); !slices.Equal(got, []string{"ввв"}) {
		t.Fatalf("устаревший единственный перевод берётся: %v", got)
	}
}

func TestPickTranslationsIgnoresLegacySenseNumbers(t *testing.T) {
	// SenseNo у кандидата без номера от Kaikki — номер старого словаря: для разнообразия значений он не годится
	scored := []Scored{
		{Candidate: Candidate{Text: "ааа", SenseNo: 1}, Score: 9},
		{Candidate: Candidate{Text: "ббб", SenseNo: 1}, Score: 8},
		{Candidate: Candidate{Text: "ввв", SenseNo: 2}, Score: 7},
	}
	if got := texts(PickTranslations(scored, 2)); !slices.Equal(got, []string{"ааа", "ббб"}) {
		t.Fatalf("номера старого словаря значений не различают: %v", got)
	}
}

func TestScoreTranslationsMergesYoVariants(t *testing.T) {
	cands := []Candidate{
		{Text: "сёрфинг", Source: "kaikki", GroupNo: 1, GroupPos: 1},
		{Text: "серфинг", Source: "freedict", SenseNo: 1},
	}
	got := ScoreTranslations(cands, nil)
	if len(got) != 1 || got[0].Text != "сёрфинг" || len(got[0].Sources) != 2 {
		t.Fatalf("«ё» и «е» — одно слово, показывается вариант с «ё», оба источника учтены: %+v", got)
	}
}

// Штраф за «грубое» проверяется отдельно от штрафа за «просторечное»: у слова только пометка vulgar.
func TestScoreTranslationsCoarsePenaltyAlone(t *testing.T) {
	cands := []Candidate{
		{Text: "соса́ть", Source: "kaikki", GroupNo: 1, GroupPos: 1, Tags: []string{"vulgar"}},
		{Text: "дуть", Source: "kaikki", GroupNo: 1, GroupPos: 2},
	}
	got := ScoreTranslations(cands, map[string]int{"сосать": 3000, "дуть": 4000})
	if got[0].Text != "дуть" {
		t.Fatalf("штраф за vulgar должен решать: %v", texts(got))
	}
}
