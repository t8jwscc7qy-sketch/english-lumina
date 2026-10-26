package kb

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func fixtureInputs(t *testing.T) Inputs {
	t.Helper()
	var entries []Entry
	for _, w := range []struct{ word, lang string }{{"dad", "en"}, {"clothes", "en"}, {"blow", "en"}, {"thin", "en"}, {"colour", "en"}, {"color", "en"}, {"nothing", "en"}} {
		e, err := ParseEntry(lineFor(t, w.word, w.lang))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return Inputs{
		Lexemes: []Lexeme{
			{1, "dad", "noun", "A1"}, {2, "clothes", "noun", "A1"}, {3, "blow", "verb", "B1"}, {4, "thin", "adjective", "A2"},
			{5, "colour/color", "noun", "A2"}, {6, "nothing", "pronoun", "A1"}, {7, "ghost", "noun", "C1"}, {8, "father", "noun", "A1"},
		},
		Entries: entries,
		Legacy: []LegacyTranslation{
			{"dad", "n", 1, "папа"}, {"dad", "n", 1, "батюшка"}, {"clothes", "n", 1, "одёжа"},
			{"thin", "adj", 1, "редкий"}, {"ghost", "n", 1, "призрак"},
		},
		RuRank: map[string]int{"папа": 274, "батюшка": 30608, "одежда": 4303, "дуть": 4000, "сосать": 3000, "тонкий": 16560, "редкий": 12922, "цвет": 150, "призрак": 8000},
		EnCount: map[string]int64{
			"dad": 1000, "dads": 100, "blow": 500, "blows": 50, "blew": 40, "thin": 300, "clothes": 400,
			"colour": 200, "color": 150, "nothing": 5000, "father": 2000, "pa": 50, "thick": 800, "fat": 700, "slim": 300,
		},
		SourceIDs: map[string]int64{"kaikki": 11, "freedict": 12},
	}
}

func chosenTexts(out Output, id int64) []string {
	var rows []TranslationRow
	for _, r := range out.Translations {
		if r.LexemeID == id && r.Chosen {
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b TranslationRow) int { return a.Rank - b.Rank })
	var texts []string
	for _, r := range rows {
		texts = append(texts, r.Text)
	}
	return texts
}

func TestAssembleChoosesTranslations(t *testing.T) {
	out, _ := Assemble(fixtureInputs(t))
	for id, want := range map[int64][]string{
		1: {"папа", "бать"}, // «батюшка» (archaic) не выбирается
		2: {"одежда"},       // «одежа» (dialectal) и «одёжа» (старый словарь) слишком слабые для второго
		3: {"дуть"},         // «сосать» (vulgar) не выбирается вторым
		4: {"тонкий", "редкий"},
		5: {"цвет"},
		6: nil,
		7: {"призрак"}, // только из старого словаря
		8: nil,
	} {
		if got := chosenTexts(out, id); !slices.Equal(got, want) {
			t.Errorf("слово %d: выбрано %v, ждали %v", id, got, want)
		}
	}
	for _, r := range out.Translations {
		if r.Chosen && (r.Rank < 1 || r.Rank > 2) {
			t.Errorf("выбранный перевод с рангом %d: %+v", r.Rank, r)
		}
		if r.Lang != "ru" {
			t.Errorf("язык %q", r.Lang)
		}
	}
	var kaikkiAndLegacy bool
	for _, r := range out.Translations {
		if r.LexemeID == 1 && r.Text == "папа" && len(r.SourceIDs) == 2 {
			kaikkiAndLegacy = true
		}
	}
	if !kaikkiAndLegacy {
		t.Fatal("у «папа» должны быть оба источника (11 и 12)")
	}
}

func TestAssembleSensesPronunciationsFormsRelations(t *testing.T) {
	out, _ := Assemble(fixtureInputs(t))
	var senseOfNothing *SenseRow
	for i, s := range out.Senses {
		if s.LexemeID == 6 {
			senseOfNothing = &out.Senses[i]
		}
		if s.LexemeID == 3 && s.SenseNo == 2 && !slices.Contains(s.Flags, "vulgar") {
			t.Errorf("значение blow №2 без флага vulgar: %+v", s)
		}
		if s.SourceID != 11 {
			t.Errorf("значение не из Kaikki: %+v", s)
		}
	}
	if senseOfNothing == nil || senseOfNothing.GlossEN != "(pronoun) not anything" || senseOfNothing.SenseNo != 1 {
		t.Fatalf("значение местоимения (pos pron) должно находиться: %+v", senseOfNothing)
	}

	var us, uk *PronRow
	for i, p := range out.Pronunciations {
		if p.LexemeID == 1 && p.Accent == "us" {
			us = &out.Pronunciations[i]
		}
		if p.LexemeID == 1 && p.Accent == "uk" {
			uk = &out.Pronunciations[i]
		}
	}
	if us == nil || us.IPA != "dæd" || us.Audio != "En-us-dad.ogg" || !us.Primary || uk == nil || uk.Primary {
		t.Fatalf("произношения dad: %+v / %+v", us, uk)
	}

	forms := map[int64][]string{}
	for _, f := range out.Forms {
		forms[f.LexemeID] = append(forms[f.LexemeID], f.Form)
	}
	for _, want := range []string{"blow", "blows", "blew", "blown", "blowing"} {
		if !slices.Contains(forms[3], want) {
			t.Errorf("у blow нет формы %q: %v", want, forms[3])
		}
	}
	if !slices.Contains(forms[5], "colour") || !slices.Contains(forms[5], "color") {
		t.Errorf("варианты лемм должны быть формами: %v", forms[5])
	}

	var syn, other *RelationRow
	for i, r := range out.Relations {
		if r.LexemeID == 1 && r.Kind == "synonym" && r.OtherLemma == "father" {
			syn = &out.Relations[i]
		}
		if r.LexemeID == 4 && r.Kind == "antonym" && r.OtherLemma == "thick" {
			other = &out.Relations[i]
		}
	}
	if syn == nil || syn.OtherLexemeID == nil || *syn.OtherLexemeID != 8 {
		t.Fatalf("синоним father должен ссылаться на слово 8: %+v", syn)
	}
	if other == nil || other.OtherLexemeID != nil {
		t.Fatalf("антоним thick не из словаря: ссылка пуста: %+v", other)
	}
}

func TestAssembleStatsAndCards(t *testing.T) {
	out, _ := Assemble(fixtureInputs(t))
	stat := map[int64]StatRow{}
	for _, s := range out.Stats {
		stat[s.LexemeID] = s
	}
	if stat[1].Count != 1100 || stat[3].Count != 590 || stat[5].Count != 350 {
		t.Fatalf("суммы по формам: dad %d, blow %d, colour/color %d", stat[1].Count, stat[3].Count, stat[5].Count)
	}
	if stat[6].Rank == nil || *stat[6].Rank != 1 || stat[8].Rank == nil || *stat[8].Rank != 2 || stat[7].Rank != nil {
		t.Fatalf("ранги: nothing %v, father %v, ghost %v", stat[6].Rank, stat[8].Rank, stat[7].Rank)
	}
	cards := map[int64]map[string]any{}
	for _, c := range out.Cards {
		var m map[string]any
		if err := json.Unmarshal(c.Payload, &m); err != nil {
			t.Fatal(err)
		}
		cards[c.LexemeID] = m
	}
	if cards[1]["lemma"] != "dad" || cards[1]["ipa"] != "dæd" || cards[1]["level"] != "A1" {
		t.Fatalf("карточка dad: %v", cards[1])
	}
	if tr, _ := cards[1]["translations"].([]any); len(tr) != 2 || tr[0] != "папа" {
		t.Fatalf("переводы в карточке dad: %v", cards[1]["translations"])
	}
	if syn, _ := cards[1]["synonyms"].([]any); len(syn) != 2 {
		t.Fatalf("синонимы в карточке dad: %v", cards[1]["synonyms"])
	}
	if _, ok := cards[7]; !ok {
		t.Fatal("у ghost карточка из старого словаря должна быть")
	}
	if _, ok := cards[6]; ok {
		t.Fatal("у nothing нет ни перевода, ни произношения: карточки нет")
	}
}

func TestAssembleReport(t *testing.T) {
	_, rep := Assemble(fixtureInputs(t))
	a1 := rep.Levels["A1"]
	if a1.Total != 4 || a1.WithTranslation != 2 || a1.WithIPA != 2 {
		t.Fatalf("A1: %+v", a1)
	}
	if rep.Levels["C1"] != (LevelStats{Total: 1, WithTranslation: 1}) {
		t.Fatalf("C1: %+v", rep.Levels["C1"])
	}
	if rep.Golden.Checked != 5 || rep.Golden.Passed != 4 || len(rep.Golden.Failed) != 1 || rep.Golden.Failed[0].Lemma != "father" {
		t.Fatalf("контрольный список: %+v", rep.Golden)
	}
	if rep.Flagged < 1 {
		t.Fatalf("помеченные переводы: %d", rep.Flagged)
	}
	if len(rep.Sample) != 6 { // слова с переводом: dad, clothes, blow, thin, colour/color, ghost
		t.Fatalf("выборка %d", len(rep.Sample))
	}
}

func TestReportCheck(t *testing.T) {
	_, rep := Assemble(fixtureInputs(t))
	if err := rep.Check(Options{}); err != nil {
		t.Fatalf("без порогов ошибки нет: %v", err)
	}
	if err := rep.Check(Options{MinCoverage: map[string]float64{"A1": 0.4}, MinGolden: 0.8}); err != nil {
		t.Fatalf("пороги выполнены: %v", err)
	}
	if err := rep.Check(Options{MinCoverage: map[string]float64{"A1": 0.9}}); !errors.Is(err, ErrBelowThreshold) {
		t.Fatalf("покрытие A1 ниже порога: %v", err)
	}
	if err := rep.Check(Options{MinGolden: 0.9}); !errors.Is(err, ErrBelowThreshold) {
		t.Fatalf("контрольный список ниже порога: %v", err)
	}
}

func TestAssembleIsDeterministic(t *testing.T) {
	in := fixtureInputs(t)
	a, repA := Assemble(in)
	slices.Reverse(in.Lexemes)
	slices.Reverse(in.Entries)
	b, repB := Assemble(in)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(repA, repB) {
		t.Fatal("порядок входа не должен менять результат")
	}
}

func TestAssembleWithoutKaikkiKeepsLegacy(t *testing.T) {
	in := fixtureInputs(t)
	in.Entries = nil
	out, _ := Assemble(in)
	if got := chosenTexts(out, 1); !slices.Equal(got, []string{"папа"}) {
		t.Fatalf("без Kaikki остаётся лучший из старого словаря: %v", got)
	}
	if len(out.Senses) != 0 || len(out.Pronunciations) != 0 {
		t.Fatal("без Kaikki значений и произношений нет")
	}
}

func TestKaikkiPosMapping(t *testing.T) {
	for in, want := range map[string][]string{
		"noun": {"noun"}, "adjective": {"adj"}, "adverb": {"adv"}, "pronoun": {"pron"}, "preposition": {"prep"},
		"modal auxiliary": {"verb"}, "be-verb": {"verb"}, "infinitive-to": {"particle"}, "unknown": nil,
	} {
		if got := KaikkiPos(in); !slices.Equal(got, want) {
			t.Errorf("%q: %v, ждали %v", in, got, want)
		}
	}
	if LegacyPos("adjective") != "adj" || LegacyPos("pronoun") != "" {
		t.Fatal("отображение частей речи старого словаря")
	}
}

// Номер значения у перевода сохраняется только от Kaikki: нумерация старого словаря с таблицей значений не связана.
func TestAssembleTranslationSenseOnlyFromKaikki(t *testing.T) {
	out, _ := Assemble(fixtureInputs(t))
	known := map[[2]int64]bool{}
	for _, s := range out.Senses {
		known[[2]int64{s.LexemeID, int64(s.SenseNo)}] = true
	}
	for _, r := range out.Translations {
		if r.SenseNo != nil && !known[[2]int64{r.LexemeID, int64(*r.SenseNo)}] {
			t.Errorf("перевод ссылается на несуществующее значение: %+v", r)
		}
		if r.LexemeID == 7 && r.SenseNo != nil {
			t.Errorf("перевод только из старого словаря не должен иметь номера значения: %+v", r)
		}
	}
	var withSense bool
	for _, r := range out.Translations {
		if r.LexemeID == 1 && r.Text == "папа" && r.SenseNo != nil && *r.SenseNo == 1 {
			withSense = true
		}
	}
	if !withSense {
		t.Fatal("у «папа» из Kaikki номер значения 1 сохраняется")
	}
}

// Пояснение перевода у Kaikki — свободный текст; к значению оно привязывается по общим словам, а порядок групп
// переводов задаёт основное значение.
func TestAssembleTranslationGroupsAndSenseMatching(t *testing.T) {
	entry := Entry{Word: "dog", Pos: "noun", LangCode: "en",
		Senses: []Sense{
			{Glosses: []string{"A mammal of the family Canidae:", "The species Canis familiaris."}},
			{Glosses: []string{"A person:", "Someone who is cowardly, worthless, or morally reprehensible."}, Tags: []string{"derogatory"}},
		},
		Translations: []Translation{
			{Code: "ru", Word: "соба́ка", Sense: "animal"},
			{Code: "ru", Word: "пёс", Sense: "animal"},
			{Code: "ru", Word: "соба́ка", Sense: "morally reprehensible person — see also scoundrel"},
			{Code: "ru", Word: "су́ка", Sense: "morally reprehensible person — see also scoundrel"},
		},
	}
	in := Inputs{Lexemes: []Lexeme{{1, "dog", "noun", "A1"}}, Entries: []Entry{entry}, RuRank: map[string]int{"собака": 300, "пёс": 2000, "сука": 3000}, SourceIDs: map[string]int64{"kaikki": 11}}
	out, _ := Assemble(in)
	if got := chosenTexts(out, 1); len(got) == 0 || got[0] != "собака" {
		t.Fatalf("основной перевод dog: %v", got)
	}
	var suka *TranslationRow
	for i, r := range out.Translations {
		if r.Text == "сука" {
			suka = &out.Translations[i]
		}
	}
	if suka == nil || suka.SenseNo == nil || *suka.SenseNo != 2 || !slices.Contains(suka.Flags, "derogatory") {
		t.Fatalf("«сука» привязана к значению 2 с пометкой derogatory: %+v", suka)
	}
}

// У слова бывает несколько статей по этимологии: группы переводов нумеруются сквозь статьи по порядку этимологии,
// иначе «первая группа» второй статьи (косяк рыб) обгоняет основное значение первой (школа). Частота слов одинаковая:
// иначе тест проходил бы и без нумерации статей.
func TestAssembleEtymologyOrderDecidesGroups(t *testing.T) {
	school := Entry{Word: "school", Pos: "noun", LangCode: "en", EtymNo: 1,
		Senses:       []Sense{{Glosses: []string{"An institution dedicated to teaching and learning."}}},
		Translations: []Translation{{Code: "ru", Word: "шко́ла", Sense: "an institution dedicated to teaching and learning"}},
	}
	fish := Entry{Word: "school", Pos: "noun", LangCode: "en", EtymNo: 2,
		Senses:       []Sense{{Glosses: []string{"A group of fish or marine mammals."}}},
		Translations: []Translation{{Code: "ru", Word: "косяк", Sense: "group of fish"}},
	}
	for _, order := range [][]Entry{{school, fish}, {fish, school}} {
		in := Inputs{Lexemes: []Lexeme{{1, "school", "noun", "A1"}}, Entries: order, RuRank: map[string]int{"школа": 1000, "косяк": 1000}, SourceIDs: map[string]int64{"kaikki": 11}}
		out, _ := Assemble(in)
		if got := chosenTexts(out, 1); len(got) == 0 || got[0] != "школа" {
			t.Fatalf("основной перевод school: %v", got)
		}
	}
}

// Слабое (нечёткое) совпадение пояснения со значением не должно переносить на перевод пометки значения.
func TestAssembleWeakSenseMatchDoesNotTransferTags(t *testing.T) {
	entry := Entry{Word: "morning", Pos: "noun", LangCode: "en",
		Senses: []Sense{
			{Glosses: []string{"The early part of the day, especially from midnight to noon."}},
			{Glosses: []string{"That part of the day from dawn until the main meal."}, Tags: []string{"obsolete"}},
		},
		Translations: []Translation{{Code: "ru", Word: "у́тро", Sense: "part of the day between dawn and midday"}},
	}
	in := Inputs{Lexemes: []Lexeme{{1, "morning", "noun", "A1"}}, Entries: []Entry{entry}, RuRank: map[string]int{"утро": 600}, SourceIDs: map[string]int64{"kaikki": 11}}
	out, _ := Assemble(in)
	for _, r := range out.Translations {
		if r.Text == "утро" && (len(r.Flags) > 0 || r.SenseNo != nil) {
			t.Fatalf("слабое совпадение не должно давать пометок и номера значения: %+v", r)
		}
	}
}

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions()
	if o.MinGolden < 0.9 || len(o.MinCoverage) != 6 {
		t.Fatalf("%+v", o)
	}
	for level, v := range o.MinCoverage {
		if v <= 0.5 || v >= 1 {
			t.Errorf("порог %s = %v вне разумных границ", level, v)
		}
	}
}

// Формы и синонимы из Викисловаря бывают устаревшими и диалектными: в базу попадают обычные формы и обычные слова.
func TestAssembleFiltersNoisyFormsAndSynonyms(t *testing.T) {
	entry := Entry{Word: "dad", Pos: "noun", LangCode: "en",
		Senses: []Sense{{Glosses: []string{"A father."}}},
		Forms: []Form{
			{Form: "dads", Tags: []string{"plural"}},
			{Form: "dadde", Tags: []string{"alternative", "obsolete"}},
			{Form: "dodge", Tags: []string{"dialectal"}},
			{Form: "dadz", Tags: []string{"nonstandard"}},
			{Form: "-", Tags: []string{"plural"}},
		},
		Synonyms: []string{"ayr", "pa", "father", "da", "papa"},
		Sounds:   []Sound{{IPA: "/dæd/"}}, // карточка строится, когда есть перевод или произношение
	}
	in := Inputs{
		Lexemes:   []Lexeme{{1, "dad", "noun", "A1"}, {2, "father", "noun", "A1"}},
		Entries:   []Entry{entry},
		EnCount:   map[string]int64{"dad": 1000, "dads": 100, "father": 5000, "pa": 50, "papa": 400},
		SourceIDs: map[string]int64{"kaikki": 11},
	}
	out, _ := Assemble(in)
	var forms []string
	for _, f := range out.Forms {
		if f.LexemeID == 1 {
			forms = append(forms, f.Form)
		}
	}
	slices.Sort(forms)
	if !slices.Equal(forms, []string{"dad", "dads"}) {
		t.Fatalf("формы: %v (устаревшие, диалектные и нестандартные не нужны)", forms)
	}
	var syn []string
	for _, r := range out.Relations {
		if r.LexemeID == 1 && r.Kind == "synonym" {
			syn = append(syn, r.OtherLemma)
		}
	}
	slices.Sort(syn)
	if !slices.Equal(syn, []string{"father", "pa", "papa"}) {
		t.Fatalf("синонимы: %v (редких слов вне словаря и частот быть не должно)", syn)
	}
	var card map[string]any
	for _, c := range out.Cards {
		if c.LexemeID == 1 {
			if err := json.Unmarshal(c.Payload, &card); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, _ := card["synonyms"].([]any)
	if len(got) != 3 || got[0] != "father" || got[1] != "papa" || got[2] != "pa" {
		t.Fatalf("в карточке синонимы по убыванию частоты: %v", got)
	}
}

// Короткое пояснение («go») не должно находиться подстрокой внутри чужого слова («good») и переносить чужие пометки.
func TestAssembleShortSenseTextDoesNotMatchInsideWords(t *testing.T) {
	entry := Entry{Word: "go", Pos: "verb", LangCode: "en",
		Senses:       []Sense{{Glosses: []string{"A good thing to do."}, Tags: []string{"obsolete"}}},
		Translations: []Translation{{Code: "ru", Word: "идти", Sense: "go"}},
	}
	in := Inputs{Lexemes: []Lexeme{{1, "go", "verb", "A1"}}, Entries: []Entry{entry}, SourceIDs: map[string]int64{"kaikki": 11}}
	out, _ := Assemble(in)
	for _, r := range out.Translations {
		if r.Text == "идти" && (r.SenseNo != nil || len(r.Flags) > 0) {
			t.Fatalf("«go» не должно привязаться к «good»: %+v", r)
		}
	}
}

func TestStatCountSkipsAbbreviations(t *testing.T) {
	forms := []FormRow{
		{Form: "inch", LexemeID: 1, Tag: "lemma"}, {Form: "inches", LexemeID: 1, Tag: "plural"},
		{Form: "in", LexemeID: 1, Tag: "abbreviation"}, {Form: "in.", LexemeID: 1, Tag: "abbreviation"},
		{Form: "inches", LexemeID: 1, Tag: "plural"}, // повтор формы не считается дважды
		{Form: "foot", LexemeID: 2, Tag: "lemma"},
	}
	counts := map[string]int64{"inch": 10, "inches": 5, "in": 7000000, "foot": 99}
	if got := statCount(forms, 1, counts, false); got != 15 {
		t.Fatalf("счёт inch %d, ждали 15 (без сокращения «in»)", got)
	}
	if got := statCount(forms, 2, counts, false); got != 99 {
		t.Fatalf("чужие формы не считаются: %d", got)
	}
}

func TestStatCountForPronounsUsesOnlyTheLemma(t *testing.T) {
	forms := []FormRow{
		{Form: "himself", LexemeID: 1, Tag: "lemma"}, {Form: "he", LexemeID: 1, Tag: "reflexive-of"},
		{Form: "herself", LexemeID: 1, Tag: "feminine"}, {Form: "itself", LexemeID: 1, Tag: "neuter"}, {Form: "themselves", LexemeID: 1, Tag: "plural"},
	}
	counts := map[string]int64{"himself": 100, "he": 9000000, "herself": 400, "itself": 500, "themselves": 600}
	if got := statCount(forms, 1, counts, true); got != 100 {
		t.Fatalf("himself %d, ждали 100: чужие формы (he, herself, itself) не считаются", got)
	}
	// У обычного слова другая форма с тегом reflexive-of тоже не считается.
	if got := statCount(forms, 1, counts, false); got != 100+400+500+600 {
		t.Fatalf("без режима «только лемма» reflexive-of всё равно пропускается: %d", got)
	}
}
