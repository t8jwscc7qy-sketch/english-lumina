package exercises

import (
	"slices"
	"strings"
	"testing"
)

func TestPickTranslationMostSensesWins(t *testing.T) {
	got, ok := PickTranslation([][]string{{"банк"}, {"берег", "банк"}, {"банк", "ряд"}})
	if !ok || got != "банк" {
		t.Fatalf("ждали «банк», получили %q %v", got, ok)
	}
}

func TestPickTranslationTieGoesToEarlierSense(t *testing.T) {
	got, ok := PickTranslation([][]string{{"идти", "ходить"}, {"ходить", "идти"}})
	if !ok || got != "идти" {
		t.Fatalf("при равенстве выигрывает вариант, раньше стоящий в первом значении: %q %v", got, ok)
	}
}

func TestPickTranslationSingleSenseTakesFirstVariant(t *testing.T) {
	got, ok := PickTranslation([][]string{{"яблоко", "яблочко"}})
	if !ok || got != "яблоко" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestPickTranslationAmbiguousWhenNoConsensus(t *testing.T) {
	// несколько значений, и ни один вариант не встречается дважды
	if got, ok := PickTranslation([][]string{{"бежать"}, {"работать"}, {"управлять"}}); ok {
		t.Fatalf("консенсуса нет, а получили %q", got)
	}
}

func TestPickTranslationNormalizes(t *testing.T) {
	got, ok := PickTranslation([][]string{{"Яблоко"}, {"я́блоко "}})
	if !ok || got != "яблоко" {
		t.Fatalf("регистр, ударение и пробелы не должны различать варианты: %q %v", got, ok)
	}
}

func TestPickTranslationSkipsPhrasesAndLong(t *testing.T) {
	got, ok := PickTranslation([][]string{{"в двух словах", "кратко"}, {"в двух словах", "кратко"}})
	if !ok || got != "кратко" {
		t.Fatalf("словосочетание не годится, берём однословный вариант: %q %v", got, ok)
	}
	if _, ok := PickTranslation([][]string{{strings.Repeat("я", 25)}}); ok {
		t.Fatal("перевод длиннее 24 символов не годится")
	}
	if _, ok := PickTranslation([][]string{{"а;б"}}); ok {
		t.Fatal("перевод с разделителем пар не годится")
	}
	if _, ok := PickTranslation([][]string{{"а=б"}}); ok {
		t.Fatal("перевод со знаком равенства не годится")
	}
}

func TestPickTranslationSkipsTooShort(t *testing.T) {
	// «ма», «па»: сокращения из словаря для плитки не годятся; нужно не меньше трёх букв
	if got, ok := PickTranslation([][]string{{"ма"}}); ok {
		t.Fatalf("двухбуквенный перевод не годится: %q", got)
	}
	if got, ok := PickTranslation([][]string{{"дом"}}); !ok || got != "дом" {
		t.Fatalf("трёхбуквенный годится: %q %v", got, ok)
	}
}

func TestPickTranslationEmpty(t *testing.T) {
	for _, in := range [][][]string{nil, {}, {{}}, {{""}, {"  "}}, {{"hello"}}} {
		if got, ok := PickTranslation(in); ok {
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
	m, ok := BuildMatch(fiveCands(), 1)
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
	m, ok := BuildMatch(cands, 3)
	if !ok || !slices.Equal(m.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("%+v %v", m, ok)
	}
}

func TestBuildMatchNoSharedTranslations(t *testing.T) {
	cands := []MatchCandidate{
		cand("big", "большой"), cand("large", "большой"), // синонимы
		cand("house", "дом"), cand("water", "вода"), cand("book", "книга"), cand("dog", "собака"),
	}
	m, ok := BuildMatch(cands, 1)
	if !ok || slices.Contains(m.Words, "large") || !slices.Contains(m.Words, "big") {
		t.Fatalf("второе слово с тем же переводом пропускается: %v", m.Words)
	}
}

func TestBuildMatchNoDuplicateWords(t *testing.T) {
	cands := append([]MatchCandidate{cand("run", "бег")}, cand("run", "бегать"))
	cands = append(cands, cand("house", "дом"), cand("water", "вода"), cand("book", "книга"), cand("dog", "собака"))
	m, ok := BuildMatch(cands, 1)
	if !ok || strings.Count(strings.Join(m.Words, ","), "run") != 1 {
		t.Fatalf("слово не должно повторяться: %v", m.Words)
	}
}

func TestBuildMatchNeedsFivePairs(t *testing.T) {
	if m, ok := BuildMatch(fiveCands()[:4], 1); ok {
		t.Fatalf("4 слова — мало: %+v", m)
	}
	if _, ok := BuildMatch(nil, 1); ok {
		t.Fatal("пусто")
	}
}

func TestBuildMatchSeedControlsOrderOfOptions(t *testing.T) {
	a, _ := BuildMatch(fiveCands(), 1)
	b, _ := BuildMatch(fiveCands(), 1)
	if !slices.Equal(a.Options, b.Options) {
		t.Fatalf("одинаковый seed — одинаковый порядок: %v %v", a.Options, b.Options)
	}
	different := false
	for seed := int64(2); seed < 40; seed++ {
		c, _ := BuildMatch(fiveCands(), seed)
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
		m, _ := BuildMatch(fiveCands(), seed)
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
