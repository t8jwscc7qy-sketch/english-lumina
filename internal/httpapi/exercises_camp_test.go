package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const nextCampPath = "/api/v1/exercises/next?mode=camp"

// campEnv — сервер с n известными «Привалу» словами (среди них dog) и одним предложением в окне.
func campEnv(t *testing.T, n int, sentence string, known ...string) (env, string) {
	t.Helper()
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B1", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.wl = map[int64]fakeWord{}
	for i := 0; i < n; i++ {
		id := int64(100 + i)
		lemma := fmt.Sprintf("filler%c%c", 'a'+rune(i/26), 'a'+rune(i%26))
		if i < len(known) {
			lemma = known[i]
		}
		e.store.addLexeme(id, lemma, "noun", "A1")
		status := "learned"
		if i%2 == 1 {
			status = "known_at_start"
		}
		e.store.wl[id] = fakeWord{status: status, source: "lesson"}
	}
	e.store.sentencePool = poolFor(sentence)
	knownWords(e, strings.Fields(strings.ToLower(strings.Trim(sentence, ".")))...)
	return e, token
}

func poolFor(text string) []db.SampleSentencesRow {
	return []db.SampleSentencesRow{poolSentence(10, text)}
}

func getCamp(t *testing.T, e env, token string) int {
	t.Helper()
	return e.do(http.MethodGet, nextCampPath, "", withCookie(token)).Code
}

func TestCampClosedBelowMinimum(t *testing.T) {
	e, token := campEnv(t, 49, "I have a dog.", "dog")
	rec := e.do(http.MethodGet, nextCampPath, "", withCookie(token))
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeCampClosed {
		t.Fatalf("49 слов: статус %d, тело %s", rec.Code, rec.Body)
	}
	if getNext(t, e, token).Code != http.StatusOK {
		t.Fatal("обычный режим работает и при закрытом «Привале»")
	}
}

func TestCampServesSentenceMadeOfKnownWords(t *testing.T) {
	e, token := campEnv(t, 50, "I have a dog.", "dog")
	rec := e.do(http.MethodGet, nextCampPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestCampSkipsSentencesWithUnknownWords(t *testing.T) {
	e, token := campEnv(t, 50, "I have a cat.", "dog") // cat не изучено: покрытие 3/4
	rec := e.do(http.MethodGet, nextCampPath, "", withCookie(token))
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExercise || !strings.Contains(rec.Body.String(), "добавь слов") {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if getNext(t, e, token).Code != http.StatusOK {
		t.Fatal("в обычном режиме это предложение годится")
	}
}

func TestCampThresholdComesFromPreferences(t *testing.T) {
	e, token := campEnv(t, 50, "I have a cat.", "dog")
	e.store.learnPrefs = &db.LearnPreference{UserID: 1, NewWordsMode: "auto", NewWordsPerDay: 12, CampCoverage: 0.7}
	if getCamp(t, e, token) != http.StatusOK {
		t.Fatal("покрытие 0,75 проходит порог 0,7")
	}
	e.store.learnPrefs.CampCoverage = 0.9
	if getCamp(t, e, token) != http.StatusNotFound {
		t.Fatal("покрытие 0,75 не проходит порог 0,9")
	}
}

func TestCampUnknownModeIsNormal(t *testing.T) {
	e, token := campEnv(t, 1, "I have a cat.", "dog")
	rec := e.do(http.MethodGet, "/api/v1/exercises/next?mode=bogus", "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d", rec.Code)
	}
}

func TestCampFormAndMatchUseOnlyKnownWords(t *testing.T) {
	e, _ := campEnv(t, 50, "I have a dog.", "dog")
	camp, err := (&authServer{store: e.store}).loadCamp(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	srv := &authServer{rnd: func(int64) int64 { return 0 }}
	ws := srv.campWordsFor(camp, func(pos string) bool { return pos == "noun" }, 10)
	if len(ws) != 10 {
		t.Fatalf("слов %d, ждали 10", len(ws))
	}
	if got := srv.campWordsFor(camp, func(pos string) bool { return pos == "verb" }, 10); len(got) != 0 {
		t.Fatalf("глаголов среди известных нет: %v", got)
	}
}

func TestLearnStatusReportsCamp(t *testing.T) {
	e, token := learnEnv(t)
	e.store.wl = map[int64]fakeWord{}
	for i := int64(0); i < 49; i++ {
		e.store.wl[i+1] = fakeWord{status: "learned", source: "card"}
	}
	s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
	if s.CampOpen || s.CampWords != 49 || s.CampMinWords != 50 {
		t.Fatalf("49 слов: %+v", s)
	}
	e.store.wl[50] = fakeWord{status: "known_at_start", source: "placement"}
	if s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String()); !s.CampOpen || s.CampWords != 50 {
		t.Fatalf("50 слов: %+v", s)
	}
}

func TestCampDoesNotRecordLevelTopic(t *testing.T) {
	e, token := campEnv(t, 50, "I have a dog.", "dog")
	rec := e.do(http.MethodGet, nextCampPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	for _, topic := range e.store.exercises[ex.Id].Topics {
		if strings.HasPrefix(topic, "level:") {
			t.Fatalf("в «Привале» тема %q не записывается: иначе ответы сдвигают предложение сменить уровень", topic)
		}
	}
	// В обычном режиме та же выдача записывает уровень.
	e2, token2 := campEnv(t, 50, "I have a dog.", "dog")
	ex2 := decodeExercise(t, getNext(t, e2, token2))
	if !slices.ContainsFunc(e2.store.exercises[ex2.Id].Topics, func(s string) bool { return strings.HasPrefix(s, "level:") }) {
		t.Fatal("обычный режим записывает level:*")
	}
}

func TestCampNeverOffersLevelTopics(t *testing.T) {
	e, token := campEnv(t, 50, "I have a dog.", "dog")
	// level:A1 — самая слабая тема: без исключения она выпала бы первой.
	e.store.topicRatings = map[topicKey]db.TopicRating{{userID: 1, topic: "level:A1"}: {UserID: 1, Topic: "level:A1", Rating: 1000, Deviation: 350}}
	rec := e.do(http.MethodGet, nextCampPath, "", withCookie(token))
	if rec.Code != http.StatusOK || decodeExercise(t, rec).Topic == "level:A1" {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}
