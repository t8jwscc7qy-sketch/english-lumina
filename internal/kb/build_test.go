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
			"colour": 200, "color": 150, "nothing": 5000, "father": 2000,
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
