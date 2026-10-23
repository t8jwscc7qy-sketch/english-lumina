package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const answerPath = "/api/v1/exercises/answer"

// seedExercise кладёт в фейк упражнение с ответом "a" и темами grammar:articles и level:<level>.
func seedExercise(t *testing.T, e env, userID int64, level string) int64 {
	t.Helper()
	topics := []string{"grammar:articles"}
	if level != "" {
		topics = append(topics, "level:"+level)
	}
	payload, _ := json.Marshal(exercisePayload{Prompt: "I have _____ dog.", Translation: "У меня есть собака.", Choices: []string{"a", "an", "the", "-"}, Topic: "grammar:articles"})
	answer, _ := json.Marshal(exerciseAnswer{Answers: []string{"a"}})
	id, err := e.store.InsertExercise(t.Context(), db.InsertExerciseParams{
		UserID: userID, Kind: "gap", SentenceID: 1, Topics: topics, Payload: payload, Answer: answer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func answerBody(id int64, given, key string) string {
	b, _ := json.Marshal(map[string]any{"exerciseId": id, "given": given, "key": key})
	return string(b)
}

func decodeResult(t *testing.T, rec *httptest.ResponseRecorder) api.AnswerResult {
	t.Helper()
	var r api.AnswerResult
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("тело %q: %v", rec.Body.String(), err)
	}
	return r
}

func answerEvents(e env) []db.Event {
	var out []db.Event
	for _, ev := range e.store.events {
		if ev.Type == "exercise.answered" {
			out = append(out, ev)
		}
	}
	return out
}

func ratingOf(r api.AnswerResult, topic string) (api.Rating, bool) {
	for _, x := range r.Ratings {
		if x.Topic == topic {
			return x, true
		}
	}
	return api.Rating{}, false
}

func TestAnswerRequiresLogin(t *testing.T) {
	e := newExerciseEnv(t)
	if rec := e.do(http.MethodPost, answerPath, answerBody(1, "a", "k")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ждали 401", rec.Code)
	}
}

func TestAnswerCorrectUpdatesRatings(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "B1")

	rec := e.do(http.MethodPost, answerPath, answerBody(id, " A ", "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	res := decodeResult(t, rec)
	if !res.Correct || res.Answer != "a" || res.Translation != "У меня есть собака." || len(res.Ratings) != 2 {
		t.Fatalf("результат: %+v", res)
	}
	for _, topic := range []string{"grammar:articles", "level:B1"} {
		r, ok := ratingOf(res, topic)
		if !ok || r.Rating <= 1500 || r.Attempts != 1 || r.Deviation >= 350 {
			t.Fatalf("рейтинг %s: %+v (есть %v)", topic, r, ok)
		}
		if stored := e.store.topicRatings[topicKey{1, topic}]; stored.Rating != r.Rating || stored.Attempts != 1 {
			t.Fatalf("в хранилище %s: %+v", topic, stored)
		}
	}
	if !e.store.exercises[id].AnsweredAt.Valid {
		t.Fatal("answered_at не записан")
	}
	evs := answerEvents(e)
	if len(evs) != 1 || evs[0].IdempotencyKey == nil || *evs[0].IdempotencyKey != "exercise:1:k1" {
		t.Fatalf("события: %+v", evs)
	}
	var p struct {
		ExerciseID   int64              `json:"exerciseId"`
		Kind         string             `json:"kind"`
		Topics       []string           `json:"topics"`
		Correct      bool               `json:"correct"`
		Given        string             `json:"given"`
		RatingBefore map[string]float64 `json:"ratingBefore"`
		RatingAfter  map[string]float64 `json:"ratingAfter"`
	}
	if err := json.Unmarshal(evs[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.ExerciseID != id || p.Kind != "gap" || !p.Correct || p.Given != " A " || len(p.Topics) != 2 ||
		p.RatingBefore["level:B1"] != 1500 || p.RatingAfter["level:B1"] <= 1500 {
		t.Fatalf("payload: %+v", p)
	}
}

func TestAnswerWrongDropsRatings(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	res := decodeResult(t, rec)
	if res.Correct || res.Answer != "a" {
		t.Fatalf("результат: %+v", res)
	}
	for _, r := range res.Ratings {
		if r.Rating >= 1500 {
			t.Fatalf("рейтинг %s не упал: %+v", r.Topic, r)
		}
	}
	evs := answerEvents(e)
	if len(evs) != 1 || !strings.Contains(string(evs[0].Payload), `"correct":false`) {
		t.Fatalf("события: %+v", evs)
	}
}

func TestAnswerReplayByKey(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A2")
	first := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token))
	upserts := e.store.upserts
	second := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token))
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("статусы %d и %d, тело %s", first.Code, second.Code, second.Body)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("ответы различаются:\n%s\n%s", first.Body, second.Body)
	}
	if e.store.upserts != upserts || len(answerEvents(e)) != 1 || e.store.topicRatings[topicKey{1, "grammar:articles"}].Attempts != 1 {
		t.Fatalf("повтор изменил данные: upserts %d→%d, событий %d", upserts, e.store.upserts, len(answerEvents(e)))
	}
}

func TestAnswerAlreadyAnswered(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A2")
	e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token))
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k2"), withCookie(token))
	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeAlreadyAnswered {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(answerEvents(e)) != 1 || e.store.topicRatings[topicKey{1, "grammar:articles"}].Attempts != 1 {
		t.Fatal("второй ответ изменил данные")
	}
}

func TestAnswerUnknownExercise(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	foreign := seedExercise(t, e, 999, "A1") // упражнение другого пользователя
	for _, id := range []int64{12345, foreign} {
		rec := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k"), withCookie(token))
		if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNotFound {
			t.Fatalf("id %d: статус %d, тело %s", id, rec.Code, rec.Body)
		}
	}
	if e.store.exercises[foreign].AnsweredAt.Valid || len(e.store.events) != 0 {
		t.Fatal("чужое упражнение изменено")
	}
}

func TestAnswerInvalidBody(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	cases := map[string]string{
		"пустой given":         answerBody(id, "", "k"),
		"только пробелы":       answerBody(id, "   ", "k"),
		"201 символ":           answerBody(id, strings.Repeat("я", 201), "k"),
		"нулевой байт":         `{"exerciseId":1,"given":"a\u0000b","key":"k"}`,
		"пустой key":           answerBody(id, "a", ""),
		"key длиннее 64":       answerBody(id, "a", strings.Repeat("k", 65)),
		"exerciseId 0":         answerBody(0, "a", "k"),
		"мусор после JSON":     answerBody(id, "a", "k") + `{"x":1}`,
		"неизвестное поле":     `{"exerciseId":1,"given":"a","key":"k","extra":1}`,
		"не JSON":              `abc`,
		"отрицательный номер":  answerBody(-5, "a", "k"),
		"слишком большое тело": `{"exerciseId":1,"given":"a","key":"k","pad":"` + strings.Repeat("x", 5000) + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := e.do(http.MethodPost, answerPath, body, withCookie(token))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
			}
		})
	}
	// Ровно 200 символов проходит.
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, strings.Repeat("я", 200), "k"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("200 символов: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestAnswerRollsBackOnEventFailure(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "B1")
	e.store.failEvents = true
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "журнал") {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(e.store.topicRatings) != 0 || e.store.exercises[id].AnsweredAt.Valid {
		t.Fatalf("откат не сработал: рейтинги %v, answered_at %v", e.store.topicRatings, e.store.exercises[id].AnsweredAt)
	}
	e.store.failEvents = false
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("повтор после сбоя: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestAnswerDifficultyFromLevel(t *testing.T) {
	// Верный ответ на C2 поднимает рейтинг сильнее, чем на A1.
	gain := func(level string) float64 {
		e := newExerciseEnv(t)
		token := login(t, e)
		id := seedExercise(t, e, 1, level)
		res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(id, "a", "k"), withCookie(token)))
		r, _ := ratingOf(res, "grammar:articles")
		return r.Rating
	}
	if hi, lo := gain("C2"), gain("A1"); hi <= lo {
		t.Fatalf("C2 %.1f, A1 %.1f: ждали, что C2 даст больше", hi, lo)
	}
	// Без ключа level — соперник 1500, и рейтинг новой темы после верного ответа выше 1500.
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "")
	res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(id, "a", "k"), withCookie(token)))
	if len(res.Ratings) != 1 || res.Ratings[0].Rating <= 1500 {
		t.Fatalf("результат: %+v", res)
	}
}
