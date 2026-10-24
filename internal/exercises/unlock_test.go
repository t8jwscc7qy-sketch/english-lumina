package exercises

import "testing"

func TestUnlockedDefaultOpen(t *testing.T) {
	for _, topic := range []string{"skill:form", "skill:match", "grammar:articles"} {
		if !Unlocked(topic, nil) {
			t.Errorf("%s: без рейтингов должна быть открыта", topic)
		}
	}
}

func TestUnlockedGate(t *testing.T) {
	gates := map[string]Gate{"skill:x": {Requires: "skill:order", MinRating: 1550, MinAttempts: 10}}
	if unlocked("skill:x", nil, gates) {
		t.Error("нет рейтинга: ожидали закрыто")
	}
	if unlocked("skill:x", []TopicRating{{Topic: "skill:order", Mu: 1560, Attempts: 9}}, gates) {
		t.Error("9 попыток: ожидали закрыто")
	}
	if !unlocked("skill:x", []TopicRating{{Topic: "skill:order", Mu: 1560, Attempts: 10}}, gates) {
		t.Error("1560 при 10 попытках: ожидали открыто")
	}
}
