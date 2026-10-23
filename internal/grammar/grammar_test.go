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
	if got := Words("grammar:future"); !reflect.DeepEqual(got, []string{"will", "going"}) {
		t.Errorf("future = %v", got)
	}
	if len(Words("grammar:prepositions")) < 25 || len(Words("grammar:modals")) != 7 {
		t.Errorf("размеры списков не те")
	}
	if Words("grammar:nope") != nil {
		t.Error("неизвестная тема должна дать nil")
	}
}
