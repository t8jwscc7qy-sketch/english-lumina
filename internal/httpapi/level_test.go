package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	levelPath   = "/api/v1/level/suggestion"
	dismissPath = "/api/v1/level/suggestion/dismiss"
)

// levelEnv — сервер с пользователем 1 на уровне B1 (настройки есть).
func levelEnv(t *testing.T) env {
	t.Helper()
	e := progressEnv(t)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "B1", TargetLevel: "B2", DailyMinMinutes: 30, Timezone: "UTC"}
	return e
}

// addAnswers кладёт n ответов темы пользователя, из них right верных; самые свежие — в конце списка верных/неверных
// не различаются по времени: ответы идут по минуте, самый старый n минут назад от base.
func addAnswers(e env, user int64, topic string, n, right int, base time.Time) {
	for i := 0; i < n; i++ {
		e.store.levelAnswers = append(e.store.levelAnswers, fakeLevelAnswer{
			UserID: user, Topic: topic, Correct: i < right, At: base.Add(-time.Duration(n-i) * time.Minute),
		})
	}
}

func getLevel(t *testing.T, e env) (api.LevelSuggestionResponse, string) {
	t.Helper()
	token := login(t, e)
	rec := e.do(http.MethodGet, levelPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var out api.LevelSuggestionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out, rec.Body.String()
}

func TestLevelSuggestionUp(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 28, progressNow)
	out, _ := getLevel(t, e)
	if out.Suggestion == nil || out.Suggestion.Direction != api.Up || out.Suggestion.From != api.LevelB1 || out.Suggestion.To != api.LevelB2 ||
		out.Suggestion.Correct != 28 || out.Suggestion.Total != 30 {
		t.Fatalf("ждали повышение B1→B2, 28 из 30: %+v", out.Suggestion)
	}
}

func TestLevelSuggestionDown(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 10, progressNow)
	out, _ := getLevel(t, e)
	if out.Suggestion == nil || out.Suggestion.Direction != api.Down || out.Suggestion.To != api.LevelA2 {
		t.Fatalf("ждали понижение до A2: %+v", out.Suggestion)
	}
}

func TestLevelSuggestionEmptyObjectWhenNothing(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 20, 20, progressNow) // мало ответов
	out, body := getLevel(t, e)
	if out.Suggestion != nil || strings.Contains(body, "suggestion") {
		t.Fatalf("предложения быть не должно, тело %s", body)
	}
}

func TestLevelSuggestionNoSettings(t *testing.T) {
	e := progressEnv(t) // онбординг не пройден
	out, _ := getLevel(t, e)
	if out.Suggestion != nil {
		t.Fatalf("без настроек уровня нет: %+v", out.Suggestion)
	}
}

func TestLevelSuggestionIgnoresOtherLevelAndOtherUser(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:A2", 30, 30, progressNow) // другой уровень
	addAnswers(e, 2, "level:B1", 30, 30, progressNow) // чужое
	out, _ := getLevel(t, e)
	if out.Suggestion != nil {
		t.Fatalf("чужие и другого уровня ответы не считаются: %+v", out.Suggestion)
	}
}

func TestLevelSuggestionIgnoresAnswersOlderThanSixtyDays(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 30, progressNow.AddDate(0, 0, -61))
	out, _ := getLevel(t, e)
	if out.Suggestion != nil {
		t.Fatalf("ответы старше 60 суток не считаются: %+v", out.Suggestion)
	}
	e2 := levelEnv(t)
	addAnswers(e2, 1, "level:B1", 30, 30, progressNow.AddDate(0, 0, -59))
	if out, _ := getLevel(t, e2); out.Suggestion == nil {
		t.Fatal("ответы 59-суточной давности должны считаться")
	}
}

func TestLevelSuggestionUsesNewestThirty(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 20, 0, progressNow.AddDate(0, 0, -10)) // старые неверные
	addAnswers(e, 1, "level:B1", 30, 30, progressNow)                   // свежие верные
	out, _ := getLevel(t, e)
	if out.Suggestion == nil || out.Suggestion.Direction != api.Up || out.Suggestion.Correct != 30 {
		t.Fatalf("считать нужно свежие 30: %+v", out.Suggestion)
	}
}

func TestLevelSuggestionDismissHidesForFourteenDays(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 30, progressNow)
	token := login(t, e)
	rec := e.do(http.MethodPost, dismissPath, "", withCookie(token))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if out, _ := getLevel(t, e); out.Suggestion != nil {
		t.Fatalf("после отказа предложения нет: %+v", out.Suggestion)
	}
	if len(e.store.events) != 1 || e.store.events[0].Type != "level.suggestion.dismissed" || e.store.events[0].UserID != 1 {
		t.Fatalf("ждали одно событие отказа: %+v", e.store.events)
	}
}

func TestLevelSuggestionReturnsAfterFourteenDays(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 30, progressNow)
	e.store.events = append(e.store.events, db.Event{
		ID: 1, UserID: 1, Type: "level.suggestion.dismissed", OccurredAt: ts(progressNow.AddDate(0, 0, -15)),
	})
	if out, _ := getLevel(t, e); out.Suggestion == nil {
		t.Fatal("через 15 суток после отказа предложение возвращается")
	}
}

func TestLevelSuggestionDismissOfOtherUserDoesNotHide(t *testing.T) {
	e := levelEnv(t)
	addAnswers(e, 1, "level:B1", 30, 30, progressNow)
	e.store.events = append(e.store.events, db.Event{ID: 1, UserID: 2, Type: "level.suggestion.dismissed", OccurredAt: ts(progressNow)})
	if out, _ := getLevel(t, e); out.Suggestion == nil {
		t.Fatal("отказ другого пользователя не должен прятать предложение")
	}
}

func TestLevelSuggestionUnauthorized(t *testing.T) {
	e := levelEnv(t)
	if rec := e.do(http.MethodGet, levelPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET статус %d", rec.Code)
	}
	if rec := e.do(http.MethodPost, dismissPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST статус %d", rec.Code)
	}
}

func TestLevelSuggestionDismissJournalFailureIs500(t *testing.T) {
	e := levelEnv(t)
	e.store.failEvents = true
	token := login(t, e)
	if rec := e.do(http.MethodPost, dismissPath, "", withCookie(token)); rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d, ждали 500", rec.Code)
	}
}
