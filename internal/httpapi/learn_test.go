package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
)

const (
	learnStatusPath      = "/api/v1/learn/status"
	learnBlockPath       = "/api/v1/learn/block"
	learnIntroducePath   = "/api/v1/learn/introduce"
	learnPlacementPath   = "/api/v1/learn/placement"
	learnPreferencesPath = "/api/v1/learn/preferences"
)

func learnEnv(t *testing.T) (env, string) {
	t.Helper()
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B1", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.kbBuilt = true
	return e, token
}

// addLearnWord кладёт слово в подборку знакомства, в карточки базы знаний и (если withExample) в примеры Tatoeba.
func addLearnWord(e env, id int64, lemma, pos, level string, rank int32, withExample bool, translations ...string) {
	payload, _ := json.Marshal(map[string]any{
		"lemma": lemma, "pos": pos, "level": level, "rank": rank, "translations": translations,
		"ipa": "ˈ" + lemma, "audio": "En-us-" + lemma + ".ogg", "forms": []string{lemma + "s"}, "synonyms": []string{"syn-" + lemma},
	})
	rk := rank
	e.store.learnCands = append(e.store.learnCands, db.LearnCandidatesRow{LexemeID: id, Lemma: lemma, Pos: pos, Level: level, FreqRank: &rk, Payload: payload})
	if e.store.kbCards == nil {
		e.store.kbCards = map[int64]json.RawMessage{}
	}
	e.store.kbCards[id] = payload
	e.store.addLexeme(id, lemma, pos, level)
	e.store.levels = append(e.store.levels, db.ListLexemeLevelsRow{LexemeID: id, Source: "cefrj", Level: level})
	if withExample {
		prev := e.store.examplesFor
		row := db.ListExamplesRow{EnID: id * 10, EnTatoebaID: id * 100, EnText: "I see a " + lemma + " today.", EnAuthor: "tom", RuTatoebaID: id * 101, RuText: "Я вижу " + lemma + " сегодня.", RuAuthor: "ivan"}
		e.store.examplesFor = func(q string) []db.ListExamplesRow {
			if strings.Contains(q, lemma) {
				return []db.ListExamplesRow{row}
			}
			if prev != nil {
				return prev(q)
			}
			return nil
		}
	}
}

func getLearn(t *testing.T, e env, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(http.MethodGet, path, "", withCookie(token))
}

func decodeStatus(t *testing.T, body string) api.LearnStatus {
	t.Helper()
	var s api.LearnStatus
	if err := json.Unmarshal([]byte(body), &s); err != nil {
		t.Fatalf("тело %q: %v", body, err)
	}
	return s
}

func decodeBlock(t *testing.T, body string) api.LearnBlock {
	t.Helper()
	var b api.LearnBlock
	if err := json.Unmarshal([]byte(body), &b); err != nil {
		t.Fatalf("тело %q: %v", body, err)
	}
	return b
}

func lemmasOf(b api.LearnBlock) []string {
	var out []string
	for _, w := range b.Words {
		out = append(out, w.Lemma)
	}
	return out
}

func TestLearnNeedsSessionAndSettings(t *testing.T) {
	e := newExerciseEnv(t)
	for _, path := range []string{learnStatusPath, learnBlockPath, learnPlacementPath, learnPreferencesPath} {
		if rec := e.do(http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s без сессии: статус %d", path, rec.Code)
		}
	}
	token := login(t, e)
	for _, path := range []string{learnStatusPath, learnBlockPath, learnPlacementPath} {
		rec := getLearn(t, e, token, path)
		if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeOnboardingRequired {
			t.Fatalf("%s без настроек: статус %d, тело %s", path, rec.Code, rec.Body)
		}
	}
}

func TestLearnStatus(t *testing.T) {
	e, token := learnEnv(t)
	now := time.Now()
	e.store.wl = map[int64]fakeWord{
		1: {status: "learned", source: "legacy_card"},
		2: {status: "learned", source: "card"},
		3: {status: "introduced", source: "lesson", introducedAt: now},
		4: {status: "known_at_start", source: "placement"},
		5: {status: "introduced", source: "lesson", introducedAt: now.Add(-48 * time.Hour)},
	}
	rec := getLearn(t, e, token, learnStatusPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	s := decodeStatus(t, rec.Body.String())
	if s.Learned != 2 || s.Introduced != 2 || s.KnownAtStart != 1 || s.NewToday != 1 || s.Limit != 5 || s.Remaining != 4 ||
		s.Reason.Code != api.LearnReasonCodeBase || s.Mode != api.LearnStatusModeAuto || s.PerDay != 12 || !s.KbReady {
		t.Fatalf("статус: %+v", s)
	}
	if s.Reason.Text == "" {
		t.Fatal("у причины нет текста")
	}
}

func TestLearnStatusReasons(t *testing.T) {
	t.Run("много просроченных карточек — лимит 0", func(t *testing.T) {
		e, token := learnEnv(t)
		for i := 0; i < 36; i++ {
			seed(t, e, int64(1000+i), time.Now().Add(-time.Hour))
		}
		s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
		if s.Limit != 0 || s.Remaining != 0 || s.Reason.Code != api.LearnReasonCodeBacklog {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("плохая точность — лимит меньше", func(t *testing.T) {
		e, token := learnEnv(t)
		e.store.reviewTotal, e.store.reviewGood = 10, 5
		s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
		if s.Limit != 3 || s.Reason.Code != api.LearnReasonCodeHard {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("мало повторений — точность не считается", func(t *testing.T) {
		e, token := learnEnv(t)
		e.store.reviewTotal, e.store.reviewGood = 4, 0
		s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
		if s.Limit != 5 || s.Reason.Code != api.LearnReasonCodeBase {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("ручной режим", func(t *testing.T) {
		e, token := learnEnv(t)
		e.store.learnPrefs = &db.LearnPreference{UserID: 1, NewWordsMode: "manual", NewWordsPerDay: 9, CampCoverage: 0.95}
		s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
		if s.Limit != 9 || s.Mode != api.LearnStatusModeManual || s.PerDay != 9 || s.Reason.Code != api.LearnReasonCodeManual {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("база знаний не собрана", func(t *testing.T) {
		e, token := learnEnv(t)
		e.store.kbBuilt = false
		if s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String()); s.KbReady {
			t.Fatalf("%+v", s)
		}
	})
}

func TestLearnBlockOrderAndDiversity(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "house", "noun", "A1", 200, false, "дом") // нет примера — пропускается
	addLearnWord(e, 3, "water", "noun", "A1", 300, true, "вода", "водичка")
	addLearnWord(e, 4, "table", "noun", "A1", 400, true, "стол")
	addLearnWord(e, 5, "run", "verb", "A1", 500, true, "бежать")
	addLearnWord(e, 6, "cat", "noun", "A1", 600, true, "кот")
	e.store.learnRelations = []db.LearnRelationsRow{{LexemeID: 4, OtherLexemeID: i64(3)}} // table связан с water
	rec := getLearn(t, e, token, learnBlockPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	b := decodeBlock(t, rec.Body.String())
	if got := lemmasOf(b); !slices.Equal(got, []string{"apple", "water", "run", "cat"}) {
		t.Fatalf("блок %v: без house (нет примера) и table (связан с water)", got)
	}
	apple := b.Words[0]
	if apple.LexemeId != 1 || apple.Level != api.CefrLevel("A1") || apple.Rank == nil || *apple.Rank != 100 || !slices.Equal(apple.Translations, []string{"яблоко"}) ||
		apple.Ipa == nil || *apple.Ipa != "ˈapple" || apple.AudioUrl == nil || *apple.AudioUrl != "https://commons.wikimedia.org/wiki/Special:FilePath/En-us-apple.ogg" ||
		!slices.Equal(apple.Forms, []string{"apples"}) || !slices.Equal(apple.Synonyms, []string{"syn-apple"}) {
		t.Fatalf("слово: %+v", apple)
	}
	if len(apple.Examples) != 1 || apple.Examples[0].En.Text != "I see a apple today." || apple.Examples[0].En.Author != "tom" || apple.Examples[0].Highlight == nil {
		t.Fatalf("примеры: %+v", apple.Examples)
	}
	if b.Limit != 5 || b.Remaining != 5 || b.Reason.Code != api.LearnReasonCodeBase {
		t.Fatalf("лимит: %+v", b)
	}
}

func TestLearnBlockSkipsWordsWithCardOrState(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "run", "verb", "A1", 200, true, "бежать")
	addLearnWord(e, 3, "big", "adjective", "A1", 300, true, "большой")
	seed(t, e, 1, time.Now().Add(time.Hour))
	e.store.wl = map[int64]fakeWord{2: {status: "learned", source: "card"}}
	if got := lemmasOf(decodeBlock(t, getLearn(t, e, token, learnBlockPath).Body.String())); !slices.Equal(got, []string{"big"}) {
		t.Fatalf("блок %v: слова с карточкой и с состоянием изучения не берутся", got)
	}
}

func TestLearnBlockLevelWindow(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "adjust", "verb", "A2", 50, true, "подстроить")
	addLearnWord(e, 3, "bargain", "noun", "B1", 10, true, "сделка")
	if got := lemmasOf(decodeBlock(t, getLearn(t, e, token, learnBlockPath).Body.String())); !slices.Equal(got, []string{"apple", "adjust"}) {
		t.Fatalf("уровень пользователя A1 и следующий A2, B1 не берётся: %v", got)
	}
}

func TestLearnBlockNoWordsIs404(t *testing.T) {
	e, token := learnEnv(t)
	rec := getLearn(t, e, token, learnBlockPath)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoWords {
		t.Fatalf("нет слов: статус %d, тело %s", rec.Code, rec.Body)
	}
	e.store.failLearn = true // например, миграция kb не применена: запрос падает, а ответ остаётся понятным
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	rec = getLearn(t, e, token, learnBlockPath)
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoWords {
		t.Fatalf("ошибка чтения kb: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestLearnBlockLimitAndBacklogReturnEmptyWithReason(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	now := time.Now()
	e.store.wl = map[int64]fakeWord{}
	for i := int64(10); i < 15; i++ {
		e.store.wl[i] = fakeWord{status: "introduced", source: "lesson", introducedAt: now}
	}
	b := decodeBlock(t, getLearn(t, e, token, learnBlockPath).Body.String())
	if len(b.Words) != 0 || b.Remaining != 0 || b.Reason.Code != api.LearnReasonCodeDone {
		t.Fatalf("лимит исчерпан: %+v", b)
	}

	e2, token2 := learnEnv(t)
	addLearnWord(e2, 1, "apple", "noun", "A1", 100, true, "яблоко")
	for i := 0; i < 36; i++ {
		seed(t, e2, int64(1000+i), time.Now().Add(-time.Hour))
	}
	b = decodeBlock(t, getLearn(t, e2, token2, learnBlockPath).Body.String())
	if len(b.Words) != 0 || b.Reason.Code != api.LearnReasonCodeBacklog {
		t.Fatalf("много просроченных: %+v", b)
	}
}

func introduceBody(ids []int64, key string) string {
	b, _ := json.Marshal(map[string]any{"lexemeIds": ids, "key": key})
	return string(b)
}

func introduce(e env, token string, ids []int64, key string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, learnIntroducePath, introduceBody(ids, key), withCookie(token))
}

func eventsOfType(e env, typ string) []db.Event {
	var out []db.Event
	for _, ev := range e.store.events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func TestLearnIntroduceCreatesCardsAndState(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "run", "verb", "A1", 200, true, "бежать")
	rec := introduce(e, token, []int64{1, 2}, "k1")
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var r api.IntroduceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Introduced != 2 || len(r.CardIds) != 2 {
		t.Fatalf("ответ %s (%v)", rec.Body, err)
	}
	if len(e.store.cards) != 2 || !e.store.hasCard(1, 1) || !e.store.hasCard(1, 2) {
		t.Fatalf("карточки: %+v", e.store.cards)
	}
	for _, id := range []int64{1, 2} {
		if w := e.store.wl[id]; w.status != "introduced" || w.source != "lesson" || w.introducedAt.IsZero() {
			t.Fatalf("состояние слова %d: %+v", id, w)
		}
	}
	if len(eventsOfType(e, "word.introduced")) != 1 {
		t.Fatalf("событий word.introduced %d, ждали 1", len(eventsOfType(e, "word.introduced")))
	}

	again := introduce(e, token, []int64{1, 2}, "k1")
	if again.Code != http.StatusOK || again.Body.String() != rec.Body.String() {
		t.Fatalf("повтор по ключу: %d %s", again.Code, again.Body)
	}
	if len(e.store.cards) != 2 || len(eventsOfType(e, "word.introduced")) != 1 {
		t.Fatalf("повтор по ключу ничего не добавляет: карточек %d, событий %d", len(e.store.cards), len(eventsOfType(e, "word.introduced")))
	}
}

func TestLearnIntroduceSkipsWordsAlreadyKnown(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "run", "verb", "A1", 200, true, "бежать")
	seed(t, e, 1, time.Now().Add(time.Hour)) // у apple уже есть карточка
	rec := introduce(e, token, []int64{1, 2}, "k1")
	var r api.IntroduceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != http.StatusOK || r.Introduced != 1 || len(r.CardIds) != 1 {
		t.Fatalf("ответ %d %s", rec.Code, rec.Body)
	}
	if _, ok := e.store.wl[1]; ok {
		t.Fatal("слово, у которого уже была карточка, не вводится заново")
	}
}

func TestLearnIntroduceLimit(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "run", "verb", "A1", 200, true, "бежать")
	e.store.wl = map[int64]fakeWord{}
	for i := int64(10); i < 14; i++ { // введено 4 из 5
		e.store.wl[i] = fakeWord{status: "introduced", source: "lesson", introducedAt: time.Now()}
	}
	rec := introduce(e, token, []int64{1, 2}, "k1")
	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeLimitReached {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if len(e.store.cards) != 0 {
		t.Fatalf("при отказе карточки не создаются: %+v", e.store.cards)
	}
}

func TestLearnIntroduceValidation(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	addLearnWord(e, 2, "run", "verb", "A1", 200, false, "бежать") // без примера
	for name, body := range map[string]string{
		"пустой список":      introduceBody([]int64{}, "k"),
		"восемь слов":        introduceBody([]int64{1, 2, 3, 4, 5, 6, 7, 8}, "k"),
		"повтор номера":      introduceBody([]int64{1, 1}, "k"),
		"нет ключа":          introduceBody([]int64{1}, ""),
		"длинный ключ":       introduceBody([]int64{1}, strings.Repeat("k", 65)),
		"слова нет в базе":   introduceBody([]int64{99}, "k"),
		"слово без примеров": introduceBody([]int64{2}, "k"),
		"лишнее поле":        `{"lexemeIds":[1],"key":"k","extra":1}`,
		"не JSON":            `не json`,
	} {
		rec := e.do(http.MethodPost, learnIntroducePath, body, withCookie(token))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: статус %d, тело %s", name, rec.Code, rec.Body)
		}
	}
	if len(e.store.cards) != 0 || len(e.store.wl) != 0 {
		t.Fatal("после отказов ничего не создано")
	}
	if rec := e.do(http.MethodPost, learnIntroducePath, introduceBody([]int64{1}, "k"), withHeader("Origin", "https://evil.example"), withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("чужой источник: %d", rec.Code)
	}
}

func TestLearnPlacementPages(t *testing.T) {
	e, token := learnEnv(t)
	for i := int64(1); i <= 60; i++ {
		e.store.learnPlacement = append(e.store.learnPlacement, db.LearnPlacementCandidatesRow{LexemeID: i, Lemma: fmt.Sprintf("w%d", i), Pos: "noun"})
	}
	var p0, p1 api.PlacementPage
	rec := getLearn(t, e, token, learnPlacementPath)
	if err := json.Unmarshal(rec.Body.Bytes(), &p0); err != nil || rec.Code != http.StatusOK || len(p0.Words) != 50 || !p0.HasMore || p0.Skip != 0 || p0.Words[0].Lemma != "w1" {
		t.Fatalf("страница 0: %d %s", rec.Code, rec.Body)
	}
	rec = getLearn(t, e, token, learnPlacementPath+"?skip=50")
	if err := json.Unmarshal(rec.Body.Bytes(), &p1); err != nil || len(p1.Words) != 10 || p1.HasMore || p1.Skip != 50 || p1.Words[0].Lemma != "w51" {
		t.Fatalf("страница 1: %d %s", rec.Code, rec.Body)
	}
	for _, bad := range []string{"?skip=x", "?skip=-1", "?skip=100000"} {
		if rec := getLearn(t, e, token, learnPlacementPath+bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: статус %d", bad, rec.Code)
		}
	}
}

func placementBody(known, shown []int64, key string) string {
	b, _ := json.Marshal(map[string]any{"known": known, "shown": shown, "key": key})
	return string(b)
}

func submitPlacement(e env, token string, known, shown []int64, key string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, learnPlacementPath, placementBody(known, shown, key), withCookie(token))
}

func TestLearnPlacementSubmit(t *testing.T) {
	e, token := learnEnv(t)
	shown := []int64{}
	for i := int64(1); i <= 10; i++ {
		addLearnWord(e, i, fmt.Sprintf("w%d", i), "noun", "A1", int32(i), true, "слово")
		shown = append(shown, i)
	}
	rec := submitPlacement(e, token, shown[:9], shown, "p1")
	var r api.PlacementResult
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != http.StatusOK || r.Marked != 9 || r.Next != api.More {
		t.Fatalf("90 %% знаю: %d %s", rec.Code, rec.Body)
	}
	if len(e.store.cards) != 0 {
		t.Fatal("«знаю» не создаёт карточек")
	}
	for _, id := range shown[:9] {
		if w := e.store.wl[id]; w.status != "known_at_start" || w.source != "placement" {
			t.Fatalf("слово %d: %+v", id, w)
		}
	}
	if _, ok := e.store.wl[10]; ok {
		t.Fatal("слово «не знаю» не записывается")
	}
	if len(eventsOfType(e, "word.known_at_start")) != 1 {
		t.Fatalf("событий %d", len(eventsOfType(e, "word.known_at_start")))
	}
	again := submitPlacement(e, token, shown[:9], shown, "p1")
	if again.Code != http.StatusOK || again.Body.String() != rec.Body.String() || len(eventsOfType(e, "word.known_at_start")) != 1 {
		t.Fatalf("повтор по ключу: %d %s", again.Code, again.Body)
	}
}

func TestLearnPlacementNext(t *testing.T) {
	for known, want := range map[int]api.PlacementResultNext{10: api.More, 8: api.More, 7: api.Learn, 5: api.Learn, 4: api.Stop, 0: api.Stop} {
		e, token := learnEnv(t)
		shown := []int64{}
		for i := int64(1); i <= 10; i++ {
			addLearnWord(e, i, fmt.Sprintf("w%d", i), "noun", "A1", int32(i), true, "слово")
			shown = append(shown, i)
		}
		rec := submitPlacement(e, token, shown[:known], shown, "p")
		var r api.PlacementResult
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Next != want || r.Marked != known {
			t.Errorf("знакомых %d из 10: %s, ждали %s", known, rec.Body, want)
		}
	}
}

func TestLearnPlacementValidation(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	for name, body := range map[string]string{
		"known вне shown":    placementBody([]int64{1}, []int64{}, "k"),
		"неизвестное слово":  placementBody([]int64{99}, []int64{99}, "k"),
		"нет ключа":          placementBody([]int64{1}, []int64{1}, ""),
		"повтор номера":      placementBody([]int64{1, 1}, []int64{1}, "k"),
		"слишком много слов": placementBody([]int64{}, make([]int64, 51), "k"),
		"лишнее поле":        `{"known":[],"shown":[],"key":"k","x":1}`,
	} {
		rec := e.do(http.MethodPost, learnPlacementPath, body, withCookie(token))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: статус %d, тело %s", name, rec.Code, rec.Body)
		}
	}
	if len(e.store.wl) != 0 {
		t.Fatal("после отказов ничего не записано")
	}
	rec := submitPlacement(e, token, []int64{}, []int64{}, "empty")
	var r api.PlacementResult
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != http.StatusOK || r.Next != api.Stop || r.Marked != 0 {
		t.Fatalf("пустая страница: %d %s", rec.Code, rec.Body)
	}
}

func TestLearnPreferences(t *testing.T) {
	e, token := learnEnv(t)
	rec := getLearn(t, e, token, learnPreferencesPath)
	var p api.LearnPreferences
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusOK || p.NewWordsMode != api.LearnPreferencesNewWordsModeAuto || p.NewWordsPerDay != 12 || p.CampCoverage < 0.949 || p.CampCoverage > 0.951 {
		t.Fatalf("по умолчанию: %d %s", rec.Code, rec.Body)
	}
	put := func(body string) *httptest.ResponseRecorder {
		return e.do(http.MethodPut, learnPreferencesPath, body, withCookie(token))
	}
	rec = put(`{"newWordsMode":"manual","newWordsPerDay":9,"campCoverage":0.97}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("сохранение: %d %s", rec.Code, rec.Body)
	}
	rec = getLearn(t, e, token, learnPreferencesPath)
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.NewWordsMode != api.LearnPreferencesNewWordsModeManual || p.NewWordsPerDay != 9 || p.CampCoverage < 0.969 || p.CampCoverage > 0.971 {
		t.Fatalf("после сохранения: %s", rec.Body)
	}
	for name, body := range map[string]string{
		"режим":             `{"newWordsMode":"turbo","newWordsPerDay":9,"campCoverage":0.95}`,
		"слов 0":            `{"newWordsMode":"auto","newWordsPerDay":0,"campCoverage":0.95}`,
		"слов 31":           `{"newWordsMode":"auto","newWordsPerDay":31,"campCoverage":0.95}`,
		"покрытие 0.5":      `{"newWordsMode":"auto","newWordsPerDay":9,"campCoverage":0.5}`,
		"покрытие 1":        `{"newWordsMode":"auto","newWordsPerDay":9,"campCoverage":1}`,
		"лишнее поле":       `{"newWordsMode":"auto","newWordsPerDay":9,"campCoverage":0.95,"x":1}`,
		"не JSON":           `не json`,
		"нет обязательного": `{"newWordsMode":"auto"}`,
	} {
		if rec := put(body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: статус %d, тело %s", name, rec.Code, rec.Body)
		}
	}
	if e.store.learnPrefs.NewWordsPerDay != 9 {
		t.Fatal("отказ не должен менять сохранённые настройки")
	}
}

func reviewLearn(t *testing.T, e env, token string, cardID int64, rating int, key string) {
	t.Helper()
	if rec := doReview(e, token, cardID, rating, key); rec.Code != http.StatusOK {
		t.Fatalf("оценка: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestReviewMarksWordLearned(t *testing.T) {
	for _, tc := range []struct {
		rating  int
		learned bool
	}{{1, false}, {2, false}, {3, true}, {4, true}} {
		e, token := learnEnv(t)
		addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
		id := seed(t, e, 1, time.Now().Add(-time.Minute))
		e.store.wl = map[int64]fakeWord{1: {status: "introduced", source: "lesson", introducedAt: time.Now()}}
		reviewLearn(t, e, token, id, tc.rating, "k1")
		got := e.store.wl[1].status
		if tc.learned != (got == "learned") {
			t.Errorf("оценка %d: состояние %q", tc.rating, got)
		}
		if n := len(eventsOfType(e, "word.learned")); tc.learned && n != 1 || !tc.learned && n != 0 {
			t.Errorf("оценка %d: событий word.learned %d", tc.rating, n)
		}
	}
}

func TestReviewLearnedOnceAndOnReplay(t *testing.T) {
	e, token := learnEnv(t)
	addLearnWord(e, 1, "apple", "noun", "A1", 100, true, "яблоко")
	id := seed(t, e, 1, time.Now().Add(-time.Minute))
	e.store.wl = map[int64]fakeWord{1: {status: "introduced", source: "lesson", introducedAt: time.Now()}}
	reviewLearn(t, e, token, id, 3, "k1")
	reviewLearn(t, e, token, id, 3, "k1") // повтор запроса по ключу
	if n := len(eventsOfType(e, "word.learned")); n != 1 {
		t.Fatalf("событий word.learned %d, ждали 1", n)
	}
	// следующее хорошее повторение (другой ключ) слово уже изучено: второго события нет
	e.store.cards[id] = func() db.Card { c := e.store.cards[id]; c.Due = ts(time.Now().Add(-time.Minute)); return c }()
	reviewLearn(t, e, token, id, 4, "k2")
	if n := len(eventsOfType(e, "word.learned")); n != 1 {
		t.Fatalf("после второго повторения событий word.learned %d, ждали 1", n)
	}
}

func TestReviewCreatesLearnedWordForManualCard(t *testing.T) {
	e, token := learnEnv(t)
	e.store.addLexeme(1, "apple", "noun", "A1")
	id, err := e.store.InsertCard(context.Background(), insertParams(1, 1, srs.New(time.Now().Add(-time.Minute))))
	if err != nil {
		t.Fatal(err)
	}
	reviewLearn(t, e, token, id, 3, "k1")
	if w := e.store.wl[1]; w.status != "learned" || w.source != "card" {
		t.Fatalf("карточка, добавленная вручную: %+v", w)
	}
}

func TestReviewKeepsKnownAtStart(t *testing.T) {
	e, token := learnEnv(t)
	e.store.addLexeme(1, "apple", "noun", "A1")
	id := seed(t, e, 1, time.Now().Add(-time.Minute))
	e.store.wl = map[int64]fakeWord{1: {status: "known_at_start", source: "placement"}}
	reviewLearn(t, e, token, id, 4, "k1")
	if w := e.store.wl[1]; w.status != "known_at_start" || len(eventsOfType(e, "word.learned")) != 0 {
		t.Fatalf("известное с начала слово остаётся таким: %+v", w)
	}
}

func TestLearnDayBoundaryUsesUserTimezone(t *testing.T) {
	e, token := learnEnv(t)
	// Часовой пояс Токио: сутки начались в 15:00 UTC предыдущего дня. Слово, введённое 5 часов назад, может принадлежать
	// прошлым суткам пользователя; проверяем, что граница считается по поясу, а не по UTC.
	e.store.settings.Timezone = "Pacific/Kiritimati" // UTC+14
	loc, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Skip("нет базы часовых поясов")
	}
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	e.store.wl = map[int64]fakeWord{
		1: {status: "introduced", source: "lesson", introducedAt: startOfDay.Add(time.Minute)},
		2: {status: "introduced", source: "lesson", introducedAt: startOfDay.Add(-time.Minute)},
	}
	s := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
	if s.NewToday != 1 {
		t.Fatalf("за сутки пользователя введено %d, ждали 1 (границу даёт его пояс)", s.NewToday)
	}
}
