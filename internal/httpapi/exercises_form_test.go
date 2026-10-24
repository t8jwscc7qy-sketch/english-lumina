package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const formSentence = "Yesterday she walked home."

func formExamples(enID int64, text string) []db.ListExamplesRow {
	return []db.ListExamplesRow{{EnID: enID, EnTatoebaID: enID + 1000, EnText: text, EnAuthor: "a", RuTatoebaID: enID + 2000, RuText: "Перевод", RuAuthor: "b"}}
}

// formFirst: у остальных тем рейтинг выше, поэтому skill:form (без попыток) выбирается первой.
func formFirst(e env) {
	orderFirst(e)
	e.store.topicRatings[topicKey{1, orderTopic}] = rated(orderTopic, 1700, 5)
}

// freqOf кладёт слова в список частот фейка (ListWordFrequencies отдаёт только их).
func freqOf(e env, ws ...string) {
	for i, w := range ws {
		e.store.frequencies = append(e.store.frequencies, db.ListWordFrequenciesRow{Word: w, Rank: int32(i + 1)})
	}
}

func formEnv(t *testing.T) (env, string) {
	e := newExerciseEnv(t)
	token := login(t, e)
	formFirst(e)
	knownAll(e, "A1", "yesterday", "she", "walked", "home", "tomorrow", "will", "we", "he")
	freqOf(e, "walk", "walks", "walked", "walking", "talk", "talks", "talked", "talking")
	return e, token
}

func storedAnswer(t *testing.T, e env, id int64) exerciseAnswer {
	t.Helper()
	var a exerciseAnswer
	if err := json.Unmarshal(e.store.exercises[id].Answer, &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestNextFormTopic(t *testing.T) {
	e, token := formEnv(t)
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "walk", Pos: "verb"}}}
	e.store.examples = formExamples(7, formSentence)

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Form || ex.Topic != "skill:form" || ex.Hint == nil || *ex.Hint != "walk" ||
		ex.Prompt != "Yesterday she _____ home." || len(ex.Choices) < 3 || len(ex.Choices) > 4 || !slices.Contains(ex.Choices, "walked") {
		t.Fatalf("упражнение: %+v", ex)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"answer", "answers", "correct", "word"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("в ответе есть поле %q: %s", k, rec.Body)
		}
	}
	stored := e.store.exercises[ex.Id]
	if stored.Kind != "form" || stored.SentenceID != 7 || !slices.Contains(stored.Topics, "skill:form") || !slices.Contains(stored.Topics, "level:A1") {
		t.Fatalf("запись: %+v", stored)
	}
	a := storedAnswer(t, e, ex.Id)
	if !slices.Equal(a.Answers, []string{"walked"}) || a.LexemeID != 1 || a.Lemma != "walk" || a.Pos != "verb" {
		t.Fatalf("ответ в записи: %+v", a)
	}
}

func TestNextFormPrefersWeakCards(t *testing.T) {
	e, token := formEnv(t)
	e.store.weakWords = []db.ListWeakCardWordsRow{{LexemeID: 9, Lemma: "talk", Pos: "verb"}}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "walk", Pos: "verb"}}}
	var queries []string
	e.store.examplesFor = func(q string) []db.ListExamplesRow {
		queries = append(queries, q)
		if strings.Contains(q, "'talk'") {
			return formExamples(7, "Yesterday she talked home.")
		}
		return formExamples(8, formSentence) // для walk тоже есть рабочее предложение
	}
	knownAll(e, "A1", "talked")
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Kind != api.Form || ex.Hint == nil || *ex.Hint != "talk" {
		t.Fatalf("упражнение: %+v", ex)
	}
	if a := storedAnswer(t, e, ex.Id); a.LexemeID != 9 {
		t.Fatalf("слово: %+v", a)
	}
	if len(queries) == 0 || !strings.Contains(queries[0], "'talk'") {
		t.Fatalf("первый поиск примеров был не для слабого слова: %v", queries)
	}
}

func TestNextFormNoSentenceFallsThrough(t *testing.T) {
	e, token := formEnv(t)
	e.store.weakWords = []db.ListWeakCardWordsRow{{LexemeID: 9, Lemma: "talk", Pos: "verb"}}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "walk", Pos: "verb"}}}
	e.store.examplesFor = func(q string) []db.ListExamplesRow {
		if strings.Contains(q, "'talk'") {
			return nil
		}
		return formExamples(7, formSentence)
	}
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Kind != api.Form || *ex.Hint != "walk" {
		t.Fatalf("упражнение: %+v", ex)
	}
}

func TestNextFormNoSentenceFallsToLevelTopic(t *testing.T) {
	e, token := formEnv(t)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.weakWords = []db.ListWeakCardWordsRow{{LexemeID: 9, Lemma: "talk", Pos: "verb"}}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {
		{ID: 1, Lemma: "dog", Pos: "noun"}, {ID: 2, Lemma: "cat", Pos: "noun"}, {ID: 3, Lemma: "house", Pos: "noun"},
		{ID: 4, Lemma: "table", Pos: "noun"}, {ID: 5, Lemma: "book", Pos: "noun"},
	}}
	e.store.examplesFor = func(q string) []db.ListExamplesRow {
		if strings.Contains(q, "'dog'") {
			return formExamples(8, "The dog is very big.")
		}
		return nil
	}
	knownWords(e, "the", "dog", "is", "very", "big")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if ex := decodeExercise(t, rec); ex.Kind == api.Form || !strings.HasPrefix(ex.Topic, "level:") {
		t.Fatalf("упражнение: %+v", ex)
	}
}

func TestNextFormNoCardsNoSettings(t *testing.T) {
	e, token := formEnv(t)
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "walk", Pos: "verb"}}}
	e.store.examples = formExamples(7, formSentence)
	if ex := decodeExercise(t, getNext(t, e, token)); ex.Kind != api.Form || *ex.Hint != "walk" {
		t.Fatalf("упражнение: %+v", ex)
	}
	e2, token2 := formEnv(t)
	rec := getNext(t, e2, token2)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExercise {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestNextFormSkipsNouns(t *testing.T) {
	e, token := formEnv(t)
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "dog", Pos: "noun"}}}
	e.store.examples = formExamples(7, "Yesterday the dogs ran.")
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if e.store.exampleCalls != 0 {
		t.Fatalf("поиск примеров для существительного: %d", e.store.exampleCalls)
	}
}

// formAnswered выдаёт form-упражнение, на которое можно отвечать; карточка слова — по желанию.
func formAnswered(t *testing.T, e env, token string, withCard bool) int64 {
	t.Helper()
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "walk", Pos: "verb"}}}
	e.store.examples = formExamples(7, formSentence)
	if withCard {
		e.store.cards = map[int64]db.Card{1: {ID: 1, UserID: 1, LexemeID: 1}}
	}
	return decodeExercise(t, getNext(t, e, token)).Id
}

func TestAnswerFormCorrectWrong(t *testing.T) {
	for _, tc := range []struct {
		given   string
		correct bool
	}{{"walked", true}, {"walking", false}} {
		e, token := formEnv(t)
		id := formAnswered(t, e, token, false)
		rec := e.do(http.MethodPost, answerPath, answerBody(id, tc.given, "k1"), withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
		}
		res := decodeResult(t, rec)
		if res.Correct != tc.correct || res.Answer != "walked" {
			t.Fatalf("результат: %+v", res)
		}
		for _, topic := range []string{"skill:form", "level:A1"} {
			if r, ok := ratingOf(res, topic); !ok || r.Attempts != 1 {
				t.Fatalf("рейтинг %s: %+v", topic, res.Ratings)
			}
		}
		evs := answerEvents(e)
		if len(evs) != 1 {
			t.Fatalf("событий %d", len(evs))
		}
		var p map[string]any
		_ = json.Unmarshal(evs[0].Payload, &p)
		if p["kind"] != "form" || p["lexemeId"] != float64(1) || p["correct"] != tc.correct {
			t.Fatalf("событие: %s", evs[0].Payload)
		}
	}
}

func TestAnswerFormWord(t *testing.T) {
	for _, inCards := range []bool{true, false} {
		e, token := formEnv(t)
		id := formAnswered(t, e, token, inCards)
		res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(id, "walked", "k1"), withCookie(token)))
		if res.Word == nil || res.Word.Lemma != "walk" || res.Word.Pos != "verb" || res.Word.InCards != inCards {
			t.Fatalf("word: %+v", res.Word)
		}
	}
}

func TestAnswerGapHasNoWord(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	if res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(id, "a", "k"), withCookie(token))); res.Word != nil {
		t.Fatalf("word у gap: %+v", res.Word)
	}
}

func TestAnswerFormRepeatByKey(t *testing.T) {
	e, token := formEnv(t)
	id := formAnswered(t, e, token, true)
	first := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(id, "walked", "k1"), withCookie(token)))
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "walked", "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("повтор: %d %s", rec.Code, rec.Body)
	}
	again := decodeResult(t, rec)
	if !again.Correct || again.Word == nil || *again.Word != *first.Word || again.Answer != "walked" {
		t.Fatalf("повтор: %+v / %+v", again, first)
	}
	if len(answerEvents(e)) != 1 {
		t.Fatalf("событий %d", len(answerEvents(e)))
	}
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, "walked", "k2"), withCookie(token)); rec.Code != http.StatusConflict {
		t.Fatalf("другой ключ: %d", rec.Code)
	}
}

func TestNextFormKeepsOnlyFrequentDistractors(t *testing.T) {
	e, token := formEnv(t)
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "stop", Pos: "verb"}}}
	e.store.examples = formExamples(7, "He stopped there yesterday.")
	knownAll(e, "A1", "he", "stopped", "there")
	freqOf(e, "stopped", "stopping", "stop") // "stops" и "stoped" в списке нет
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Kind != api.Form {
		t.Fatalf("упражнение: %+v", ex)
	}
	for _, c := range ex.Choices {
		if !slices.Contains([]string{"stopped", "stopping", "stop"}, c) {
			t.Fatalf("вариант %q не из списка частот: %v", c, ex.Choices)
		}
	}
	if e.store.freqCalls != 1 {
		t.Fatalf("запросов частот %d, ждали 1 на слово", e.store.freqCalls)
	}
}

func TestNextFormSkipsWordWithFewFrequentDistractors(t *testing.T) {
	e, token := formEnv(t)
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": {{ID: 1, Lemma: "stop", Pos: "verb"}}}
	e.store.examples = formExamples(7, "He stopped there yesterday.")
	knownAll(e, "A1", "he", "stopped", "there")
	freqOf(e, "stopped", "stop") // один подходящий вариант: мало
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}
