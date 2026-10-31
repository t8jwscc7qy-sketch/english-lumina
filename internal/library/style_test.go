package library

import "testing"

func styleText(targets []Target, sentences ...string) Text {
	t := Text{ID: "t", Band: 100, Targets: targets}
	for _, s := range sentences {
		t.Sentences = append(t.Sentences, Sentence{En: s, Ru: "я"})
	}
	return t
}

func styleRules(t Text) map[string]bool {
	got := map[string]bool{}
	for _, is := range t.CheckStyle() {
		got[is.Rule] = true
	}
	return got
}

func TestCheckStyleFindsLearnerMistakes(t *testing.T) {
	dog := []Target{{Lemma: "dog", Pos: "noun"}}
	cases := map[string]struct {
		text Text
		rule string
	}{
		"can not":            {styleText(dog, "He can not see the dog."), "can not"},
		"без апострофа":      {styleText(dog, "He dont see the dog."), "апостроф"},
		"без точки":          {styleText(dog, "He sees the dog"), "знак в конце"},
		"строчная":           {styleText(dog, "he sees the dog."), "заглавная"},
		"a перед гласной":    {styleText(dog, "He sees a apple."), "a/an"},
		"an перед согласной": {styleText(dog, "He sees an dog."), "a/an"},
		"нет артикля":        {styleText(dog, "He sees dog."), "артикль"},
		"время":              {styleText(dog, "Tom went home.", "Then he says hello."), "время"},
	}
	for name, c := range cases {
		if !styleRules(c.text)[c.rule] {
			t.Errorf("%s: правило %q не сработало: %v", name, c.rule, c.text.CheckStyle())
		}
	}
}

func TestCheckStyleAcceptsCorrectEnglish(t *testing.T) {
	dog := []Target{{Lemma: "dog", Pos: "noun"}, {Lemma: "water", Pos: "noun", Mass: true}}
	tx := styleText(dog,
		"He cannot see the dog.",
		"\"I can't find it!\" said Tom.",
		"There was water in a university.",
		"Tom saw an old dog. Then he went home.",
		"Dog, come here!",
	)
	if got := tx.CheckStyle(); len(got) != 0 {
		t.Fatalf("правильный английский не должен вызывать замечаний: %v", got)
	}
}

func TestCheckStyleAllowSilencesOneIssue(t *testing.T) {
	tx := styleText([]Target{{Lemma: "night", Pos: "noun"}}, "It is night.")
	if len(tx.CheckStyle()) == 0 {
		t.Fatal("без исключения замечание ожидалось")
	}
	tx.Allow = []string{"артикль:1"}
	if got := tx.CheckStyle(); len(got) != 0 {
		t.Fatalf("исключение должно снять замечание: %v", got)
	}
}

func TestCheckVocabularyAcceptsInflectedForms(t *testing.T) {
	allowed := AllowedForms([]BandWord{{Lemma: "find", Pos: "verb"}, {Lemma: "go", Pos: "verb"}, {Lemma: "be", Pos: "verb"}, {Lemma: "he", Pos: "pronoun"}, {Lemma: "it", Pos: "pronoun"}, {Lemma: "home", Pos: "adverb"}})
	tx := styleText(nil, "He found it.", "He went home.", "It was here.", "He goed home.")
	var bad []string
	for _, p := range tx.CheckVocabulary(allowed) {
		bad = append(bad, p.Word)
	}
	want := map[string]bool{"here": true, "goed": true}
	if len(bad) != len(want) {
		t.Fatalf("вне словаря должны быть только here и goed: %v", bad)
	}
	for _, w := range bad {
		if !want[w] {
			t.Errorf("лишнее слово вне словаря: %s", w)
		}
	}
}
