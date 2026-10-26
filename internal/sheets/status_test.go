package sheets

import "testing"

func ptr(s string) *string { return &s }

func TestWordStatus(t *testing.T) {
	cases := []struct {
		name     string
		learning *string
		card     bool
		lapses   int
		want     string
	}{
		{"нет данных", nil, false, 0, WordUnseen},
		{"только карточка", nil, true, 1, WordLearning},
		{"введено", ptr("introduced"), true, 0, WordIntroduced},
		{"изучено", ptr("learned"), true, 2, WordLearned},
		{"знаю с начала", ptr("known_at_start"), false, 0, WordKnownAtStart},
		{"забывал 3 раза — трудное", ptr("learned"), true, 3, WordHard},
		{"забывал 2 раза — не трудное", ptr("learned"), true, 2, WordLearned},
		{"забывания без карточки не считаются", ptr("learned"), false, 5, WordLearned},
		{"трудное без состояния", nil, true, 4, WordHard},
		{"неизвестное состояние", ptr("bogus"), false, 0, WordUnseen},
	}
	for _, c := range cases {
		if got := WordStatus(c.learning, c.card, c.lapses); got != c.want {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}

func TestBestStatus(t *testing.T) {
	if BestStatus() != WordUnseen {
		t.Fatal("пусто — unseen")
	}
	if got := BestStatus(WordLearned, WordHard, WordUnseen); got != WordHard {
		t.Fatalf("hard важнее: %s", got)
	}
	if got := BestStatus(WordUnseen, WordKnownAtStart); got != WordKnownAtStart {
		t.Fatalf("известное важнее незнакомого: %s", got)
	}
	if got := BestStatus("bogus"); got != WordUnseen {
		t.Fatalf("%s", got)
	}
}

func TestTopicWeak(t *testing.T) {
	all := []float64{1300, 1500, 1700}
	if !TopicWeak(1300, all) || TopicWeak(1500, all) || TopicWeak(1700, all) {
		t.Fatal("при трёх темах слаба только самая низкая")
	}
	if TopicWeak(1000, []float64{1000, 2000}) {
		t.Fatal("тем меньше трёх — слабой не бывает")
	}
	if TopicWeak(1500, []float64{1500, 1500, 1500}) {
		t.Fatal("три равных рейтинга: слабой нет")
	}
	if TopicWeak(1400, []float64{1400, 1400, 1600}) {
		t.Fatal("две равные ниже всех не помещаются в треть")
	}
	six := []float64{1, 2, 3, 4, 5, 6}
	if !TopicWeak(1, six) || !TopicWeak(2, six) || TopicWeak(3, six) {
		t.Fatal("нижняя треть из шести — две темы")
	}
}

func TestMineMistakes(t *testing.T) {
	if got := MineMistakes(nil); got == nil || len(got) != 0 {
		t.Fatalf("пустой банк — пустые строки, не nil: %#v", got)
	}
	got := MineMistakes([]MistakeRow{{"I _____ a dog", "have", "has"}})
	if len(got) != 1 || len(got[0].Cells) != 3 || got[0].Cells[2] != "has" {
		t.Fatalf("%+v", got)
	}
}
