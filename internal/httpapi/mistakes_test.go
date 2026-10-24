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
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/mistakes"
)

const mistakesPath = "/api/v1/mistakes"

func mistakeEvents(e env, typ string) []db.Event {
	var out []db.Event
	for _, ev := range e.store.events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func onlyMistake(t *testing.T, e env) db.Mistake {
	t.Helper()
	if len(e.store.mistakes) != 1 {
		t.Fatalf("ошибок в банке %d, ждали 1: %+v", len(e.store.mistakes), e.store.mistakes)
	}
	for _, m := range e.store.mistakes {
		return m
	}
	return db.Mistake{}
}

// seedMistake кладёт открытую ошибку «gap» (верный ответ "a") с заданным сроком и серией.
func seedMistake(e env, sentenceID int64, nextDue time.Time, streak int16) int64 {
	payload, _ := json.Marshal(exercisePayload{Prompt: "I have _____ dog.", Translation: "У меня есть собака.", Choices: []string{"a", "an", "the", "-"}, Topic: "grammar:articles"})
	answer, _ := json.Marshal(exerciseAnswer{Answers: []string{"a"}})
	return putMistake(e, db.Mistake{
		UserID: 1, Kind: "gap", SentenceID: sentenceID, Topics: []string{"grammar:articles", "level:A1"},
		Payload: payload, Answer: answer, Given: "the", Streak: streak, NextDue: ts(nextDue),
	})
}

func putMistake(e env, m db.Mistake) int64 {
	if e.store.mistakes == nil {
		e.store.mistakes = map[int64]db.Mistake{}
	}
	e.store.nextMistakeID++
	m.ID, m.Status = e.store.nextMistakeID, "open"
	m.FirstWrongAt = ts(time.Now().Add(-48 * time.Hour))
	e.store.mistakes[m.ID] = m
	return m.ID
}

func TestAnswerWrongOpensMistake(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	before := time.Now()
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	m := onlyMistake(t, e)
	if m.Status != "open" || m.Streak != 0 || m.Given != "the" || m.Kind != "gap" || m.SentenceID != 1 ||
		m.SourceExerciseID == nil || *m.SourceExerciseID != id || !slices.Equal(m.Topics, []string{"grammar:articles", "level:A1"}) {
		t.Fatalf("ошибка: %+v", m)
	}
	if d := m.NextDue.Time.Sub(before); d < mistakes.FirstDelay || d > mistakes.FirstDelay+time.Minute {
		t.Fatalf("next_due через %v, ждали 24ч", d)
	}
	evs := mistakeEvents(e, "mistake.opened")
	if len(evs) != 1 {
		t.Fatalf("событий opened %d", len(evs))
	}
	var p struct {
		MistakeID, ExerciseID int64
		Kind                  string
		Topics                []string
	}
	if err := json.Unmarshal(evs[0].Payload, &p); err != nil || p.MistakeID != m.ID || p.ExerciseID != id || p.Kind != "gap" || len(p.Topics) != 2 {
		t.Fatalf("payload %s (%v)", evs[0].Payload, err)
	}
	res := decodeResult(t, rec)
	if res.Mistake == nil || res.Mistake.Status != api.Open || res.Mistake.Streak != 0 || !res.Mistake.NextDue.Equal(m.NextDue.Time) {
		t.Fatalf("mistake в результате: %+v", res.Mistake)
	}
}

func TestAnswerCorrectOpensNothing(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "a", "k1"), withCookie(token))
	if len(e.store.mistakes) != 0 || decodeResult(t, rec).Mistake != nil {
		t.Fatalf("верный ответ открыл ошибку: %+v", e.store.mistakes)
	}
}

func TestAnswerWrongSameSentenceResets(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	first := seedExercise(t, e, 1, "A1")
	e.do(http.MethodPost, answerPath, answerBody(first, "the", "k1"), withCookie(token))
	m := onlyMistake(t, e)
	m.Streak = 1
	e.store.mistakes[m.ID] = m

	second := seedExercise(t, e, 1, "A1")
	rec := e.do(http.MethodPost, answerPath, answerBody(second, "an", "k2"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	got := onlyMistake(t, e)
	if got.ID != m.ID || got.Streak != 0 || got.Given != "an" || got.Status != "open" {
		t.Fatalf("после сброса: %+v", got)
	}
	if n := len(mistakeEvents(e, "mistake.opened")); n != 1 {
		t.Fatalf("событий opened %d, ждали 1", n)
	}
	if res := decodeResult(t, rec); res.Mistake == nil || res.Mistake.Status != api.Open || res.Mistake.Streak != 0 {
		t.Fatalf("mistake в результате: %+v", res.Mistake)
	}
}

func TestAnswerReplayByKeyNoSecondMistake(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "A1")
	first := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k1"), withCookie(token))
	m := onlyMistake(t, e)
	second := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k1"), withCookie(token))
	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("повтор: статус %d\n%s\n%s", second.Code, first.Body, second.Body)
	}
	if got := onlyMistake(t, e); !mistakesEqual(got, m) {
		t.Fatalf("повтор изменил ошибку: %+v -> %+v", m, got)
	}
	if n := len(mistakeEvents(e, "mistake.opened")); n != 1 {
		t.Fatalf("событий opened %d", n)
	}
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k2"), withCookie(token)); rec.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d", rec.Code)
	}
}

func mistakesEqual(a, b db.Mistake) bool {
	return a.ID == b.ID && a.Streak == b.Streak && a.Status == b.Status && a.Given == b.Given && a.NextDue.Time.Equal(b.NextDue.Time)
}

// Повтор запроса по ключу на ответ на повтор не двигает серию и отдаёт тот же mistake.
func TestRepeatReplayDoesNotAdvance(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 1, time.Now().Add(-time.Hour), 0)
	ex := decodeExercise(t, getNext(t, e, token))
	first := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "a", "k1"), withCookie(token))
	if m := e.store.mistakes[mid]; m.Streak != 1 {
		t.Fatalf("серия %d, ждали 1", m.Streak)
	}
	second := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "a", "k1"), withCookie(token))
	if second.Body.String() != first.Body.String() || e.store.mistakes[mid].Streak != 1 {
		t.Fatalf("повтор изменил серию или ответ:\n%s\n%s", first.Body, second.Body)
	}
	if decodeResult(t, second).Mistake == nil {
		t.Fatal("в повторном ответе нет mistake")
	}
}

func TestNextServesDueRepeat(t *testing.T) {
	for _, kind := range []string{"gap", "order"} {
		t.Run(kind, func(t *testing.T) {
			e := newExerciseEnv(t)
			token := login(t, e)
			mid := seedMistake(e, 7, time.Now().Add(-time.Minute), 0)
			if kind == "order" {
				m := e.store.mistakes[mid]
				m.Kind = "order"
				m.Payload, _ = json.Marshal(exercisePayload{Prompt: "У меня есть собака.", Translation: "У меня есть собака.", Choices: []string{}, Tiles: []string{"i", "have", "a", "big", "dog"}, Topic: "skill:order"})
				m.Answer, _ = json.Marshal(exerciseAnswer{Answers: []string{"I have a big dog."}})
				e.store.mistakes[mid] = m
			}
			rec := getNext(t, e, token)
			if rec.Code != http.StatusOK {
				t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
			}
			ex := decodeExercise(t, rec)
			if ex.Repeat == nil || !*ex.Repeat || string(ex.Kind) != kind {
				t.Fatalf("упражнение: %+v", ex)
			}
			stored := e.store.exercises[ex.Id]
			if stored.MistakeID == nil || *stored.MistakeID != mid || stored.SentenceID != 7 || stored.Kind != kind {
				t.Fatalf("строка exercises: %+v", stored)
			}
			var raw map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &raw)
			for _, k := range []string{"answer", "answers", "correct"} {
				if _, ok := raw[k]; ok {
					t.Fatalf("в ответе есть %q: %s", k, rec.Body)
				}
			}
			if kind == "order" {
				if ex.Tiles == nil || len(*ex.Tiles) != 5 || exercises.CheckOrder([]string{"I have a big dog."}, strings.Join(*ex.Tiles, " ")) {
					t.Fatalf("плитки: %v", ex.Tiles)
				}
			} else if !slices.Contains(ex.Choices, "a") || len(ex.Choices) != 4 {
				t.Fatalf("варианты: %v", ex.Choices)
			}
		})
	}
}

func TestNextNoRepeatBeforeDue(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	seedMistake(e, 7, time.Now().Add(time.Hour), 0)
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ждали 404 (повтор рано): %s", rec.Code, rec.Body)
	}
}

func answeredExercise(e env, mistakeID *int64, at time.Time) {
	if e.store.exercises == nil {
		e.store.exercises = map[int64]db.Exercise{}
	}
	e.store.nextExerciseID++
	e.store.exercises[e.store.nextExerciseID] = db.Exercise{
		ID: e.store.nextExerciseID, UserID: 1, Kind: "gap", SentenceID: 1, MistakeID: mistakeID, AnsweredAt: ts(at),
		Topics: []string{"grammar:articles"},
	}
}

// Повтор не выдаётся, если ЛЮБОЕ из двух последних отвеченных упражнений было повтором.
func TestNextRepeatNotTwiceInRow(t *testing.T) {
	cases := []struct {
		name       string
		flags      []bool // от старого к новому: true — повтор
		wantRepeat bool
	}{
		{"повтор, потом обычное", []bool{true, false}, false},
		{"обычное, потом повтор", []bool{false, true}, false},
		{"два повтора", []bool{true, true}, false},
		{"повтор, два обычных", []bool{true, false, false}, true},
		{"два обычных", []bool{false, false}, true},
		{"одно обычное", []bool{false}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newExerciseEnv(t)
			token := login(t, e)
			e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
			knownWords(e, "i", "have", "a", "dog")
			mid := seedMistake(e, 7, time.Now().Add(-time.Hour), 0)
			for i, isRepeat := range tc.flags {
				var m *int64
				if isRepeat {
					m = &mid
				}
				answeredExercise(e, m, time.Now().Add(time.Duration(i-10)*time.Minute))
			}
			ex := decodeExercise(t, getNext(t, e, token))
			if got := ex.Repeat != nil && *ex.Repeat; got != tc.wantRepeat {
				t.Fatalf("повтор выдан: %v, ждали %v (%+v)", got, tc.wantRepeat, ex)
			}
		})
	}
}

func TestNextRepeatDailyLimit(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a", "dog")
	mid := seedMistake(e, 7, time.Now().Add(-time.Hour), 0)
	start := mistakes.DayStart(time.Now(), "")
	for i := range mistakes.MaxRepeatsPerDay {
		answeredExercise(e, &mid, start.Add(time.Duration(i+1)*time.Millisecond))
	}
	// Два последних — обычные, чтобы лимит сработал сам, а не правило «не подряд».
	answeredExercise(e, nil, start.Add(100*time.Millisecond))
	answeredExercise(e, nil, start.Add(101*time.Millisecond))
	if ex := decodeExercise(t, getNext(t, e, token)); ex.Repeat != nil && *ex.Repeat {
		t.Fatalf("повтор выдан сверх лимита: %+v", ex)
	}
	// Четыре за сегодня — ещё можно.
	delete(e.store.exercises, 1)
	if ex := decodeExercise(t, getNext(t, e, token)); ex.Repeat == nil || !*ex.Repeat {
		t.Fatalf("повтор не выдан при 4 за сегодня: %+v", ex)
	}
}

func TestDayStartUsesSettingsTimezone(t *testing.T) {
	e := newExerciseEnv(t)
	srv := newAuthServer(e.store, nil)
	now := time.Date(2026, 10, 24, 23, 30, 0, 0, time.UTC)
	got, err := srv.dayStart(t.Context(), 1, now)
	if err != nil || !got.Equal(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("без настроек: %v (%v)", got, err)
	}
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B1", Timezone: "Europe/Moscow"}
	got, err = srv.dayStart(t.Context(), 1, now)
	if err != nil || !got.Equal(time.Date(2026, 10, 24, 21, 0, 0, 0, time.UTC)) {
		t.Fatalf("Москва: %v (%v)", got, err)
	}
}

func TestNextOldestDueFirst(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	seedMistake(e, 1, time.Now().Add(-1*time.Hour), 0)
	oldest := seedMistake(e, 2, time.Now().Add(-5*time.Hour), 0)
	seedMistake(e, 3, time.Now().Add(-3*time.Hour), 0)
	ex := decodeExercise(t, getNext(t, e, token))
	if got := e.store.exercises[ex.Id].MistakeID; got == nil || *got != oldest {
		t.Fatalf("выдана ошибка %v, ждали %d", got, oldest)
	}
}

func TestNextSameRepeatOnReload(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	seedMistake(e, 1, time.Now().Add(-time.Hour), 0)
	a := decodeExercise(t, getNext(t, e, token))
	b := decodeExercise(t, getNext(t, e, token))
	if a.Id != b.Id || len(e.store.exercises) != 1 || b.Repeat == nil || !*b.Repeat || !slices.Equal(a.Choices, b.Choices) {
		t.Fatalf("перезагрузка: %+v и %+v, строк %d", a, b, len(e.store.exercises))
	}
}

func TestRepeatCorrectAdvancesAndCloses(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 1, time.Now().Add(-time.Hour), 0)
	ex := decodeExercise(t, getNext(t, e, token))
	before := time.Now()
	res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(ex.Id, "a", "k1"), withCookie(token)))
	m := e.store.mistakes[mid]
	if m.Streak != 1 || m.Status != "open" {
		t.Fatalf("после первого верного: %+v", m)
	}
	if d := m.NextDue.Time.Sub(before); d < mistakes.SecondDelay || d > mistakes.SecondDelay+time.Minute {
		t.Fatalf("next_due через %v, ждали 72ч", d)
	}
	if res.Mistake == nil || res.Mistake.Status != api.Open || res.Mistake.Streak != 1 || !res.Mistake.NextDue.Equal(m.NextDue.Time) {
		t.Fatalf("mistake: %+v", res.Mistake)
	}
	// Срок наступил (сдвигаем руками) и ещё один верный.
	m.NextDue = ts(time.Now().Add(-time.Minute))
	e.store.mistakes[mid] = m
	// Два обычных упражнения после повтора, иначе второй повтор не выдаётся.
	answeredExercise(e, nil, time.Now().Add(time.Second))
	answeredExercise(e, nil, time.Now().Add(2*time.Second))
	ex2 := decodeExercise(t, getNext(t, e, token))
	if ex2.Id == ex.Id || ex2.Repeat == nil || !*ex2.Repeat {
		t.Fatalf("второй повтор: %+v", ex2)
	}
	res = decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(ex2.Id, "a", "k2"), withCookie(token)))
	m = e.store.mistakes[mid]
	if m.Status != "closed" || m.Streak != 2 || !m.ClosedAt.Valid {
		t.Fatalf("после второго верного: %+v", m)
	}
	if res.Mistake == nil || res.Mistake.Status != api.Closed || res.Mistake.Streak != 2 {
		t.Fatalf("mistake: %+v", res.Mistake)
	}
	evs := mistakeEvents(e, "mistake.closed")
	var p struct{ MistakeID int64 }
	if len(evs) != 1 || json.Unmarshal(evs[0].Payload, &p) != nil || p.MistakeID != mid {
		t.Fatalf("события closed: %+v", evs)
	}
	// Закрытая больше не выдаётся.
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("после закрытия next: %d %s", rec.Code, rec.Body)
	}
}

func TestRepeatWrongResets(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 1, time.Now().Add(-time.Hour), 1)
	ex := decodeExercise(t, getNext(t, e, token))
	before := time.Now()
	res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(ex.Id, "an", "k1"), withCookie(token)))
	m := e.store.mistakes[mid]
	if len(e.store.mistakes) != 1 || m.Streak != 0 || m.Status != "open" || m.Given != "an" {
		t.Fatalf("после неверного: %+v", e.store.mistakes)
	}
	if d := m.NextDue.Time.Sub(before); d < mistakes.FirstDelay || d > mistakes.FirstDelay+time.Minute {
		t.Fatalf("next_due через %v", d)
	}
	if res.Mistake == nil || res.Mistake.Status != api.Open || res.Mistake.Streak != 0 {
		t.Fatalf("mistake: %+v", res.Mistake)
	}
	if n := len(mistakeEvents(e, "mistake.opened")); n != 0 {
		t.Fatalf("повтор открыл новую ошибку: %d", n)
	}
}

// Ответ на копию повтора, пока ошибка уже закрыта, ничего не открывает заново.
func TestRepeatAnswerOnClosedMistakeDoesNotReopen(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	mid := seedMistake(e, 1, time.Now().Add(-time.Hour), 0)
	ex := decodeExercise(t, getNext(t, e, token))
	m := e.store.mistakes[mid]
	m.Status, m.Streak, m.ClosedAt = "closed", 2, ts(time.Now())
	e.store.mistakes[mid] = m
	res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(ex.Id, "the", "k1"), withCookie(token)))
	if got := e.store.mistakes[mid]; got.Status != "closed" || got.Streak != 2 {
		t.Fatalf("закрытая тронута: %+v", got)
	}
	if res.Mistake == nil || res.Mistake.Status != api.Closed {
		t.Fatalf("mistake: %+v", res.Mistake)
	}
}

func TestRepeatSentenceMissingSkipped(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.sentencePool = []db.SampleSentencesRow{poolSentence(10, "I have a dog.")}
	knownWords(e, "i", "have", "a", "dog")
	// Самая старая ошибка с испорченными данными; предложения нет.
	bad := putMistake(e, db.Mistake{UserID: 1, Kind: "gap", SentenceID: 99, Topics: []string{"grammar:articles"},
		Payload: []byte(`{"nope":`), Answer: []byte(`{}`), NextDue: ts(time.Now().Add(-9 * time.Hour))})
	good := seedMistake(e, 1, time.Now().Add(-time.Hour), 0)
	ex := decodeExercise(t, getNext(t, e, token))
	if got := e.store.exercises[ex.Id].MistakeID; got == nil || *got != good {
		t.Fatalf("выдана ошибка %v, ждали следующую %d", got, good)
	}
	// Только испорченная: обычный выбор, не 500.
	delete(e.store.mistakes, good)
	e.store.exercises = nil
	rec := getNext(t, e, token)
	ex = decodeExercise(t, rec)
	if rec.Code != http.StatusOK || (ex.Repeat != nil && *ex.Repeat) || e.store.exercises[ex.Id].MistakeID != nil {
		t.Fatalf("ждали обычное упражнение: %d %s", rec.Code, rec.Body)
	}
	if m := e.store.mistakes[bad]; m.Status != "open" {
		t.Fatalf("испорченная ошибка изменена: %+v", m)
	}
}

func TestListMistakesRequiresLogin(t *testing.T) {
	e := newExerciseEnv(t)
	if rec := e.do(http.MethodGet, mistakesPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d", rec.Code)
	}
}

func TestListMistakesEmpty(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, mistakesPath, "", withCookie(token))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"open":[]`) {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.MistakeList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Counts != (api.MistakeCounts{}) {
		t.Fatalf("тело %s (%v)", rec.Body, err)
	}
}

func TestListMistakes(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	base := time.Now().Add(-500 * time.Hour)
	for i := range 201 {
		seedMistake(e, int64(i+1), base.Add(time.Duration(i)*time.Hour), 0)
	}
	// Срок последней — в будущем; ждут повтора первые 201 - (будущие).
	last := e.store.mistakes[e.store.nextMistakeID]
	last.NextDue = ts(time.Now().Add(48 * time.Hour))
	e.store.mistakes[last.ID] = last
	// Чужая и закрытые.
	other := seedMistake(e, 1000, base, 0)
	o := e.store.mistakes[other]
	o.UserID = 2
	e.store.mistakes[other] = o
	closedNow := seedMistake(e, 1001, base, 2)
	c := e.store.mistakes[closedNow]
	c.Status, c.ClosedAt = "closed", ts(time.Now().Add(-24*time.Hour))
	e.store.mistakes[closedNow] = c
	closedOld := seedMistake(e, 1002, base, 2)
	c = e.store.mistakes[closedOld]
	c.Status, c.ClosedAt = "closed", ts(time.Now().Add(-10*24*time.Hour))
	e.store.mistakes[closedOld] = c

	rec := e.do(http.MethodGet, mistakesPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.MistakeList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Open) != 200 || out.Counts.Open != 201 || out.Counts.DueNow != 200 || out.Counts.ClosedThisWeek != 1 {
		t.Fatalf("список %d, счётчики %+v", len(out.Open), out.Counts)
	}
	if !slices.IsSortedFunc(out.Open, func(a, b api.Mistake) int { return a.NextDue.Compare(b.NextDue) }) {
		t.Fatal("не отсортировано по сроку")
	}
	first := out.Open[0]
	if first.Prompt != "I have _____ dog." || first.Answer != "a" || first.Given != "the" || first.Topic != "grammar:articles" ||
		first.Kind != "gap" || first.Id != 1 {
		t.Fatalf("первая: %+v", first)
	}
	for _, m := range out.Open {
		if m.Id == other {
			t.Fatal("чужая ошибка в списке")
		}
	}
}

// Новая неверная ошибка с другим пропуском в том же предложении сбрасывает открытую ошибку и заменяет её данные.
func TestAnswerWrongResetReplacesBlank(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	first := seedExercise(t, e, 1, "A1")
	e.do(http.MethodPost, answerPath, answerBody(first, "the", "k1"), withCookie(token))
	m := onlyMistake(t, e)
	m.Streak = 1
	e.store.mistakes[m.ID] = m

	payload, _ := json.Marshal(exercisePayload{Prompt: "I have a big _____.", Translation: "У меня большая собака.", Choices: []string{"dog", "cat", "bird"}, Topic: "level:A1"})
	answer, _ := json.Marshal(exerciseAnswer{Answers: []string{"dog"}})
	second, err := e.store.InsertExercise(t.Context(), db.InsertExerciseParams{
		UserID: 1, Kind: "gap", SentenceID: 1, Topics: []string{"level:A1", "pos:noun"}, Payload: payload, Answer: answer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := e.do(http.MethodPost, answerPath, answerBody(second, "cat", "k2"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	got := onlyMistake(t, e)
	if got.ID != m.ID || got.Streak != 0 || got.Given != "cat" || string(got.Payload) != string(payload) ||
		string(got.Answer) != string(answer) || !slices.Equal(got.Topics, []string{"level:A1", "pos:noun"}) {
		t.Fatalf("после сброса: %+v", got)
	}
	if n := len(mistakeEvents(e, "mistake.opened")); n != 1 {
		t.Fatalf("событий opened %d, ждали 1", n)
	}
	// Следующий повтор строится из новых данных.
	got.NextDue = ts(time.Now().Add(-time.Minute))
	e.store.mistakes[got.ID] = got
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Repeat == nil || ex.Prompt != "I have a big _____." || len(ex.Choices) != 3 || !slices.Contains(ex.Choices, "dog") {
		t.Fatalf("повтор из старых данных: %+v", ex)
	}
	if stored := e.store.exercises[ex.Id]; !slices.Equal(stored.Topics, []string{"level:A1", "pos:noun"}) || string(stored.Answer) != string(answer) {
		t.Fatalf("строка повтора: %+v", stored)
	}
}

// Одновременное открытие: первый поиск не видит открытую ошибку, вставка упирается в уникальный индекс (23505),
// транзакция откатывается и проходит со второго раза: одна строка, рейтинг один раз, одно событие ответа.
func TestAnswerRetriesOnUniqueViolation(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	seedMistake(e, 1, time.Now().Add(time.Hour), 1)
	e.store.missOpenOnce = true
	id := seedExercise(t, e, 1, "A1")
	rec := e.do(http.MethodPost, answerPath, answerBody(id, "the", "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	m := onlyMistake(t, e)
	if m.Streak != 0 || m.Given != "the" {
		t.Fatalf("ошибка не сброшена: %+v", m)
	}
	if got := e.store.topicRatings[topicKey{1, "grammar:articles"}].Attempts; got != 1 {
		t.Fatalf("попыток по теме %d, ждали 1", got)
	}
	if n := len(answerEvents(e)); n != 1 {
		t.Fatalf("событий ответа %d, ждали 1", n)
	}
	if n := len(mistakeEvents(e, "mistake.opened")); n != 0 {
		t.Fatalf("событий opened %d, ждали 0 (ошибка уже была)", n)
	}
}
