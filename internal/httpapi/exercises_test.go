package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// newExerciseEnv — как newEnv, но случайность фиксирована (всегда 0): выбор предложений детерминирован.
func newExerciseEnv(t *testing.T) env {
	t.Helper()
	store := newFakeStore(t)
	srv := newAuthServer(store, auth.NewThrottle(5, time.Minute, time.Now))
	srv.rnd = func(int64) int64 { return 0 }
	h := api.HandlerWithOptions(srv, api.StdHTTPServerOptions{
		BaseRouter:  http.NewServeMux(),
		Middlewares: []api.MiddlewareFunc{checkOrigin},
	})
	return env{h: h, store: store}
}

func str(s string) *string { return &s }

func lvl(level string) db.SentenceLevelsRow { return db.SentenceLevelsRow{Level: str(level)} }

// knownWords помечает слова предложения как A1.
func knownWords(e env, words ...string) {
	if e.store.wordLevels == nil {
		e.store.wordLevels = map[string]db.SentenceLevelsRow{}
	}
	for _, w := range words {
		e.store.wordLevels[w] = lvl("A1")
	}
}

func poolSentence(id int64, text string) db.SampleSentencesRow {
	return db.SampleSentencesRow{ID: id, TatoebaID: id + 1000, Text: text, Author: "a", WordCount: 5, RuID: id + 5000, RuText: "Перевод"}
}

func getNext(t *testing.T, e env, token string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(http.MethodGet, "/api/v1/exercises/next", "", withCookie(token))
}

func decodeExercise(t *testing.T, rec *httptest.ResponseRecorder) api.Exercise {
	t.Helper()
	var ex api.Exercise
	if err := json.Unmarshal(rec.Body.Bytes(), &ex); err != nil {
		t.Fatalf("тело %q: %v", rec.Body.String(), err)
	}
	return ex
}

func TestNextExerciseGrammarTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a", "dog")

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if !strings.Contains(ex.Prompt, "_____") || ex.Kind != api.Gap || ex.Topic != "grammar:articles" || ex.Translation == "" || len(ex.Choices) != 3 {
		t.Fatalf("упражнение: %+v", ex)
	}
	if !slices.Contains(ex.Choices, "a") {
		t.Fatalf("среди вариантов нет верного: %v", ex.Choices)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"answer", "answers", "correct"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("в ответе есть поле %q: %s", k, rec.Body)
		}
	}
	if strings.Contains(rec.Body.String(), "have a dog") {
		t.Fatalf("в ответе исходное предложение: %s", rec.Body)
	}
	stored, ok := e.store.exercises[ex.Id]
	if !ok {
		t.Fatalf("записи exercises нет: %+v", e.store.exercises)
	}
	if stored.SentenceID == nil || *stored.SentenceID != 10 || stored.Kind != "gap" || !slices.Equal(stored.Topics, []string{"grammar:articles", "level:A1"}) {
		t.Fatalf("запись: %+v", stored)
	}
	var ans struct{ Answers []string }
	if err := json.Unmarshal(stored.Answer, &ans); err != nil || !slices.Equal(ans.Answers, []string{"a"}) {
		t.Fatalf("ответ в записи: %s (%v)", stored.Answer, err)
	}
}

func TestNextExerciseNoCandidates(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExercise {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(e.store.exercises) != 0 {
		t.Fatal("запись создана без упражнения")
	}
}

func rated(topic string, mu float64, attempts int32) db.TopicRating {
	return db.TopicRating{UserID: 1, Topic: topic, Rating: mu, Deviation: 100, Volatility: 0.06, Attempts: attempts}
}

func TestNextExerciseWeakestFirst(t *testing.T) {
	for _, weak := range []string{"grammar:prepositions", "grammar:articles"} {
		t.Run(weak, func(t *testing.T) {
			e := newExerciseEnv(t)
			token := login(t, e)
			e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We live in a small house.")}
			knownWords(e, "we", "live", "in", "a", "small", "house")
			e.store.topicRatings = map[topicKey]db.TopicRating{
				topicKey{1, orderTopic}: rated(orderTopic, 1900, 3), topicKey{1, formTopic}: rated(formTopic, 1900, 3), // порядок слов, выбор формы и перевод не мешают
				topicKey{1, translateTopic}: rated(translateTopic, 1900, 3),
			}
			for _, tp := range []string{"grammar:articles", "grammar:prepositions", "grammar:future", "grammar:modals", "grammar:past_simple"} {
				mu := 1700.0
				if tp == weak {
					mu = 1300
				}
				e.store.topicRatings[topicKey{1, tp}] = rated(tp, mu, 3)
			}
			rec := getNext(t, e, token)
			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
			}
			if ex := decodeExercise(t, rec); ex.Topic != weak {
				t.Fatalf("тема %q, ждали %q", ex.Topic, weak)
			}
		})
	}
}

func TestNextExerciseSkipsTopicWithoutCandidates(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	// Слабейшая тема (модальные) в предложении не встречается: берётся следующая по слабости.
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a", "dog")
	e.store.topicRatings = map[topicKey]db.TopicRating{ // порядок слов, выбор формы и сопоставление не мешают
		{userID: 1, topic: orderTopic}: rated(orderTopic, 1900, 3), {userID: 1, topic: formTopic}: rated(formTopic, 1900, 3),
		{userID: 1, topic: matchTopic}: rated(matchTopic, 1900, 3), {userID: 1, topic: translateTopic}: rated(translateTopic, 1900, 3),
	}
	for tp, mu := range map[string]float64{"grammar:modals": 1000, "grammar:future": 1100, "grammar:past_simple": 1200,
		"grammar:prepositions": 1300, "grammar:articles": 1400} {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, mu, 3)
	}
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if ex := decodeExercise(t, rec); ex.Topic != "grammar:articles" {
		t.Fatalf("тема %q", ex.Topic)
	}
}

func TestNextExerciseRequiresLogin(t *testing.T) {
	e := newExerciseEnv(t)
	for _, path := range []string{"/api/v1/exercises/next", "/api/v1/exercises/topics"} {
		if rec := e.do(http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: статус %d, ждали 401", path, rec.Code)
		}
	}
}

func TestNextExerciseNoRepeatAfterAnswer(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a", "dog")
	first := getNext(t, e, token)
	if first.Code != http.StatusOK {
		t.Fatalf("первый: статус %d", first.Code)
	}
	// Обновление страницы: упражнение не отвечено, предложение не сгорело.
	if rec := getNext(t, e, token); rec.Code != http.StatusOK {
		t.Fatalf("после обновления: статус %d, тело %s", rec.Code, rec.Body)
	}
	// Ответ на все выданные: единственное предложение использовано, в других темах его нет.
	for id, ex := range e.store.exercises {
		ex.AnsweredAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		e.store.exercises[id] = ex
	}
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("после ответа: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestNextExerciseRejectsUnleveledSentence(t *testing.T) {
	tests := map[string]struct {
		zorb   db.SentenceLevelsRow
		wantOK bool
	}{
		"нет в словаре и в частотах": {db.SentenceLevelsRow{}, false},
		"редкое слово":               {db.SentenceLevelsRow{Rank: func() *int32 { r := int32(50000); return &r }()}, false},
		"частое слово без уровня":    {db.SentenceLevelsRow{Rank: func() *int32 { r := int32(100); return &r }()}, true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newExerciseEnv(t)
			token := login(t, e)
			e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a zorb.")}
			knownWords(e, "i", "have", "a")
			e.store.wordLevels["zorb"] = tt.zorb
			rec := getNext(t, e, token)
			if tt.wantOK {
				if rec.Code != http.StatusOK {
					t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusNotFound {
				t.Fatalf("статус %d, ждали 404", rec.Code)
			}
		})
	}
}

func TestNextExerciseSentenceLevelIsMax(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a")
	e.store.wordLevels["dog"] = lvl("B2")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d", rec.Code)
	}
	if got := e.store.exercises[decodeExercise(t, rec).Id].Topics; !slices.Contains(got, "level:B2") {
		t.Fatalf("темы %v, ждали level:B2", got)
	}
}

func TestNextExercisePastSimple(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We walked home together.")}
	knownWords(e, "we", "walked", "home", "together", "walk")
	e.store.topicRatings = map[topicKey]db.TopicRating{ // порядок слов и выбор формы не мешают
		{userID: 1, topic: orderTopic}: rated(orderTopic, 1900, 3), {userID: 1, topic: formTopic}: rated(formTopic, 1900, 3), {userID: 1, topic: translateTopic}: rated(translateTopic, 1900, 3),
	}
	for _, tp := range []string{"grammar:articles", "grammar:prepositions", "grammar:future", "grammar:modals"} {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 3)
	}
	e.store.topicRatings[topicKey{1, "grammar:past_simple"}] = rated("grammar:past_simple", 1300, 3)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Topic != "grammar:past_simple" || ex.Prompt != "We _____ home together." {
		t.Fatalf("упражнение: %+v", ex)
	}
	for _, want := range []string{"walked", "walk"} {
		if !slices.Contains(ex.Choices, want) {
			t.Fatalf("варианты %v без %q", ex.Choices, want)
		}
	}
}

func TestNextExerciseLevelTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A2": {
		{ID: 1, Lemma: "dog", Pos: "noun"}, {ID: 2, Lemma: "cat", Pos: "noun"}, {ID: 3, Lemma: "house", Pos: "noun"},
		{ID: 4, Lemma: "table", Pos: "noun"}, {ID: 5, Lemma: "book", Pos: "noun"},
	}}
	e.store.examples = []db.ListExamplesRow{{EnID: 7, EnTatoebaID: 77, EnText: "The dog is very big.", EnAuthor: "a", RuTatoebaID: 78, RuText: "Собака очень большая.", RuAuthor: "b"}}
	knownWords(e, "the", "dog", "is", "very", "big")
	e.store.topicRatings = map[topicKey]db.TopicRating{}
	for _, tp := range []string{"grammar:articles", "grammar:prepositions", "grammar:future", "grammar:modals", "grammar:past_simple", "level:B1"} {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 3)
	}
	e.store.topicRatings[topicKey{1, "level:A2"}] = rated("level:A2", 1300, 3)

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Topic != "level:A2" || ex.Prompt != "The _____ is very big." || !slices.Contains(ex.Choices, "dog") {
		t.Fatalf("упражнение: %+v", ex)
	}
	for _, c := range ex.Choices {
		if !slices.Contains([]string{"dog", "cat", "house", "table", "book"}, c) {
			t.Fatalf("лишний вариант %q", c)
		}
	}
	stored := e.store.exercises[ex.Id]
	if stored.SentenceID == nil || *stored.SentenceID != 7 || !slices.Equal(stored.Topics, []string{"level:A2", "level:A1", "pos:noun"}) {
		t.Fatalf("запись: %+v", stored)
	}
}

func TestNextExerciseLevelTopicNeedsThreeDistractors(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A2": {
		{ID: 1, Lemma: "dog", Pos: "noun"}, {ID: 2, Lemma: "cat", Pos: "noun"}, {ID: 3, Lemma: "run", Pos: "verb"},
	}}
	e.store.examples = []db.ListExamplesRow{{EnID: 7, EnText: "The dog is very big.", RuText: "x"}}
	knownWords(e, "the", "dog", "is", "very", "big")
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ждали 404", rec.Code)
	}
}

func TestTopicsEmpty(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, "/api/v1/exercises/topics", "", withCookie(token))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"topics":[]}` {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestTopicsList(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.topicRatings = map[topicKey]db.TopicRating{
		{1, "grammar:modals"}: {UserID: 1, Topic: "grammar:modals", Rating: 1420.5, Deviation: 210, Attempts: 4, UpdatedAt: pgtype.Timestamptz{}},
	}
	rec := e.do(http.MethodGet, "/api/v1/exercises/topics", "", withCookie(token))
	var got api.TopicList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s (%v)", rec.Code, rec.Body, err)
	}
	want := api.Rating{Topic: "grammar:modals", Rating: 1420.5, Deviation: 210, Attempts: 4}
	if len(got.Topics) != 1 || got.Topics[0] != want {
		t.Fatalf("темы: %+v", got.Topics)
	}
}

func TestNextExerciseApostropheIsNeutral(t *testing.T) {
	t.Run("не отбрасывается из-за tom's", func(t *testing.T) {
		e := newExerciseEnv(t)
		token := login(t, e)
		e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "Tom's dog is in a house.")}
		knownWords(e, "dog", "is", "in", "a", "house")
		if rec := getNext(t, e, token); rec.Code != http.StatusOK {
			t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
		}
	})
	t.Run("отбрасывается из-за редкого слова", func(t *testing.T) {
		e := newExerciseEnv(t)
		token := login(t, e)
		e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "Tom's zorb is in a house.")}
		knownWords(e, "is", "in", "a", "house")
		if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
			t.Fatalf("статус %d, ждали 404", rec.Code)
		}
	})
}

func TestNextExerciseQueryBudget(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	// 30 подходящих по теме предложений, но с редким словом: все отбрасываются одной проверкой уровней на тему.
	for i := int64(1); i <= 30; i++ {
		e.store.sentencePool = append(e.store.sentencePool, poolSentence(i, "I have a zorb in the house."))
	}
	knownWords(e, "i", "have", "a", "in", "the", "house")
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ждали 404", rec.Code)
	}
	if e.store.rangeCalls != 1 {
		t.Errorf("SentenceIDRange вызван %d раз, ждали 1", e.store.rangeCalls)
	}
	// Пять тем, в каждой не больше одной проверки уровней на окно кандидатов.
	if e.store.levelCalls > 5 {
		t.Errorf("SentenceLevels вызван %d раз, ждали не больше 5", e.store.levelCalls)
	}
	if e.store.levelCalls == 0 {
		t.Error("проверка уровней не вызывалась")
	}
}

func TestNextExerciseRetriesFromZeroOnEmptyWindow(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	e.store.rangeOverride = &db.SentenceIDRangeRow{MinID: 50, MaxID: 60} // окно с 50 пусто
	knownWords(e, "i", "have", "a", "dog")
	if rec := getNext(t, e, token); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestNextExerciseStoreError(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	e.store.sampleErr = errors.New("база недоступна")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeInternal {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "недоступна") {
		t.Fatal("текст ошибки базы утёк клиенту")
	}
}

func TestNextExerciseLevelTopicRejectsHarderSentence(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A2": {
		{ID: 1, Lemma: "dog", Pos: "noun"}, {ID: 2, Lemma: "cat", Pos: "noun"}, {ID: 3, Lemma: "house", Pos: "noun"},
		{ID: 4, Lemma: "table", Pos: "noun"}, {ID: 5, Lemma: "book", Pos: "noun"},
	}}
	e.store.examples = []db.ListExamplesRow{{EnID: 7, EnText: "The dog is very big.", RuText: "x"}}
	knownWords(e, "the", "is", "very", "big")
	e.store.wordLevels["dog"] = lvl("B1") // уровень предложения B1 выше темы A2
	e.store.topicRatings = map[topicKey]db.TopicRating{}
	for _, tp := range []string{"grammar:articles", "grammar:prepositions", "grammar:future", "grammar:modals", "grammar:past_simple", "level:B1"} {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 3)
	}
	e.store.topicRatings[topicKey{1, "level:A2"}] = rated("level:A2", 1300, 3)
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

// Перед словом-целью стоит «an»: отвлекающие тоже на гласную, иначе артикль выдаёт ответ.
func TestNextExerciseLevelTopicArticleAgreement(t *testing.T) {
	setup := func(lemmas ...string) (env, string) {
		e := newExerciseEnv(t)
		token := login(t, e)
		e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A2", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
		var rows []db.RandomLexemesByLevelRow
		for i, l := range lemmas {
			rows = append(rows, db.RandomLexemesByLevelRow{ID: int64(i + 1), Lemma: l, Pos: "noun"})
		}
		e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A2": rows}
		e.store.examples = []db.ListExamplesRow{{EnID: 7, EnText: "She ate an apple.", RuText: "Она съела яблоко."}}
		knownWords(e, "she", "ate", "an", "apple")
		e.store.topicRatings = map[topicKey]db.TopicRating{}
		for _, tp := range []string{"grammar:articles", "grammar:prepositions", "grammar:future", "grammar:modals", "grammar:past_simple", "level:B1"} {
			e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 3)
		}
		e.store.topicRatings[topicKey{1, "level:A2"}] = rated("level:A2", 1300, 3)
		return e, token
	}
	e, token := setup("apple", "orange", "egg", "ear", "table", "dog")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	for _, c := range ex.Choices {
		if !strings.ContainsRune("aeiou", rune(c[0])) {
			t.Errorf("вариант %q не на гласную после an: %v", c, ex.Choices)
		}
	}
	// Гласных отвлекающих всего два: предложение пропускается, упражнения нет.
	e, token = setup("apple", "orange", "egg", "table", "dog")
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ждали 404", rec.Code)
	}
}

// Пул не пуст, но ни одно предложение не даёт темы: 404 no_exercise, а не сбой.
func TestNextExercisePoolWithoutTopics(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "Dogs run fast."), poolSentence(11, "Birds sing loud.")}
	knownWords(e, "dogs", "run", "fast", "birds", "sing", "loud")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExercise {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(e.store.exercises) != 0 {
		t.Fatal("запись создана без упражнения")
	}
}

// «Mr.» с точкой в середине предложения не мешает собрать упражнение.
func TestNextExerciseSentenceWithMr(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "Mr. Smith has a dog.")}
	knownWords(e, "mr", "smith", "has", "a", "dog")
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if ex := decodeExercise(t, rec); ex.Prompt != "Mr. Smith has _____ dog." {
		t.Fatalf("упражнение: %+v", ex)
	}
}
