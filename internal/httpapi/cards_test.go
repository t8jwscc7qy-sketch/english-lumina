package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
)

const cardsPath = "/api/v1/cards"

func runExamples() []db.ListExamplesRow {
	return []db.ListExamplesRow{
		{EnTatoebaID: 11, EnText: "I like to run every morning.", EnAuthor: "CK", RuTatoebaID: 22, RuText: "Я люблю бегать по утрам.", RuAuthor: "marafon"},
		{EnTatoebaID: 12, EnText: "He was running along the river.", EnAuthor: "CM", RuTatoebaID: 23, RuText: "Он бежал вдоль реки.", RuAuthor: "ksenija"},
	}
}

// cardsEnv: слово run/verb (id 1, уровень B2) с подходящими примерами и вошедший пользователь.
func cardsEnv(t *testing.T) (env, string) {
	t.Helper()
	e := newEnv(t)
	e.store.addLexeme(1, "run", "verb", "B2")
	e.store.examples = runExamples()
	return e, login(t, e)
}

func addBody(lemma, pos string) string {
	b, _ := json.Marshal(map[string]string{"lemma": lemma, "pos": pos})
	return string(b)
}

func decodeAdd(t *testing.T, body string) api.AddCardResponse {
	t.Helper()
	var r api.AddCardResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("тело %q: %v", body, err)
	}
	return r
}

func TestAddCardNeedsSessionAndOrigin(t *testing.T) {
	e, token := cardsEnv(t)
	if rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("без cookie: статус %d, ожидали 401", rec.Code)
	}
	rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token), withHeader("Origin", "https://evil.example"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("чужой Origin: статус %d, ожидали 403", rec.Code)
	}
	if len(e.store.cards) != 0 || len(e.store.events) != 0 {
		t.Fatal("запрос без прав что-то записал")
	}
}

func TestAddCardRejectsBadInput(t *testing.T) {
	tests := map[string]string{
		"пустая лемма":         addBody("", "verb"),
		"пробелы вместо леммы": addBody("   ", "verb"),
		"лемма из 65 символов": addBody(strings.Repeat("x", 65), "verb"),
		"нулевой байт в лемме": addBody("ru\x00n", "verb"),
		"часть речи из 33":     addBody("run", strings.Repeat("x", 33)),
		"битый JSON":           `{"lemma":`,
		"лишнее поле":          `{"lemma":"run","pos":"verb","admin":true}`,
		"нет тела":             ``,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			e, token := cardsEnv(t)
			rec := e.do(http.MethodPost, cardsPath, body, withCookie(token))
			if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeInvalidRequest {
				t.Fatalf("статус %d, тело %s; ожидали 400 invalid_request", rec.Code, rec.Body)
			}
			if len(e.store.cards) != 0 || len(e.store.events) != 0 {
				t.Fatal("плохой запрос что-то записал")
			}
		})
	}
}

func TestAddCardUnknownWord(t *testing.T) {
	e, token := cardsEnv(t)
	rec := e.do(http.MethodPost, cardsPath, addBody("zebra", "noun"), withCookie(token))
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeWordNotFound {
		t.Fatalf("статус %d, тело %s; ожидали 404 word_not_found", rec.Code, rec.Body)
	}
	// Слово с другой частью речи — другое слово.
	if rec := e.do(http.MethodPost, cardsPath, addBody("run", "noun"), withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("run/noun: статус %d, ожидали 404", rec.Code)
	}
	if len(e.store.cards) != 0 || len(e.store.events) != 0 {
		t.Fatal("неизвестное слово что-то записало")
	}
}

func TestAddCardWithoutUsableExamples(t *testing.T) {
	tests := map[string][]db.ListExamplesRow{
		"примеров нет": nil,
		"форма слова в предложении не найдена": {
			{EnTatoebaID: 1, EnText: "Nothing matches here at all.", EnAuthor: "CK", RuTatoebaID: 2, RuText: "Ничего.", RuAuthor: "a"},
		},
	}
	for name, rows := range tests {
		t.Run(name, func(t *testing.T) {
			e, token := cardsEnv(t)
			e.store.examples = rows
			rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token))
			if rec.Code != http.StatusConflict || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExamples {
				t.Fatalf("статус %d, тело %s; ожидали 409 no_examples", rec.Code, rec.Body)
			}
			if len(e.store.cards) != 0 || len(e.store.events) != 0 {
				t.Fatal("слово без примеров создало карточку")
			}
		})
	}
}

func TestAddCardCreatesCardAndEvent(t *testing.T) {
	e, token := cardsEnv(t)
	rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token))
	if rec.Code != http.StatusCreated {
		t.Fatalf("статус %d, тело %s; ожидали 201", rec.Code, rec.Body)
	}
	got := decodeAdd(t, rec.Body.String())
	if !got.Created || got.CardId == 0 {
		t.Fatalf("ответ %+v", got)
	}
	card, ok := e.store.cards[got.CardId]
	if !ok || card.UserID != 1 || card.LexemeID != 1 || card.State != 0 || card.Reps != 0 || card.LastReview.Valid || !card.Due.Valid {
		t.Fatalf("карточка %+v", card)
	}
	if len(e.store.events) != 1 {
		t.Fatalf("событий %d, ожидали 1", len(e.store.events))
	}
	ev := e.store.events[0]
	var payload struct {
		CardID   int64  `json:"cardId"`
		LexemeID int64  `json:"lexemeId"`
		Lemma    string `json:"lemma"`
		Pos      string `json:"pos"`
	}
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "card.added" || ev.SchemaVersion != 1 || ev.UserID != 1 || !ev.OccurredAt.Valid ||
		payload.CardID != got.CardId || payload.LexemeID != 1 || payload.Lemma != "run" || payload.Pos != "verb" {
		t.Fatalf("событие %+v, нагрузка %+v", ev, payload)
	}
}

func TestAddCardTwiceDoesNotDuplicate(t *testing.T) {
	e, token := cardsEnv(t)
	first := decodeAdd(t, e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token)).Body.String())
	rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("повтор: статус %d, ожидали 200", rec.Code)
	}
	second := decodeAdd(t, rec.Body.String())
	if second.Created || second.CardId != first.CardId {
		t.Fatalf("повтор: %+v, первый %+v", second, first)
	}
	if len(e.store.cards) != 1 || len(e.store.events) != 1 {
		t.Fatalf("карточек %d, событий %d; ожидали по одному", len(e.store.cards), len(e.store.events))
	}
}

func TestAddCardRollsBackWhenEventFails(t *testing.T) {
	e, token := cardsEnv(t)
	e.store.failEvents = true
	rec := e.do(http.MethodPost, cardsPath, addBody("run", "verb"), withCookie(token))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d, ожидали 500", rec.Code)
	}
	if len(e.store.cards) != 0 {
		t.Fatalf("после ошибки журнала осталась карточка: %+v", e.store.cards)
	}
}

func TestAddCardUsesLemmaAsStoredNotAsTyped(t *testing.T) {
	e, token := cardsEnv(t)
	e.store.lexemesByID[2] = db.Lexeme{ID: 2, Lemma: "cafe/café", Pos: "noun"}
	e.store.cardLevels[2] = "A1"
	e.store.examples = []db.ListExamplesRow{{EnTatoebaID: 5, EnText: "We had lunch at a café today.", EnAuthor: "CK", RuTatoebaID: 6, RuText: "Мы пообедали в кафе.", RuAuthor: "a"}}
	rec := e.do(http.MethodPost, cardsPath, addBody("cafe/café", "noun"), withCookie(token))
	if rec.Code != http.StatusCreated {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var created db.Card
	for _, c := range e.store.cards {
		created = c
	}
	if created.LexemeID != 2 {
		t.Fatalf("карточка на слово %d, ожидали 2", created.LexemeID)
	}
}

// --- очередь и оценка ---

const queuePath = "/api/v1/cards/queue"

func decodeQueue(t *testing.T, body string) api.CardQueue {
	t.Helper()
	var q api.CardQueue
	if err := json.Unmarshal([]byte(body), &q); err != nil {
		t.Fatalf("тело %q: %v", body, err)
	}
	return q
}

// seed кладёт карточку напрямую в хранилище: новая, со сроком due.
func seed(t *testing.T, e env, lexemeID int64, due time.Time) int64 {
	t.Helper()
	id, err := e.store.InsertCard(context.Background(), insertParams(1, lexemeID, srs.New(due)))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func getQueue(t *testing.T, e env, token string) api.CardQueue {
	t.Helper()
	rec := e.do(http.MethodGet, queuePath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("очередь: статус %d, тело %s", rec.Code, rec.Body)
	}
	return decodeQueue(t, rec.Body.String())
}

func reviewBody(cardID int64, rating int, key string) string {
	b, _ := json.Marshal(map[string]any{"cardId": cardID, "rating": rating, "key": key})
	return string(b)
}

func doReview(e env, token string, cardID int64, rating int, key string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, "/api/v1/cards/review", reviewBody(cardID, rating, key), withCookie(token))
}

func TestQueueNeedsSession(t *testing.T) {
	e, _ := cardsEnv(t)
	if rec := e.do(http.MethodGet, queuePath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", rec.Code)
	}
}

func TestQueueEmptyAndFuture(t *testing.T) {
	e, token := cardsEnv(t)
	q := getQueue(t, e, token)
	if q.DueCount != 0 || q.TotalCount != 0 || q.Card != nil {
		t.Fatalf("пустая очередь: %+v", q)
	}
	seed(t, e, 1, time.Now().Add(time.Hour))
	q = getQueue(t, e, token)
	if q.DueCount != 0 || q.TotalCount != 1 || q.Card != nil {
		t.Fatalf("карточка в будущем не должна быть в очереди: %+v", q)
	}
}

func TestQueueReturnsNextCardWithSentenceAndIntervals(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	q := getQueue(t, e, token)
	if q.DueCount != 1 || q.TotalCount != 1 || q.Card == nil {
		t.Fatalf("очередь: %+v", q)
	}
	c := q.Card
	if c.CardId != id || c.Lemma != "run" || c.Pos != "verb" || c.Level != api.CefrLevelB2 || c.State != 0 {
		t.Fatalf("карточка: %+v", c)
	}
	in := c.Intervals
	if !(in.Again > 0 && in.Again < in.Hard && in.Hard <= in.Good && in.Good < in.Easy) {
		t.Fatalf("интервалы: %+v", in)
	}
	if c.Sentence == nil || c.Sentence.En.Text != "I like to run every morning." || c.Sentence.Ru.Text != "Я люблю бегать по утрам." ||
		c.Sentence.En.TatoebaId != 11 || c.Sentence.Ru.Author != "marafon" || c.Sentence.Gap.Start != 10 || c.Sentence.Gap.End != 13 {
		t.Fatalf("предложение: %+v", c.Sentence)
	}
}

func TestQueueSentenceRotatesWithReps(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	wantText := []string{"I like to run every morning.", "He was running along the river.", "I like to run every morning."}
	for reps, want := range wantText {
		card := e.store.cards[id]
		card.Reps = int32(reps)
		e.store.cards[id] = card
		got := getQueue(t, e, token).Card
		if got == nil || got.Sentence == nil || got.Sentence.En.Text != want {
			t.Fatalf("reps=%d: предложение %+v, ожидали %q", reps, got, want)
		}
	}
	// Для второго примера пропуск стоит на «running».
	card := e.store.cards[id]
	card.Reps = 1
	e.store.cards[id] = card
	if g := getQueue(t, e, token).Card.Sentence.Gap; g.Start != 7 || g.End != 14 {
		t.Fatalf("пропуск для running: %+v, ожидали 7–14", g)
	}
}

func TestQueueSkipsExamplesWithoutFoundFormAndHandlesNoExamples(t *testing.T) {
	e, token := cardsEnv(t)
	seed(t, e, 1, time.Now().Add(-time.Hour))
	e.store.examples = []db.ListExamplesRow{
		{EnTatoebaID: 1, EnText: "Nothing matches here at all.", EnAuthor: "a", RuTatoebaID: 2, RuText: "Ничего.", RuAuthor: "b"},
		runExamples()[0],
	}
	got := getQueue(t, e, token).Card
	if got == nil || got.Sentence == nil || got.Sentence.En.TatoebaId != 11 {
		t.Fatalf("нужен только пример с найденной формой: %+v", got)
	}

	e.store.examples = nil // примеры исчезли после добавления карточки
	got = getQueue(t, e, token).Card
	if got == nil || got.Sentence != nil || got.Lemma != "run" {
		t.Fatalf("без примеров карточка приходит без предложения: %+v", got)
	}
}

func TestQueueIsOrderedByDue(t *testing.T) {
	e, token := cardsEnv(t)
	e.store.addLexeme(2, "go", "verb", "A1")
	seed(t, e, 1, time.Now().Add(-time.Hour))
	older := seed(t, e, 2, time.Now().Add(-3*time.Hour))
	q := getQueue(t, e, token)
	if q.DueCount != 2 || q.Card == nil || q.Card.CardId != older || q.Card.Level != api.CefrLevelA1 {
		t.Fatalf("очередь: %+v", q)
	}
}

func TestReviewNeedsSessionAndOrigin(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	if rec := e.do(http.MethodPost, "/api/v1/cards/review", reviewBody(id, 3, "k")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("без cookie: %d", rec.Code)
	}
	rec := e.do(http.MethodPost, "/api/v1/cards/review", reviewBody(id, 3, "k"), withCookie(token), withHeader("Origin", "https://evil.example"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("чужой Origin: %d", rec.Code)
	}
	if e.store.cards[id].Reps != 0 || len(e.store.events) != 0 {
		t.Fatal("запрос без прав что-то изменил")
	}
}

func TestReviewRejectsBadInput(t *testing.T) {
	tests := map[string]string{
		"оценка 0":            reviewBody(1, 0, "k"),
		"оценка 5":            reviewBody(1, 5, "k"),
		"оценка -1":           reviewBody(1, -1, "k"),
		"пустой ключ":         reviewBody(1, 3, ""),
		"ключ из 65 символов": reviewBody(1, 3, strings.Repeat("k", 65)),
		"номер карточки 0":    reviewBody(0, 3, "k"),
		"битый JSON":          `{"cardId":`,
		"лишнее поле":         `{"cardId":1,"rating":3,"key":"k","x":1}`,
		"нет тела":            ``,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			e, token := cardsEnv(t)
			seed(t, e, 1, time.Now().Add(-time.Hour))
			rec := e.do(http.MethodPost, "/api/v1/cards/review", body, withCookie(token))
			if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeInvalidRequest {
				t.Fatalf("статус %d, тело %s; ожидали 400 invalid_request", rec.Code, rec.Body)
			}
			if e.store.cards[1].Reps != 0 || len(e.store.events) != 0 {
				t.Fatal("плохой запрос что-то изменил")
			}
		})
	}
}

func TestReviewUnknownOrForeignCard(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	foreign := e.store.cards[id]
	foreign.UserID = 2
	e.store.cards[id] = foreign
	snapshot := e.store.cards[id]

	for name, cardID := range map[string]int64{"чужая": id, "несуществующая": 999} {
		rec := doReview(e, token, cardID, 3, "k")
		if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNotFound {
			t.Fatalf("%s: статус %d, тело %s; ожидали 404 not_found", name, rec.Code, rec.Body)
		}
	}
	if e.store.cards[id] != snapshot || len(e.store.events) != 0 {
		t.Fatal("отказ что-то изменил")
	}
}

func TestReviewAppliesRatingAndWritesEvent(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	before := time.Now()
	rec := doReview(e, token, id, int(srs.Good), "key-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	card := e.store.cards[id]
	if card.Reps != 1 || card.State == 0 || !card.Due.Time.After(before) || !card.LastReview.Valid {
		t.Fatalf("карточка после «хорошо»: %+v", card)
	}
	if len(e.store.events) != 1 {
		t.Fatalf("событий %d, ожидали 1", len(e.store.events))
	}
	ev := e.store.events[0]
	var p struct {
		CardID, LexemeID, Rating, StateBefore, StateAfter, ScheduledDays int
		DueAfter                                                         string
		Stability, Difficulty                                            float64
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "card.reviewed" || ev.SchemaVersion != 1 || ev.IdempotencyKey == nil || *ev.IdempotencyKey != fmt.Sprintf("review:%d:key-1", id) ||
		p.CardID != int(id) || p.LexemeID != 1 || p.Rating != 3 || p.StateBefore != 0 || p.StateAfter != int(card.State) ||
		p.Stability != card.Stability || p.Difficulty != card.Difficulty {
		t.Fatalf("событие %+v, нагрузка %+v", ev, p)
	}
	if _, err := time.Parse(time.RFC3339Nano, p.DueAfter); err != nil {
		t.Fatalf("dueAfter %q: %v", p.DueAfter, err)
	}
	// После оценки новая карточка вернётся не сразу: очередь пуста, но карточка учтена.
	if q := decodeQueue(t, rec.Body.String()); q.DueCount != 0 || q.TotalCount != 1 || q.Card != nil {
		t.Fatalf("очередь после оценки: %+v", q)
	}
}

func TestReviewWithSameKeyIsAppliedOnce(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	if rec := doReview(e, token, id, 3, "same"); rec.Code != http.StatusOK {
		t.Fatalf("первая оценка: %d", rec.Code)
	}
	afterFirst := e.store.cards[id]
	rec := doReview(e, token, id, 3, "same")
	if rec.Code != http.StatusOK {
		t.Fatalf("повтор: статус %d, ожидали 200", rec.Code)
	}
	if e.store.cards[id] != afterFirst || len(e.store.events) != 1 {
		t.Fatalf("повтор изменил карточку или добавил событие: %+v, событий %d", e.store.cards[id], len(e.store.events))
	}
	// Даже с другой оценкой тот же ключ — повтор, а не новая оценка.
	if rec := doReview(e, token, id, 1, "same"); rec.Code != http.StatusOK || e.store.cards[id] != afterFirst {
		t.Fatalf("тот же ключ с другой оценкой должен быть проигнорирован: %d", rec.Code)
	}
	// Новый ключ — новая оценка.
	if rec := doReview(e, token, id, 3, "other"); rec.Code != http.StatusOK || e.store.cards[id].Reps != 2 || len(e.store.events) != 2 {
		t.Fatalf("новый ключ: reps %d, событий %d", e.store.cards[id].Reps, len(e.store.events))
	}
}

func TestReviewSameKeyOnDifferentCardsIsSeparate(t *testing.T) {
	e, token := cardsEnv(t)
	e.store.addLexeme(2, "go", "verb", "A1")
	a := seed(t, e, 1, time.Now().Add(-time.Hour))
	b := seed(t, e, 2, time.Now().Add(-time.Hour))
	doReview(e, token, a, 3, "k")
	doReview(e, token, b, 3, "k")
	if e.store.cards[a].Reps != 1 || e.store.cards[b].Reps != 1 || len(e.store.events) != 2 {
		t.Fatalf("ключ должен действовать в рамках карточки: %d/%d повторений, событий %d", e.store.cards[a].Reps, e.store.cards[b].Reps, len(e.store.events))
	}
}

func TestReviewAgainOnMatureCardCountsLapse(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	// Доводим карточку до зрелой настоящим алгоритмом.
	c, now := srs.New(time.Now().Add(-100*24*time.Hour)), time.Now().Add(-100*24*time.Hour)
	for i := 0; i < 10 && c.State != srs.StateReview; i++ {
		c, _ = srs.Review(c, srs.Good, now)
		now = c.Due
	}
	if c.State != srs.StateReview {
		t.Fatalf("карточка не стала зрелой: %+v", c)
	}
	c.Due = time.Now().Add(-time.Hour)
	p := insertParams(1, 1, c)
	e.store.cards[id] = db.Card{ID: id, UserID: 1, LexemeID: 1, Due: p.Due, Stability: p.Stability, Difficulty: p.Difficulty,
		ScheduledDays: p.ScheduledDays, Reps: p.Reps, Lapses: p.Lapses, State: p.State, LastReview: p.LastReview, RemainingSteps: p.RemainingSteps}

	if rec := doReview(e, token, id, int(srs.Again), "k"); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if got := e.store.cards[id]; got.Lapses != 1 || got.State != int16(srs.StateRelearning) {
		t.Fatalf("после «забыл»: %+v", got)
	}
}

func TestReviewReturnsNextCard(t *testing.T) {
	e, token := cardsEnv(t)
	e.store.addLexeme(2, "go", "verb", "A1")
	first := seed(t, e, 1, time.Now().Add(-3*time.Hour))
	second := seed(t, e, 2, time.Now().Add(-time.Hour))
	rec := doReview(e, token, first, 3, "k")
	q := decodeQueue(t, rec.Body.String())
	if q.DueCount != 1 || q.TotalCount != 2 || q.Card == nil || q.Card.CardId != second {
		t.Fatalf("после оценки первой должна прийти вторая: %+v", q)
	}
}

func TestReviewRollsBackWhenEventFails(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	snapshot := e.store.cards[id]
	e.store.failEvents = true
	if rec := doReview(e, token, id, 3, "k"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d, ожидали 500", rec.Code)
	}
	if e.store.cards[id] != snapshot {
		t.Fatalf("после ошибки журнала карточка изменилась: %+v", e.store.cards[id])
	}
}

func TestReviewWorksWhenExamplesVanished(t *testing.T) {
	e, token := cardsEnv(t)
	id := seed(t, e, 1, time.Now().Add(-time.Hour))
	e.store.examples = nil
	if rec := doReview(e, token, id, 3, "k"); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}
