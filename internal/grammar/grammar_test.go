package grammar

import (
	"reflect"
	"strings"
	"testing"
)

func TestTopics(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"I have a dog", []string{"grammar:articles"}},
		{"She is in the garden", []string{"grammar:articles", "grammar:prepositions"}},
		{"I will call you", []string{"grammar:future"}},
		{"You must go now", []string{"grammar:modals"}},
		{"I will say you should", []string{"grammar:future", "grammar:modals"}},
		{"He went home", []string{"grammar:past_simple"}},
		{"They walked", []string{"grammar:past_simple"}},
		{"It was red", nil},
		{"THE DOG", []string{"grammar:articles"}},
		{"a an the a", []string{"grammar:articles"}},
		{"", nil},
		// going только в «going to»
		{"I am going home", nil},
		{"I am going to run", []string{"grammar:future"}},
		// причастия и пассив — не past_simple
		{"I have gone home", nil},
		{"It was written well", nil},
		{"She has got it", nil},
		// -ed: исключения и прилагательные-причастия
		{"a hundred men", []string{"grammar:articles"}},
		{"the speed", []string{"grammar:articles"}},
		{"I am tired", nil},
		{"He got married", nil},
		{"very excited", nil},
		{"I need it", nil},
		// неоднозначные слова
		{"a bit tired", []string{"grammar:articles"}},
		{"on the left", []string{"grammar:articles", "grammar:prepositions"}},
		// will как существительное
		{"free will", nil},
		{"my will", nil},
		{"I want to go", nil},
	}
	for _, tt := range tests {
		got := Topics(strings.Fields(tt.in))
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Topics(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	if got := Topics(nil); got != nil {
		t.Errorf("Topics(nil) = %v", got)
	}
}

func TestWords(t *testing.T) {
	if got := Words("grammar:articles"); !reflect.DeepEqual(got, []string{"a", "an", "the"}) {
		t.Errorf("articles = %v", got)
	}
	if got := Words("grammar:future"); !reflect.DeepEqual(got, []string{"will"}) {
		t.Errorf("future = %v", got)
	}
	if len(Words("grammar:prepositions")) < 25 || len(Words("grammar:modals")) != 7 {
		t.Errorf("размеры списков не те")
	}
	if Words("grammar:nope") != nil {
		t.Error("неизвестная тема должна дать nil")
	}
}

func TestInterchangeable(t *testing.T) {
	yes := [][2]string{{"may", "might"}, {"might", "may"}, {"may", "could"}, {"can", "could"}, {"over", "above"},
		{"under", "below"}, {"among", "between"}, {"in", "into"}, {"MAY", "might"}}
	for _, p := range yes {
		if !Interchangeable(p[0], p[1]) {
			t.Errorf("%s и %s должны считаться взаимозаменяемыми", p[0], p[1])
		}
	}
	no := [][2]string{{"must", "can"}, {"in", "under"}, {"a", "an"}, {"may", "may2"}}
	for _, p := range no {
		if Interchangeable(p[0], p[1]) {
			t.Errorf("%s и %s не группа", p[0], p[1])
		}
	}
}
