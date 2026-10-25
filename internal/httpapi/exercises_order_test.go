package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// orderFirst выставляет рейтинги так, чтобы skill:order (без попыток) выбиралась первой.
func orderFirst(e env) {
	e.store.topicRatings = map[topicKey]db.TopicRating{}
	for _, tp := range grammarTopics {
		e.store.topicRatings[topicKey{1, tp}] = rated(tp, 1700, 5)
	}
}

func knownAll(e env, level string, ws ...string) {
	if e.store.wordLevels == nil {
		e.store.wordLevels = map[string]db.SentenceLevelsRow{}
	}
	for _, w := range ws {
		e.store.wordLevels[w] = lvl(level)
	}
}

func TestNextOrderTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We eat red apples every morning.")}
	knownAll(e, "A1", "we", "eat", "red", "apples", "every", "morning")

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Order || ex.Topic != "skill:order" || ex.Prompt != "Перевод" || len(ex.Choices) != 0 || ex.Tiles == nil || len(*ex.Tiles) != 6 {
		t.Fatalf("упражнение: %+v", ex)
	}
	want := []string{"we", "eat", "red", "apples", "every", "morning"}
	if slices.Equal(*ex.Tiles, want) {
		t.Fatalf("плитки в исходном порядке: %v", *ex.Tiles)
	}
	got := slices.Clone(*ex.Tiles)
	slices.Sort(got)
	sorted := slices.Clone(want)
	slices.Sort(sorted)
	if !slices.Equal(got, sorted) {
		t.Fatalf("плитки не из слов предложения: %v", *ex.Tiles)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"answer", "answers", "correct"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("в ответе есть поле %q: %s", k, rec.Body)
		}
	}
	if strings.Contains(rec.Body.String(), "We eat red apples") {
		t.Fatalf("в ответе исходное предложение: %s", rec.Body)
	}
	stored := e.store.exercises[ex.Id]
	if stored.Kind != "order" || stored.SentenceID == nil || *stored.SentenceID != 10 || !slices.Equal(stored.Topics, []string{"skill:order", "level:A1"}) {
		t.Fatalf("запись: %+v", stored)
	}
	var ans exerciseAnswer
	if err := json.Unmarshal(stored.Answer, &ans); err != nil || ans.Answers[0] != "We eat red apples every morning." {
		t.Fatalf("ответ в записи: %s (%v)", stored.Answer, err)
	}
}

func TestNextOrderUsesAlternatives(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We eat red apples every morning.")}
	knownAll(e, "A1", "we", "eat", "red", "apples", "every", "morning")
	e.store.addPair(10, 5010, "We eat red apples every morning.", 6)
	e.store.addPair(11, 5010, "Every morning we eat red apples.", 6)
	e.store.addPair(12, 5010, "We eat green pears every morning.", 6) // другой состав: отбрасывается

	ex := decodeExercise(t, getNext(t, e, token))
	var ans exerciseAnswer
	_ = json.Unmarshal(e.store.exercises[ex.Id].Answer, &ans)
	if !slices.Contains(ans.Answers, "Every morning we eat red apples.") || slices.Contains(ans.Answers, "We eat green pears every morning.") {
		t.Fatalf("допустимые ответы: %v", ans.Answers)
	}
}

func TestNextOrderFallsThroughToNextTopic(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	// Для порядка слов не годится (имя Tom не в начале), для пропуска годится.
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I met Tom in a park.")}
	knownAll(e, "A1", "i", "met", "tom", "in", "a", "park")

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if ex := decodeExercise(t, rec); ex.Kind != api.Gap {
		t.Fatalf("ожидали пропуск после отказа порядка слов: %+v", ex)
	}
}

func TestNextOrderNoCandidates(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	// Короткое предложение без грамматических тем: ни порядок слов, ни пропуск не собираются.
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "Run now.")}
	rec := getNext(t, e, token)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExercise {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestNextOrderLevelCap(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B2", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We eat red apples every morning.")}
	knownAll(e, "B2", "we", "eat", "red", "apples", "every", "morning") // выше «уровень пользователя + 1» (A2)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("предложение уровня B2 для A1 не должно выдаваться: %d %s", rec.Code, rec.Body)
	}
	// Уровень A2 допустим (пользователь A1 + 1).
	knownAll(e, "A2", "we", "eat", "red", "apples", "every", "morning")
	if rec := getNext(t, e, token); rec.Code != http.StatusOK {
		t.Fatalf("A2 для A1 допустим: %d %s", rec.Code, rec.Body)
	}
}

func TestNextOrderNoRepeat(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	orderFirst(e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "We eat red apples every morning.")}
	knownAll(e, "A1", "we", "eat", "red", "apples", "every", "morning")
	ex := decodeExercise(t, getNext(t, e, token))
	// Пока не отвечено, предложение выдаётся снова (обновление страницы не сжигает его).
	if rec := getNext(t, e, token); rec.Code != http.StatusOK {
		t.Fatalf("повторная выдача неотвеченного: %d", rec.Code)
	}
	stored := e.store.exercises[ex.Id]
	stored.AnsweredAt = pgtype.Timestamptz{Time: stored.CreatedAt.Time, Valid: true}
	e.store.exercises[ex.Id] = stored
	for id, x := range e.store.exercises { // все выданные упражнения этого предложения помечаем отвеченными
		x.AnsweredAt = stored.AnsweredAt
		e.store.exercises[id] = x
	}
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("после ответа предложение не должно повторяться: %d %s", rec.Code, rec.Body)
	}
}

// seedOrderExercise кладёт упражнение order с двумя допустимыми ответами.
func seedOrderExercise(t *testing.T, e env, userID int64, level string) int64 {
	t.Helper()
	topics := []string{"skill:order"}
	if level != "" {
		topics = append(topics, "level:"+level)
	}
	tiles := []string{"morning", "we", "red", "every", "eat", "apples"}
	payload, _ := json.Marshal(exercisePayload{Prompt: "Мы едим красные яблоки каждое утро.", Translation: "Мы едим красные яблоки каждое утро.", Tiles: tiles, Topic: "skill:order"})
	answer, _ := json.Marshal(exerciseAnswer{Answers: []string{"We eat red apples every morning.", "Every morning we eat red apples."}})
	id, err := e.store.InsertExercise(t.Context(), db.InsertExerciseParams{
		UserID: userID, Kind: "order", SentenceID: i64(1), Topics: topics, Payload: payload, Answer: answer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAnswerOrder(t *testing.T) {
	cases := []struct {
		name, given string
		correct     bool
	}{
		{"верный", "We eat red apples every morning.", true},
		{"альтернативный порядок", "Every morning we eat red apples.", true},
		{"лишние пробелы, регистр и без точки", "  every  morning we eat red apples ", true},
		{"неверный порядок", "We red eat apples every morning.", false},
		{"не все слова", "We eat red apples.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newExerciseEnv(t)
			token := login(t, e)
			id := seedOrderExercise(t, e, 1, "A1")
			rec := e.do(http.MethodPost, answerPath, answerBody(id, c.given, "k1"), withCookie(token))
			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
			}
			res := decodeResult(t, rec)
			if res.Correct != c.correct || res.Answer != "We eat red apples every morning." {
				t.Fatalf("результат: %+v", res)
			}
			skill, ok := ratingOf(res, "skill:order")
			if !ok || skill.Attempts != 1 || (skill.Rating > 1500) != c.correct {
				t.Fatalf("рейтинг skill:order: %+v", res.Ratings)
			}
			if _, ok := ratingOf(res, "level:A1"); !ok {
				t.Fatalf("нет рейтинга level:A1: %+v", res.Ratings)
			}
			evs := answerEvents(e)
			var ev struct {
				Kind  string `json:"kind"`
				Given string `json:"given"`
			}
			if len(evs) != 1 || json.Unmarshal(evs[0].Payload, &ev) != nil || ev.Kind != "order" || ev.Given != c.given {
				t.Fatalf("событие: %+v", evs)
			}
		})
	}
}
