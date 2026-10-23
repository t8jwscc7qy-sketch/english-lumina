package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
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
