package course

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

func lesson(id string, words []Word, sentences ...string) Lesson {
	l := Lesson{ID: id, Title: "Т", Words: words, Names: []string{"Anna", "Tom"},
		Questions: []Question{{Prompt: "?", Options: []string{"а", "б"}, Answer: 0}}}
	for _, s := range sentences {
		l.Sentences = append(l.Sentences, Sentence{En: s, Ru: "р"})
	}
	return l
}

var (
	w1 = []Word{{Lemma: "hello", Pos: "noun", Ru: "привет"}, {Lemma: "I", Pos: "pronoun", Ru: "я"}, {Lemma: "am", Pos: "be-verb", Ru: "есть", Forms: []string{"I'm"}}}
	w2 = []Word{{Lemma: "you", Pos: "pronoun", Ru: "ты"}, {Lemma: "are", Pos: "be-verb", Ru: "есть"}, {Lemma: "fine", Pos: "adjective", Ru: "хорошо"}}
)

func twoLessons() Course {
	return Course{Units: []Unit{{ID: 1, Title: "Б", Lessons: []Lesson{
		lesson("u1-l1", w1, "Hello! I am Anna.", "Hello, Anna! I am Tom.", "I'm Tom. Hello! Hello! I am Tom, hello!"),
		lesson("u1-l2", w2, "Hello, Tom! Are you fine?", "I am fine. You are fine!", "Anna's fine, and Tom is."),
	}}}}
}

func TestValidateAcceptsCourseBuiltFromPreviousWords(t *testing.T) {
	c := twoLessons()
	c.Units[0].Lessons[1].Sentences[2] = Sentence{En: "Anna's fine. You are fine, Tom!", Ru: "р"}
	if err := c.Validate(); err != nil {
		t.Fatalf("курс из слов прошлых уроков, имён и явных форм должен проходить: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(*Course){
		"слово из будущего урока": func(c *Course) { c.Units[0].Lessons[0].Sentences[2].En = "You are Tom. Hello, I am Anna." },
		"слово не из курса":       func(c *Course) { c.Units[0].Lessons[1].Sentences[2].En = "Anna is fine, Tom. You are fine." },
		"новое слово не в тексте": func(c *Course) {
			c.Units[0].Lessons[1].Words = append(c.Units[0].Lessons[1].Words, Word{Lemma: "how", Pos: "adverb", Ru: "как"})
		},
		"слово введено дважды": func(c *Course) { c.Units[0].Lessons[1].Words[0] = Word{Lemma: "hello", Pos: "noun", Ru: "п"} },
		"короткий текст":       func(c *Course) { c.Units[0].Lessons[0].Sentences = c.Units[0].Lessons[0].Sentences[:1] },
		"неверный ответ":       func(c *Course) { c.Units[0].Lessons[0].Questions[0].Answer = 2 },
		"плохой номер урока":   func(c *Course) { c.Units[0].Lessons[0].ID = "lesson1" },
		"повтор урока":         func(c *Course) { c.Units[0].Lessons[1].ID = "u1-l1" },
		"блок не с 1":          func(c *Course) { c.Units[0].ID = 2 },
		"одинаковые варианты":  func(c *Course) { c.Units[0].Lessons[0].Questions[0].Options = []string{"а", "а"} },
	}
	for name, mutate := range cases {
		c := twoLessons()
		c.Units[0].Lessons[1].Sentences[2] = Sentence{En: "Anna's fine. You are fine, Tom!", Ru: "р"}
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
}

func TestCheckVocabularyIsStrict(t *testing.T) {
	l := lesson("u1-l1", w1, "Hello! I am Anna. Paris is big, 5 times.")
	got := l.CheckVocabulary(nil)
	sort.Strings(got)
	if want := []string{"big", "is", "paris", "times"}; !slices.Equal(got, want) {
		t.Fatalf("строгая проверка: %v, ждали %v (числа и имена героев пропускаются, формы не выводятся)", got, want)
	}
}

func TestDrills(t *testing.T) {
	c := twoLessons()
	c.Units[0].Lessons[1].Sentences[2] = Sentence{En: "Anna's fine. You are fine, Tom!", Ru: "р"}
	first, second := Drills(c, 0), Drills(c, 1)
	if !slices.EqualFunc(first, Drills(c, 0), func(a, b Drill) bool { return a.Text == b.Text && a.Answer == b.Answer }) {
		t.Fatal("задания одного урока должны собираться одинаково")
	}
	kinds := map[string]int{}
	oldGaps := 0
	for _, d := range second {
		kinds[d.Kind]++
		switch d.Kind {
		case KindGap:
			if strings.Count(d.Text, "___") != 1 || !slices.Contains(d.Options, d.Answer) || len(d.Options) < 2 {
				t.Errorf("пропуск собран неверно: %+v", d)
			}
			if d.Old {
				oldGaps++
			}
		case KindOrder:
			tiles := append([]string(nil), d.Tiles...)
			want := strings.Fields(d.Answer)
			sort.Strings(tiles)
			sort.Strings(want)
			if !slices.Equal(tiles, want) {
				t.Errorf("плитки не совпадают со словами ответа: %+v", d)
			}
		case KindMatch:
			if len(d.Pairs) < 3 || !d.Old {
				t.Errorf("сопоставление должно включать и старые слова: %+v", d)
			}
		}
	}
	if oldGaps == 0 {
		t.Fatalf("со второго урока в закреплении должны быть слова прошлых уроков: %+v", second)
	}
	if kinds[KindGap] == 0 || kinds[KindMatch] != 1 {
		t.Fatalf("виды заданий: %v", kinds)
	}
	for _, d := range first {
		if d.Old {
			t.Fatalf("в первом уроке старых слов нет: %+v", d)
		}
	}
}

func TestBlankReplacesWholeWordOnly(t *testing.T) {
	got, ok := blank("Anna, I am Ian. I am here.", "I")
	if !ok || got != "Anna, ___ am Ian. I am here." {
		t.Fatalf("got %q", got)
	}
	if _, ok := blank("Ian is here.", "I"); ok {
		t.Fatal("часть слова не заменяется")
	}
}

func TestGlossaryHasFormsUpToLesson(t *testing.T) {
	g := Glossary(twoLessons(), 0)
	if g["i'm"] != "есть" || g["hello"] != "привет" || g["you"] != "" {
		t.Fatalf("словарик урока: %v", g)
	}
}

func TestGapOptionsShareAnswerCase(t *testing.T) {
	c := twoLessons()
	c.Units[0].Lessons[1].Sentences[2] = Sentence{En: "Anna's fine. You are fine, Tom!", Ru: "р"}
	for i := range c.Ordered() {
		for _, d := range Drills(c, i) {
			if d.Kind != KindGap {
				continue
			}
			upper := func(s string) bool { return s != "I" && strings.ToUpper(s[:1]) == s[:1] }
			for _, o := range d.Options {
				if o != "I" && d.Answer != "I" && upper(o) != upper(d.Answer) {
					t.Errorf("регистр подсказывает ответ: %q, варианты %v", d.Text, d.Options)
				}
			}
		}
	}
}

func TestOneGapPerLineAndUniquePairs(t *testing.T) {
	c := twoLessons()
	c.Units[0].Lessons[1].Sentences[2] = Sentence{En: "Anna's fine. You are fine, Tom!", Ru: "р"}
	// Два слова с одинаковым переводом: пары не должны повторять перевод.
	c.Units[0].Lessons[1].Words[2].Ru = "есть"
	for i := range c.Ordered() {
		seen := map[string]bool{}
		for _, d := range Drills(c, i) {
			switch d.Kind {
			case KindGap:
				if seen[d.Prompt+"|"+d.Text] {
					t.Errorf("урок %d: повтор задания %q", i+1, d.Text)
				}
				seen[d.Prompt+"|"+d.Text] = true
			case KindMatch:
				en, ru := map[string]bool{}, map[string]bool{}
				for _, p := range d.Pairs {
					if en[strings.ToLower(p.En)] || ru[p.Ru] {
						t.Errorf("урок %d: пары повторяются: %+v", i+1, d.Pairs)
					}
					en[strings.ToLower(p.En)], ru[p.Ru] = true, true
				}
			}
		}
	}
}

func TestBlankMatchesTokenizer(t *testing.T) {
	if got, ok := blank("Tom’s here.", "Tom's"); !ok || got != "___ here." {
		t.Errorf("типографский апостроф: %q %v", got, ok)
	}
	if got, ok := blank("I’m fine. I am Tom.", "I"); !ok || got != "I’m fine. ___ am Tom." {
		t.Errorf("слово внутри сокращения: %q %v", got, ok)
	}
	if got, ok := blank("A well-known man is well.", "well"); !ok || got != "A well-known man is ___." {
		t.Errorf("слово внутри слова через дефис: %q %v", got, ok)
	}
}

func TestCheckVocabularyChecksPartsAfterDigits(t *testing.T) {
	l := lesson("u1-l1", w1, "Hello! I am 5-year-old Anna, 12.")
	if got := l.CheckVocabulary(nil); !slices.Contains(got, "5-year-old") {
		t.Fatalf("слово с цифрой и буквами проверяется: %v", got)
	}
}
