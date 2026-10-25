package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// translateFirst выставляет рейтинги так, чтобы skill:translate (без попыток) выбиралась первой.
func translateFirst(e env) {
	orderFirst(e)
	delete(e.store.topicRatings, topicKey{1, translateTopic})
	for _, tp := range []string{orderTopic, formTopic, matchTopic} {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 5)
	}
}

const translateSentence = "We eat red apples every morning."

var translateWords = []string{"we", "eat", "red", "apples", "every", "morning"}

// seedTranslate кладёт в фейк упражнение «перевод» с допустимыми ответами answers.
func seedTranslate(t *testing.T, e env, userID int64, answers ...string) int64 {
	t.Helper()
	payload, _ := json.Marshal(exercisePayload{Prompt: "Мы едим красные яблоки каждое утро.", Choices: []string{}, Topic: translateTopic})
	answer, _ := json.Marshal(exerciseAnswer{Answers: answers})
	id, err := e.store.InsertExercise(t.Context(), db.InsertExerciseParams{
		UserID: userID, Kind: "translate", SentenceID: i64(1), Topics: []string{translateTopic, "level:A1"}, Payload: payload, Answer: answer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func answerTranslate(t *testing.T, e env, token string, id int64, given, key string) api.AnswerResult {
	t.Helper()
	rec := e.do(http.MethodPost, answerPath, answerBody(id, given, key), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	return decodeResult(t, rec)
}

func TestNextTranslateTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	translateFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, translateSentence)}
	knownAll(e, "A1", translateWords...)

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Translate || ex.Topic != translateTopic || ex.Prompt != "Перевод" || len(ex.Choices) != 0 || ex.Tiles != nil {
		t.Fatalf("упражнение: %+v", ex)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"answer", "answers", "correct", "accepted", "nearMiss"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("в ответе есть поле %q: %s", k, rec.Body)
		}
	}
	if strings.Contains(rec.Body.String(), "red apples") {
		t.Fatalf("в ответе английское предложение: %s", rec.Body)
	}
	stored := e.store.exercises[ex.Id]
	if stored.Kind != "translate" || stored.SentenceID == nil || *stored.SentenceID != 10 || !slices.Equal(stored.Topics, []string{translateTopic}) {
		t.Fatalf("запись: %+v", stored)
	}
	var ans exerciseAnswer
	if err := json.Unmarshal(stored.Answer, &ans); err != nil || ans.Answers[0] != translateSentence {
		t.Fatalf("ответ в записи: %s (%v)", stored.Answer, err)
	}
}

func TestNextTranslateSkipsUnsuitable(t *testing.T) {
	for _, text := range []string{`We said "hello" every morning.`, "We eat red apples in the big house every morning now."} {
		e := newExerciseEnv(t)
		token := login(t, e)
		translateFirst(e)
		e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, text)}
		knownAll(e, "A1", "we", "said", "hello", "every", "morning", "eat", "red", "apples", "in", "the", "big", "house", "now")
		// порядок слов и остальные виды выключены тем, что для них нет данных; остаётся 404
		rec := getNext(t, e, token)
		if rec.Code == http.StatusOK {
			if ex := decodeExercise(t, rec); ex.Kind == api.Translate {
				t.Fatalf("предложение %q не должно выдаваться как перевод: %+v", text, ex)
			}
		}
	}
}

func TestAnswerTranslateExact(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence)
	res := answerTranslate(t, e, token, id, "  we EAT red apples every morning ", "k1")
	if !res.Correct || res.NearMiss != nil || res.Answer != translateSentence {
		t.Fatalf("результат: %+v", res)
	}
	if res.Accepted == nil || len(*res.Accepted) != 0 {
		t.Fatalf("accepted должен быть пустым списком, а не null: %+v", res.Accepted)
	}
	if r, ok := ratingOf(res, translateTopic); !ok || r.Rating <= 1500 || r.Attempts != 1 {
		t.Fatalf("рейтинг: %+v", res.Ratings)
	}
	evs := answerEvents(e)
	if len(evs) != 1 || !strings.Contains(string(evs[0].Payload), `"correct":true`) {
		t.Fatalf("события: %+v", evs)
	}
	if len(e.store.mistakes) != 0 {
		t.Fatalf("верный ответ открыл ошибку: %+v", e.store.mistakes)
	}
}

func TestAnswerTranslateAlternative(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence, "Every morning we eat red apples.", "In the morning we eat red apples.", "Red apples are what we eat every morning.", "We have red apples every morning.")
	res := answerTranslate(t, e, token, id, "Every morning we eat red apples", "k1")
	if !res.Correct || res.Answer != translateSentence || res.NearMiss != nil {
		t.Fatalf("результат: %+v", res)
	}
	// до трёх других допустимых переводов: без основного и без того, что ввёл пользователь
	want := []string{"In the morning we eat red apples.", "Red apples are what we eat every morning.", "We have red apples every morning."}
	if res.Accepted == nil || !slices.Equal(*res.Accepted, want) {
		t.Fatalf("accepted %v, ждали %v", res.Accepted, want)
	}
}

func TestAnswerTranslateTypoKeepsStreakAndOpensNothing(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence)
	res := answerTranslate(t, e, token, id, "we eat red aples every morning", "k1")
	if !res.Correct || res.NearMiss == nil || *res.NearMiss != translateSentence {
		t.Fatalf("результат: %+v", res)
	}
	if r, ok := ratingOf(res, translateTopic); !ok || r.Rating <= 1500 {
		t.Fatalf("опечатка засчитывается как верный ответ: %+v", res.Ratings)
	}
	if len(e.store.mistakes) != 0 || len(mistakeEvents(e, "mistake.opened")) != 0 || res.Mistake != nil {
		t.Fatalf("опечатка открыла ошибку: %+v", e.store.mistakes)
	}
	if evs := answerEvents(e); len(evs) != 1 || !strings.Contains(string(evs[0].Payload), `"nearMiss"`) {
		t.Fatalf("событие должно хранить nearMiss: %+v", evs)
	}

	// повтор запроса по ключу: тот же результат, рейтинг не двигается, событие одно
	again := answerTranslate(t, e, token, id, "we eat red aples every morning", "k1")
	if !again.Correct || again.NearMiss == nil || *again.NearMiss != translateSentence {
		t.Fatalf("повтор по ключу: %+v", again)
	}
	if r1, _ := ratingOf(res, translateTopic); func() bool {
		r2, _ := ratingOf(again, translateTopic)
		return r1.Rating != r2.Rating || r2.Attempts != 1
	}() {
		t.Fatalf("повтор по ключу сдвинул рейтинг: %+v → %+v", res.Ratings, again.Ratings)
	}
	if len(answerEvents(e)) != 1 {
		t.Fatalf("событий %d, ждали 1", len(answerEvents(e)))
	}
}

func TestAnswerTranslateWrongOpensMistake(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence, "Every morning we eat red apples.")
	res := answerTranslate(t, e, token, id, "they drink green tea", "k1")
	if res.Correct || res.Answer != translateSentence || res.NearMiss != nil {
		t.Fatalf("результат: %+v", res)
	}
	if res.Accepted == nil || !slices.Equal(*res.Accepted, []string{"Every morning we eat red apples."}) {
		t.Fatalf("при ошибке показываются другие допустимые переводы: %+v", res.Accepted)
	}
	m := onlyMistake(t, e)
	if m.Kind != "translate" || m.Given != "they drink green tea" || m.Status != "open" || res.Mistake == nil || res.Mistake.Status != api.Open {
		t.Fatalf("ошибка: %+v / %+v", m, res.Mistake)
	}
	if len(mistakeEvents(e, "mistake.opened")) != 1 {
		t.Fatal("нет события mistake.opened")
	}
}

func TestAnswerTranslateTooLongIsWrong(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence)
	res := answerTranslate(t, e, token, id, strings.Repeat("a", 201), "k1")
	if res.Correct {
		t.Fatalf("слишком длинный ответ засчитан: %+v", res)
	}
}

func TestRepeatTranslateMistake(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 7, time.Now().Add(-time.Minute), 0)
	m := e.store.mistakes[mid]
	m.Kind = "translate"
	m.Topics = []string{translateTopic, "level:A1"}
	m.Payload, _ = json.Marshal(exercisePayload{Prompt: "Мы едим красные яблоки каждое утро.", Choices: []string{}, Topic: translateTopic})
	m.Answer, _ = json.Marshal(exerciseAnswer{Answers: []string{translateSentence, "Every morning we eat red apples."}})
	e.store.mistakes[mid] = m

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Translate || ex.Repeat == nil || !*ex.Repeat || ex.Prompt != "Мы едим красные яблоки каждое утро." || len(ex.Choices) != 0 || ex.Tiles != nil {
		t.Fatalf("упражнение: %+v", ex)
	}
	stored := e.store.exercises[ex.Id]
	if stored.MistakeID == nil || *stored.MistakeID != mid || stored.Kind != "translate" {
		t.Fatalf("строка exercises: %+v", stored)
	}
	if strings.Contains(rec.Body.String(), "red apples") {
		t.Fatalf("в теле есть ответ: %s", rec.Body)
	}

	// верный повтор двигает серию ошибки; вторая верная подряд закрывает её
	res := answerTranslate(t, e, token, ex.Id, "Every morning we eat red apples", "k1")
	if !res.Correct || res.Mistake == nil || res.Mistake.Streak != 1 {
		t.Fatalf("после первого верного повтора: %+v", res.Mistake)
	}
}

// Верны переводы любой длины: «I said I'd try» и «I said I would try» — один смысл (в отличие от «порядка слов»).
func TestNextTranslateAcceptsAlternativeOfOtherLength(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	translateFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I said I'd try.")}
	e.store.addPair(10, 5010, "I said I'd try.", 4)
	e.store.addPair(11, 5010, "I said I would try.", 5)
	knownAll(e, "A1", "i", "said", "d", "i'd", "would", "try")

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Translate {
		t.Fatalf("упражнение: %+v", ex)
	}
	var ans exerciseAnswer
	if err := json.Unmarshal(e.store.exercises[ex.Id].Answer, &ans); err != nil || !slices.Equal(ans.Answers, []string{"I said I'd try.", "I said I would try."}) {
		t.Fatalf("допустимые ответы: %s (%v)", e.store.exercises[ex.Id].Answer, err)
	}
	res := answerTranslate(t, e, token, ex.Id, "i said i would try", "k1")
	if !res.Correct || res.NearMiss != nil {
		t.Fatalf("перевод другой длины должен быть верным: %+v", res)
	}
}

// Повтор по ключу после ответа другим допустимым переводом отдаёт тот же список «Так тоже можно»:
// совпавший вариант хранится в событии.
func TestAnswerTranslateReplayKeepsAccepted(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence, "Every morning we eat red apples.", "We have red apples every morning.")
	first := answerTranslate(t, e, token, id, "Every morning we eat red apples", "k1")
	again := answerTranslate(t, e, token, id, "Every morning we eat red apples", "k1")
	if first.Accepted == nil || again.Accepted == nil || !slices.Equal(*first.Accepted, *again.Accepted) {
		t.Fatalf("список изменился при повторе: %v → %v", first.Accepted, again.Accepted)
	}
	if !slices.Equal(*again.Accepted, []string{"We have red apples every morning."}) {
		t.Fatalf("accepted %v: без основного и без совпавшего", *again.Accepted)
	}
}

func TestAnswerTranslateOtherKeyAfterAnswerIsConflict(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedTranslate(t, e, 1, translateSentence)
	answerTranslate(t, e, token, id, "we eat red apples every morning", "k1")
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "they drink tea", "k2"), withCookie(token))
	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeAlreadyAnswered {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(answerEvents(e)) != 1 {
		t.Fatalf("событий %d, ждали 1", len(answerEvents(e)))
	}
}

// Опечатка на повторе ошибки засчитывается как верный ответ: серия ошибки двигается.
func TestRepeatTranslateTypoAdvancesStreak(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 7, time.Now().Add(-time.Minute), 0)
	m := e.store.mistakes[mid]
	m.Kind = "translate"
	m.Topics = []string{translateTopic, "level:A1"}
	m.Payload, _ = json.Marshal(exercisePayload{Prompt: "Мы едим красные яблоки каждое утро.", Choices: []string{}, Topic: translateTopic})
	m.Answer, _ = json.Marshal(exerciseAnswer{Answers: []string{translateSentence}})
	e.store.mistakes[mid] = m

	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Repeat == nil || !*ex.Repeat {
		t.Fatalf("упражнение: %+v", ex)
	}
	res := answerTranslate(t, e, token, ex.Id, "we eat red aples every morning", "k1")
	if !res.Correct || res.NearMiss == nil || res.Mistake == nil || res.Mistake.Streak != 1 {
		t.Fatalf("результат: %+v / %+v", res, res.Mistake)
	}
}

// Контроль к TestNextTranslateSkipsUnsuitable: тот же набор данных с подходящим предложением даёт «перевод», поэтому
// отсутствие «перевода» для плохих предложений — не следствие настройки теста.
func TestNextTranslateControlSentenceIsServed(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	translateFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, translateSentence)}
	knownAll(e, "A1", translateWords...)
	if ex := decodeExercise(t, getNext(t, e, token)); ex.Kind != api.Translate {
		t.Fatalf("упражнение: %+v", ex)
	}
}

// Уровень предложения в темы «перевода» не кладём: ввод труднее выбора, и точность предложения сменить уровень
// (ответы по теме level:<уровень>) от него занижалась бы.
func TestTranslateAnswersDoNotTouchLevelTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	translateFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, translateSentence)}
	knownAll(e, "A1", translateWords...)
	ex := decodeExercise(t, getNext(t, e, token))
	res := answerTranslate(t, e, token, ex.Id, "they drink green tea", "k1")
	for _, r := range res.Ratings {
		if strings.HasPrefix(r.Topic, "level:") {
			t.Fatalf("ответ на translate затронул тему уровня: %+v", res.Ratings)
		}
	}
}

// Предложение, на которое пользователь уже ответил «порядком слов» (или наоборот), для «перевода» не выдаётся:
// собрав плитки, он видел английский ответ.
func TestTranslateAndOrderDoNotShareSentences(t *testing.T) {
	for _, seen := range []string{orderTopic, translateTopic} {
		e := newExerciseEnv(t)
		token := login(t, e)
		e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, translateSentence)}
		knownAll(e, "A1", translateWords...)
		e.store.exercises = map[int64]db.Exercise{1: {ID: 1, UserID: 1, Kind: "order", SentenceID: i64(10), Topics: []string{seen}, AnsweredAt: ts(time.Now())}}
		e.store.nextExerciseID = 1
		other := translateTopic
		if seen == translateTopic {
			other = orderTopic
		}
		// парк остальных тем: выбирается только «другая» тема
		e.store.topicRatings = map[topicKey]db.TopicRating{}
		for _, tp := range append(append([]string{}, grammarTopics...), formTopic, matchTopic) {
			e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1900, 5)
		}
		e.store.topicRatings[topicKey{1, seen}] = rated(seen, 1900, 5)
		e.store.topicRatings[topicKey{1, other}] = rated(other, 100, 5)
		rec := getNext(t, e, token)
		if rec.Code == http.StatusOK {
			if ex := decodeExercise(t, rec); ex.Topic == other {
				t.Fatalf("предложение, виденное в %s, выдано в %s: %+v", seen, other, ex)
			}
		}
	}
}
