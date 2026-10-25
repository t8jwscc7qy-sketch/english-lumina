package exercises

import (
	"slices"
	"strings"
	"testing"
)

func TestPickTranslationMostSensesWins(t *testing.T) {
	got, ok := PickTranslation([][]string{{"банк"}, {"берег", "банк"}, {"банк", "ряд"}}, nil)
	if !ok || got != "банк" {
		t.Fatalf("ждали «банк», получили %q %v", got, ok)
	}
}

func TestPickTranslationTieGoesToEarlierSense(t *testing.T) {
	got, ok := PickTranslation([][]string{{"идти", "ходить"}, {"ходить", "идти"}}, nil)
	if !ok || got != "идти" {
		t.Fatalf("при равенстве выигрывает вариант, раньше стоящий в первом значении: %q %v", got, ok)
	}
}

func TestPickTranslationSingleSenseTakesFirstVariant(t *testing.T) {
	got, ok := PickTranslation([][]string{{"яблоко", "яблочко"}}, nil)
	if !ok || got != "яблоко" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestPickTranslationAmbiguousWhenNoConsensus(t *testing.T) {
	// несколько значений, и ни один вариант не встречается дважды
	if got, ok := PickTranslation([][]string{{"бежать"}, {"работать"}, {"управлять"}}, nil); ok {
		t.Fatalf("консенсуса нет, а получили %q", got)
	}
}

func TestPickTranslationNormalizes(t *testing.T) {
	got, ok := PickTranslation([][]string{{"Яблоко"}, {"я́блоко "}}, nil)
	if !ok || got != "яблоко" {
		t.Fatalf("регистр, ударение и пробелы не должны различать варианты: %q %v", got, ok)
	}
}

func TestPickTranslationSkipsPhrasesAndLong(t *testing.T) {
	got, ok := PickTranslation([][]string{{"в двух словах", "кратко"}, {"в двух словах", "кратко"}}, nil)
	if !ok || got != "кратко" {
		t.Fatalf("словосочетание не годится, берём однословный вариант: %q %v", got, ok)
	}
	if _, ok := PickTranslation([][]string{{strings.Repeat("я", 25)}}, nil); ok {
		t.Fatal("перевод длиннее 24 символов не годится")
	}
	if _, ok := PickTranslation([][]string{{"а;б"}}, nil); ok {
		t.Fatal("перевод с разделителем пар не годится")
	}
	if _, ok := PickTranslation([][]string{{"а=б"}}, nil); ok {
		t.Fatal("перевод со знаком равенства не годится")
	}
}

func TestPickTranslationSkipsTooShort(t *testing.T) {
	// «ма», «па»: сокращения из словаря для плитки не годятся; нужно не меньше трёх букв
	if got, ok := PickTranslation([][]string{{"ма"}}, nil); ok {
		t.Fatalf("двухбуквенный перевод не годится: %q", got)
	}
	if got, ok := PickTranslation([][]string{{"дом"}}, nil); !ok || got != "дом" {
		t.Fatalf("трёхбуквенный годится: %q %v", got, ok)
	}
}

func TestPickTranslationEmpty(t *testing.T) {
	for _, in := range [][][]string{nil, {}, {{}}, {{""}, {"  "}}, {{"hello"}}} {
		if got, ok := PickTranslation(in, nil); ok {
			t.Fatalf("%v: получили %q", in, got)
		}
	}
}

func cand(word string, ru ...string) MatchCandidate {
	return MatchCandidate{Word: word, Senses: [][]string{ru}}
}

func fiveCands() []MatchCandidate {
	return []MatchCandidate{
		cand("apple", "яблоко"), cand("house", "дом"), cand("water", "вода"),
		cand("book", "книга"), cand("dog", "собака"), cand("cat", "кошка"),
	}
}

func TestBuildMatchTakesFirstFiveSuitable(t *testing.T) {
	m, ok := BuildMatch(fiveCands(), nil, 1)
	if !ok || len(m.Words) != MatchPairs || len(m.Options) != MatchPairs || len(m.Answers) != MatchPairs {
		t.Fatalf("%+v %v", m, ok)
	}
	if !slices.Equal(m.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("слова идут в порядке кандидатов: %v", m.Words)
	}
	for i, p := range m.Answers {
		if p.Word != m.Words[i] || !slices.Contains(m.Options, p.Ru) {
			t.Fatalf("пара %d: %+v, варианты %v", i, p, m.Options)
		}
	}
}

func TestBuildMatchSkipsUnsuitableWords(t *testing.T) {
	cands := []MatchCandidate{
		cand("ice cream", "мороженое"),           // с пробелом
		cand("a", "а"),                           // короче 2 букв
		cand(strings.Repeat("a", 21), "длинное"), // длиннее 20
		cand("x1", "икс"),                        // цифра
		{Word: "run", Senses: [][]string{{"бежать"}, {"работать"}, {"управлять"}}}, // неоднозначно
		{Word: "nothing"}, // нет значений
		cand("apple", "яблоко"), cand("house", "дом"), cand("water", "вода"), cand("book", "книга"), cand("dog", "собака"),
	}
	m, ok := BuildMatch(cands, nil, 3)
	if !ok || !slices.Equal(m.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("%+v %v", m, ok)
	}
}

func TestBuildMatchNoSharedTranslations(t *testing.T) {
	cands := []MatchCandidate{
		cand("big", "большой"), cand("large", "большой"), // синонимы
		cand("house", "дом"), cand("water", "вода"), cand("book", "книга"), cand("dog", "собака"),
	}
	m, ok := BuildMatch(cands, nil, 1)
	if !ok || slices.Contains(m.Words, "large") || !slices.Contains(m.Words, "big") {
		t.Fatalf("второе слово с тем же переводом пропускается: %v", m.Words)
	}
}

func TestBuildMatchNoDuplicateWords(t *testing.T) {
	cands := append([]MatchCandidate{cand("run", "бег")}, cand("run", "бегать"))
	cands = append(cands, cand("house", "дом"), cand("water", "вода"), cand("book", "книга"), cand("dog", "собака"))
	m, ok := BuildMatch(cands, nil, 1)
	if !ok || strings.Count(strings.Join(m.Words, ","), "run") != 1 {
		t.Fatalf("слово не должно повторяться: %v", m.Words)
	}
}

func TestBuildMatchNeedsFivePairs(t *testing.T) {
	if m, ok := BuildMatch(fiveCands()[:4], nil, 1); ok {
		t.Fatalf("4 слова — мало: %+v", m)
	}
	if _, ok := BuildMatch(nil, nil, 1); ok {
		t.Fatal("пусто")
	}
}

func TestBuildMatchSeedControlsOrderOfOptions(t *testing.T) {
	a, _ := BuildMatch(fiveCands(), nil, 1)
	b, _ := BuildMatch(fiveCands(), nil, 1)
	if !slices.Equal(a.Options, b.Options) {
		t.Fatalf("одинаковый seed — одинаковый порядок: %v %v", a.Options, b.Options)
	}
	different := false
	for seed := int64(2); seed < 40; seed++ {
		c, _ := BuildMatch(fiveCands(), nil, seed)
		if !slices.Equal(a.Options, c.Options) {
			different = true
		}
		got, want := slices.Clone(c.Options), slices.Clone(a.Options)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("варианты — те же переводы: %v %v", c.Options, a.Options)
		}
	}
	if !different {
		t.Fatal("разные seed должны давать разный порядок")
	}
}

func TestBuildMatchAnswersAreNotInWordOrderOfOptions(t *testing.T) {
	// хотя бы для одного seed варианты не совпадают с порядком слов (иначе упражнение не перемешано)
	for seed := int64(1); seed < 40; seed++ {
		m, _ := BuildMatch(fiveCands(), nil, seed)
		var inOrder []string
		for _, p := range m.Answers {
			inOrder = append(inOrder, p.Ru)
		}
		if !slices.Equal(inOrder, m.Options) {
			return
		}
	}
	t.Fatal("варианты никогда не перемешиваются")
}

var matchAnswers = []MatchPair{
	{"apple", "яблоко"}, {"house", "дом"}, {"water", "вода"}, {"book", "книга"}, {"dog", "собака"},
}

func TestCheckMatchAllCorrect(t *testing.T) {
	res, n, err := CheckMatch(matchAnswers, "apple=яблоко;house=дом;water=вода;book=книга;dog=собака")
	if err != nil || n != 5 || len(res) != 5 {
		t.Fatalf("%v %d %v", res, n, err)
	}
	for i, r := range res {
		if !r.Correct || r.Word != matchAnswers[i].Word || r.Answer != matchAnswers[i].Ru || r.Given != matchAnswers[i].Ru {
			t.Fatalf("%d: %+v", i, r)
		}
	}
}

func TestCheckMatchPartial(t *testing.T) {
	// собака и книга перепутаны; порядок пар в ответе не важен
	res, n, err := CheckMatch(matchAnswers, "dog=книга;book=собака;water=вода;house=дом;apple=яблоко")
	if err != nil || n != 3 {
		t.Fatalf("%d %v", n, err)
	}
	if res[0].Word != "apple" || !res[0].Correct || res[3].Word != "book" || res[3].Correct || res[3].Given != "собака" || res[3].Answer != "книга" {
		t.Fatalf("%+v", res)
	}
	if res[4].Word != "dog" || res[4].Correct || res[4].Given != "книга" {
		t.Fatalf("%+v", res[4])
	}
}

func TestCheckMatchIgnoresCaseSpacesAndStress(t *testing.T) {
	res, n, err := CheckMatch(matchAnswers, " Apple = Яблоко ;house=до́м; water=вода;book=книга ;dog=собака;")
	if err != nil || n != 5 {
		t.Fatalf("%d %v %+v", n, err, res)
	}
}

func TestCheckMatchErrors(t *testing.T) {
	cases := map[string]string{
		"":             "все",
		"apple=яблоко": "все",
		"apple=яблоко;house=дом;water=вода;book=книга":                      "все",
		"apple=яблоко;house=дом;water=вода;book=книга;dog=собака;cat=кошка": "все",
		"apple=яблоко;house=дом;water=вода;book=книга;собака":               "разобрать",
		"apple=яблоко;apple=дом;water=вода;book=книга;dog=собака":           "дважды",
		"apple=яблоко;house=яблоко;water=вода;book=книга;dog=собака":        "дважды",
		"apple=яблоко;house=дом;water=вода;book=книга;cat=собака":           "не из этого",
		"apple=яблоко;house=дом;water=вода;book=книга;dog=кошка":            "не из этого",
	}
	for given, want := range cases {
		_, _, err := CheckMatch(matchAnswers, given)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: ждали ошибку с %q, получили %v", given, want, err)
		}
	}
}

func TestPickTranslationRankDropsRareVariant(t *testing.T) {
	// «батюшка» встречается так же часто, но стоит за порогом списка частот
	senses := [][]string{{"батюшка", "папа"}, {"батюшка", "папа"}}
	got, ok := PickTranslation(senses, map[string]int{"папа": 274, "батюшка": 30608})
	if !ok || got != "папа" {
		t.Fatalf("ждали «папа», получили %q %v", got, ok)
	}
}

func TestPickTranslationDropsUnrankedVariant(t *testing.T) {
	got, ok := PickTranslation([][]string{{"одёжа", "одежда"}}, map[string]int{"одежда": 4303})
	if !ok || got != "одежда" {
		t.Fatalf("вариант без места отбрасывается: %q %v", got, ok)
	}
}

func TestPickTranslationAllRareKeepsWord(t *testing.T) {
	// частых вариантов нет: слово не теряем, выбор как без частот (книжная лексика в субтитрах редкая)
	got, ok := PickTranslation([][]string{{"батюшка"}}, map[string]int{"батюшка": 30608, "папа": 274})
	if !ok || got != "батюшка" {
		t.Fatalf("ждали запасной «батюшка», получили %q %v", got, ok)
	}
}

func TestPickTranslationUnrankedWordKept(t *testing.T) {
	got, ok := PickTranslation([][]string{{"туризм"}}, map[string]int{"папа": 274})
	if !ok || got != "туризм" {
		t.Fatalf("слово без места не теряется: %q %v", got, ok)
	}
}

func TestPickTranslationEmptyRanksKeepsOldBehaviour(t *testing.T) {
	senses := [][]string{{"идти", "ходить"}, {"ходить", "идти"}}
	for _, ranks := range []map[string]int{nil, {}} {
		if got, ok := PickTranslation(senses, ranks); !ok || got != "идти" {
			t.Fatalf("ranks=%v: прежний выбор «идти», получили %q %v", ranks, got, ok)
		}
	}
}

func TestPickTranslationWeakFilteredChoiceFallsBackToOldRule(t *testing.T) {
	// после отсева лучший вариант встречается в одном значении из двух: частоты ничего не доказывают,
	// решает прежнее правило (здесь «юбка» стоит за порогом, но встречается в обоих значениях)
	senses := [][]string{{"юбка", "подол"}, {"юбка", "край"}}
	ranks := map[string]int{"подол": 500, "край": 300, "юбка": 30000}
	got, ok := PickTranslation(senses, ranks)
	want, wantOK := PickTranslation(senses, nil)
	if ok != wantOK || got != want || got != "юбка" {
		t.Fatalf("как без частот («юбка»), получили %q %v", got, ok)
	}
}

func TestRuCandidates(t *testing.T) {
	got := RuCandidates([][]string{{"Папа", "па́па", "в двух словах"}, {"Тонкий", "папа"}})
	want := []string{"папа", "тонкий"}
	if !slices.Equal(got, want) {
		t.Fatalf("ждали %v, получили %v", want, got)
	}
}

func TestBuildMatchUsesRanks(t *testing.T) {
	cands := fiveCands()
	cands[0].Senses = [][]string{{"батюшка", "папа"}, {"батюшка", "папа"}}
	ranks := map[string]int{"папа": 274, "батюшка": 30608}
	for _, c := range cands[1:] {
		for _, s := range c.Senses {
			for _, ru := range s {
				ranks[ru] = 100
			}
		}
	}
	m, ok := BuildMatch(cands, ranks, 1)
	if !ok {
		t.Fatal("набор не собрался")
	}
	if m.Answers[0].Ru != "папа" {
		t.Fatalf("ждали «папа», получили %q", m.Answers[0].Ru)
	}
}

func TestPickTranslationRareVariantInMoreSensesLoses(t *testing.T) {
	// «батюшка» в трёх значениях, «папа» в двух: редкий вариант не побеждает числом значений
	senses := [][]string{{"батюшка", "папа"}, {"батюшка", "папа"}, {"батюшка"}}
	got, ok := PickTranslation(senses, map[string]int{"папа": 274, "батюшка": 30608})
	if !ok || got != "папа" {
		t.Fatalf("ждали «папа», получили %q %v", got, ok)
	}
}

func TestPickTranslationRankBoundary(t *testing.T) {
	senses := [][]string{{"ааа", "ббб"}, {"ааа", "ббб"}}
	// «ааа» ровно на пороге — частый, «ббб» на одно место дальше — редкий и отбрасывается
	if got, ok := PickTranslation(senses, map[string]int{"ааа": maxRuRank, "ббб": maxRuRank + 1}); !ok || got != "ааа" {
		t.Fatalf("на пороге вариант частый: %q %v", got, ok)
	}
	// наоборот: порядок слов в словаре («ааа» раньше) не должен спасать вариант за порогом
	if got, ok := PickTranslation(senses, map[string]int{"ааа": maxRuRank + 1, "ббб": maxRuRank}); !ok || got != "ббб" {
		t.Fatalf("за порогом вариант редкий: %q %v", got, ok)
	}
}

func TestPickTranslationOneCommonVariantInOneSenseKeepsAmbiguity(t *testing.T) {
	// частый вариант только в одном из значений: после отсева остальные значения пусты, но слово по-прежнему
	// неоднозначно (раньше такие слова отбрасывались) — принимать случайное значение нельзя
	senses := [][]string{{"сердечный"}, {"приятный"}, {"тёплый"}}
	if got, ok := PickTranslation(senses, map[string]int{"тёплый": 100}); ok {
		t.Fatalf("консенсуса нет, а получили %q", got)
	}
	// если неоднозначности не было и без частот, выбор остаётся прежним
	senses = [][]string{{"сердечный", "тёплый"}, {"тёплый"}}
	if got, ok := PickTranslation(senses, map[string]int{"тёплый": 100}); !ok || got != "тёплый" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestPickTranslationRanksDoNotReorderCommonVariants(t *testing.T) {
	// оба варианта частые и встречаются одинаково часто: место не решает (в субтитрах «оружие» стоит выше «руки»)
	senses := [][]string{{"оружие", "рука"}, {"оружие", "рука"}}
	got, ok := PickTranslation(senses, map[string]int{"оружие": 4000, "рука": 651})
	want, _ := PickTranslation(senses, nil)
	if !ok || got != want {
		t.Fatalf("как без частот (%q), получили %q %v", want, got, ok)
	}
}
