package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	textNextPath = "/api/v1/texts/next"
	textRatePath = "/api/v1/texts/rate"
	textKnowPath = "/api/v1/texts/know-word"
)

// textsEnv — сервер с nKnown известными словами kwNN и шестью целевыми словами tw00…tw05 (по три предложения у каждого).
func textsEnv(t *testing.T, nKnown int) (env, string) {
	t.Helper()
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.wl = map[int64]fakeWord{}
	for i := 0; i < nKnown; i++ {
		id := int64(100 + i)
		e.store.addLexeme(id, fmt.Sprintf("kw%02d", i), "noun", "A1")
		e.store.wl[id] = fakeWord{status: "learned", source: "lesson"}
	}
	var rows []db.ListExamplesRow
	for w := 0; w < 6; w++ {
		target := fmt.Sprintf("tw%02d", w)
		payload, _ := json.Marshal(map[string]any{"lemma": target, "translations": []string{"перевод-" + target}, "ipa": "ˈ" + target})
		e.store.textTargets = append(e.store.textTargets, db.TextTargetWordsRow{LexemeID: int64(500 + w), Lemma: target, Pos: "noun", Payload: payload})
		e.store.addLexeme(int64(500+w), target, "noun", "A2")
		for s := 0; s < 3; s++ {
			id := int64(w*10 + s + 1)
			k := func(j int) string { return fmt.Sprintf("kw%02d", (w*3+s*2+j)%max(nKnown, 1)) }
			text := fmt.Sprintf("%s and %s %s with %s %s.", k(0), k(1), target, k(2), k(3))
			rows = append(rows, db.ListExamplesRow{EnID: id, EnTatoebaID: id + 1000, EnText: text, EnAuthor: "tom", RuTatoebaID: id + 2000, RuText: "перевод " + text, RuAuthor: "ivan"})
		}
	}
	e.store.examplesFor = func(q string) []db.ListExamplesRow {
		var out []db.ListExamplesRow
		for _, r := range rows {
			for w := 0; w < 6; w++ {
				if strings.Contains(q, fmt.Sprintf("'tw%02d'", w)) && strings.Contains(r.EnText, fmt.Sprintf("tw%02d", w)) {
					out = append(out, r)
				}
			}
		}
		return out
	}
	return e, token
}

func nextText(t *testing.T, e env, token, query string) (api.ReadText, int, string) {
	t.Helper()
	rec := e.do(http.MethodGet, textNextPath+query, "", withCookie(token))
	var text api.ReadText
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &text); err != nil {
			t.Fatalf("тело %s: %v", rec.Body, err)
		}
	}
	return text, rec.Code, rec.Body.String()
}

func TestTextNeedsSession(t *testing.T) {
	e, _ := textsEnv(t, 25)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, textNextPath, ""}, {http.MethodPost, textRatePath, `{"textId":1,"understood":"ok","mode":"read"}`},
		{http.MethodPost, textKnowPath, `{"lexemeId":1}`},
	} {
		if rec := e.do(c.method, c.path, c.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d", c.path, rec.Code)
		}
	}
}

func TestTextNextBuildsTextAtGoal(t *testing.T) {
	e, token := textsEnv(t, 25)
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", code, body)
	}
	if text.Id == 0 || text.Mode != api.ReadTextModeRead || text.Goal < 0.849 || text.Goal > 0.851 || text.Coverage < 0.80 || text.Coverage > 0.90 {
		t.Fatalf("текст: id=%d mode=%s goal=%v coverage=%v", text.Id, text.Mode, text.Goal, text.Coverage)
	}
	if len(text.Targets) < 3 || len(text.Targets) > 6 || len(text.Sentences) < 6 || len(text.Sentences) > 10 {
		t.Fatalf("целевых слов %d, предложений %d", len(text.Targets), len(text.Sentences))
	}
	for _, s := range text.Sentences {
		if s.TatoebaId < 1000 || s.Author != "tom" || s.En == "" || s.Ru == "" || len(s.Tokens) < 4 {
			t.Fatalf("предложение: %+v", s)
		}
	}
	for _, target := range text.Targets {
		i := slices.IndexFunc(text.Hints, func(h api.ReadWordHint) bool { return h.Token == target && h.Target })
		if i < 0 {
			t.Fatalf("нет подсказки для целевого слова %s: %+v", target, text.Hints)
		}
		h := text.Hints[i]
		if h.LexemeId < 500 || len(h.Translations) != 1 || h.Ipa == nil {
			t.Fatalf("подсказка: %+v", h)
		}
	}
	if len(e.store.texts) != 1 {
		t.Fatalf("текст сохранён %d раз", len(e.store.texts))
	}
}

func TestTextNextRespectsModeAndGoalPreference(t *testing.T) {
	e, token := textsEnv(t, 25)
	e.store.learnPrefs = &db.LearnPreference{UserID: 1, NewWordsMode: "auto", NewWordsPerDay: 12, CampCoverage: 0.95, TextCoverage: 0.9}
	text, code, body := nextText(t, e, token, "?mode=listen")
	if code != http.StatusOK || text.Mode != api.ReadTextModeListen || text.Goal < 0.899 || text.Goal > 0.901 {
		t.Fatalf("статус %d, режим %s, цель %v, тело %s", code, text.Mode, text.Goal, body)
	}
	if text, _, _ := nextText(t, e, token, "?mode=bogus"); text.Mode != api.ReadTextModeRead {
		t.Fatalf("неизвестный режим — read: %s", text.Mode)
	}
}

func TestTextNextNoTextCases(t *testing.T) {
	e, token := textsEnv(t, 19) // мало известных слов
	if _, code, body := nextText(t, e, token, ""); code != http.StatusNotFound || errorCode(t, body) != api.ErrorErrorCodeNoText {
		t.Fatalf("19 слов: %d %s", code, body)
	}
	e2, token2 := textsEnv(t, 25)
	e2.store.examplesFor = func(string) []db.ListExamplesRow { return nil }
	if _, code, body := nextText(t, e2, token2, ""); code != http.StatusNotFound || errorCode(t, body) != api.ErrorErrorCodeNoText {
		t.Fatalf("нет предложений: %d %s", code, body)
	}
	e3, token3 := textsEnv(t, 25)
	e3.store.textTargets = nil
	if _, code, _ := nextText(t, e3, token3, ""); code != http.StatusNotFound {
		t.Fatalf("нет целевых слов: %d", code)
	}
}

func TestTextNextDoesNotRepeatRecentSentences(t *testing.T) {
	e, token := textsEnv(t, 25)
	first, code, _ := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatal(code)
	}
	second, code, body := nextText(t, e, token, "?new=1")
	if code != http.StatusOK {
		t.Fatalf("второй текст: %d %s", code, body)
	}
	seen := map[int64]bool{}
	for _, s := range first.Sentences {
		seen[s.TatoebaId] = true
	}
	for _, s := range second.Sentences {
		if seen[s.TatoebaId] {
			t.Fatalf("предложение %d повторилось в следующем тексте", s.TatoebaId)
		}
	}
}

func rateBody(id int64, understood string) string {
	return fmt.Sprintf(`{"textId":%d,"understood":%q,"mode":"read"}`, id, understood)
}

func TestTextRateMovesGoalOnceAndWritesEvent(t *testing.T) {
	e, token := textsEnv(t, 25)
	text, _, _ := nextText(t, e, token, "")
	rec := e.do(http.MethodPost, textRatePath, rateBody(text.Id, "easy"), withCookie(token))
	var r api.RateTextResult
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != http.StatusOK || !r.Changed || r.Goal < 0.829 || r.Goal > 0.831 {
		t.Fatalf("оценка: %d %s", rec.Code, rec.Body)
	}
	again := e.do(http.MethodPost, textRatePath, rateBody(text.Id, "easy"), withCookie(token))
	var r2 api.RateTextResult
	if err := json.Unmarshal(again.Body.Bytes(), &r2); err != nil || r2.Changed || r2.Goal < 0.829 || r2.Goal > 0.831 {
		t.Fatalf("повтор не двигает цель: %s", again.Body)
	}
	if n := len(eventsOfType(e, "text.read")); n != 1 {
		t.Fatalf("событий %d, ждали 1", n)
	}
	next, _, _ := nextText(t, e, token, "")
	if next.Goal < 0.829 || next.Goal > 0.831 {
		t.Fatalf("следующий текст берёт новую цель: %v", next.Goal)
	}
	hard := e.do(http.MethodPost, textRatePath, rateBody(next.Id, "hard"), withCookie(token))
	var r3 api.RateTextResult
	if err := json.Unmarshal(hard.Body.Bytes(), &r3); err != nil || r3.Goal < 0.849 || r3.Goal > 0.851 {
		t.Fatalf("«трудно» поднимает цель: %s", hard.Body)
	}
}

func TestTextRateValidation(t *testing.T) {
	e, token := textsEnv(t, 25)
	text, _, _ := nextText(t, e, token, "")
	for name, body := range map[string]string{
		"ответ вне списка": rateBody(text.Id, "great"), "нулевой номер": rateBody(0, "ok"), "нет режима": fmt.Sprintf(`{"textId":%d,"understood":"ok"}`, text.Id),
		"режим write": fmt.Sprintf(`{"textId":%d,"understood":"ok","mode":"write"}`, text.Id), "лишнее поле": fmt.Sprintf(`{"textId":%d,"understood":"ok","mode":"read","x":1}`, text.Id),
		"хвост": rateBody(text.Id, "ok") + "x", "не json": "не json",
	} {
		if rec := e.do(http.MethodPost, textRatePath, body, withCookie(token)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := e.do(http.MethodPost, textRatePath, rateBody(999, "ok"), withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("чужой или несуществующий текст: %d", rec.Code)
	}
	if len(e.store.textRated) != 0 || len(eventsOfType(e, "text.read")) != 0 {
		t.Fatal("после отказов ничего не записано")
	}
}

func TestTextKnowWord(t *testing.T) {
	e, token := textsEnv(t, 25)
	know := func(id int64) (api.KnowWordResult, int) {
		rec := e.do(http.MethodPost, textKnowPath, fmt.Sprintf(`{"lexemeId":%d}`, id), withCookie(token))
		var r api.KnowWordResult
		_ = json.Unmarshal(rec.Body.Bytes(), &r)
		return r, rec.Code
	}
	e.store.kbCards = map[int64]json.RawMessage{500: json.RawMessage(`{"lemma":"tw00","translations":["x"]}`), 501: json.RawMessage(`{"lemma":"tw01","translations":["y"]}`)}
	if r, code := know(500); code != http.StatusOK || !r.Marked {
		t.Fatalf("отметка: %d %+v", code, r)
	}
	if w := e.store.wl[500]; w.status != "known_at_start" {
		t.Fatalf("состояние: %+v", w)
	}
	if r, code := know(500); code != http.StatusOK || r.Marked {
		t.Fatalf("повтор не меняет: %d %+v", code, r)
	}
	seed(t, e, 501, time.Now().Add(time.Hour))
	if r, code := know(501); code != http.StatusOK || r.Marked {
		t.Fatalf("слово с карточкой не помечается: %d %+v", code, r)
	}
	if _, code := know(777); code != http.StatusNotFound {
		t.Fatalf("слова нет в базе знаний: %d", code)
	}
	for _, body := range []string{`{}`, `{"lexemeId":0}`, `{"lexemeId":1,"x":1}`, `не json`} {
		if rec := e.do(http.MethodPost, textKnowPath, body, withCookie(token)); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: %d", body, rec.Code)
		}
	}
	if n := len(eventsOfType(e, "text.word_added")); n != 1 {
		t.Fatalf("событий %d, ждали 1", n)
	}
}

func TestTextTargetsSkipFunctionWords(t *testing.T) {
	e, token := textsEnv(t, 25)
	payload, _ := json.Marshal(map[string]any{"lemma": "just", "translations": []string{"только"}})
	e.store.textTargets = append([]db.TextTargetWordsRow{{LexemeID: 900, Lemma: "just", Pos: "adverb", Payload: payload}}, e.store.textTargets...)
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK || slices.Contains(text.Targets, "just") {
		t.Fatalf("служебное слово не бывает целевым: %d %v %s", code, text.Targets, body)
	}
}

func TestTextTargetsAreDistinctLemmas(t *testing.T) {
	e, token := textsEnv(t, 25)
	// right как прилагательное и наречие — два слова базы с одной леммой: целевым становится один раз.
	dup := e.store.textTargets[0]
	dup.LexemeID, dup.Pos = 901, "adjective"
	e.store.textTargets = append([]db.TextTargetWordsRow{e.store.textTargets[0], dup}, e.store.textTargets[1:]...)
	text, code, _ := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatal(code)
	}
	seen := map[string]bool{}
	for _, tg := range text.Targets {
		if seen[tg] {
			t.Fatalf("лемма %s дважды среди целевых слов: %v", tg, text.Targets)
		}
		seen[tg] = true
	}
}

func TestTextNextReturnsSameUnratedTextUntilRated(t *testing.T) {
	e, token := textsEnv(t, 25)
	first, _, _ := nextText(t, e, token, "")
	again, code, _ := nextText(t, e, token, "?mode=listen")
	if code != http.StatusOK || again.Id != first.Id || again.Mode != api.ReadTextModeListen || len(again.Sentences) != len(first.Sentences) {
		t.Fatalf("неоценённый текст отдаётся повторно: %d %d (режим %s)", first.Id, again.Id, again.Mode)
	}
	if len(again.Hints) != len(first.Hints) || again.Sentences[0].Author != first.Sentences[0].Author {
		t.Fatal("подсказки и атрибуция сохраняются вместе с текстом")
	}
	if len(e.store.texts) != 1 {
		t.Fatalf("текстов в базе %d, ждали 1", len(e.store.texts))
	}
	forced, _, _ := nextText(t, e, token, "?new=1")
	if forced.Id == first.Id {
		t.Fatal("new=1 собирает новый текст")
	}
	if rec := e.do(http.MethodPost, textRatePath, rateBody(forced.Id, "ok"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	after, _, _ := nextText(t, e, token, "")
	if after.Id == forced.Id {
		t.Fatal("после оценки собирается новый текст")
	}
}

func TestTextCarriesBothSidesAttribution(t *testing.T) {
	e, token := textsEnv(t, 25)
	text, _, _ := nextText(t, e, token, "")
	for _, s := range text.Sentences {
		if s.RuAuthor != "ivan" || s.RuTatoebaId < 2000 {
			t.Fatalf("у русского перевода должны быть автор и номер: %+v", s)
		}
	}
}

func TestTextTargetWithKnownFormIsSkipped(t *testing.T) {
	e, token := textsEnv(t, 25)
	// «kw00» уже известно как слово; такая же лемма другой части речи целевой быть не может.
	payload, _ := json.Marshal(map[string]any{"lemma": "kw00", "translations": []string{"x"}})
	e.store.textTargets = append([]db.TextTargetWordsRow{{LexemeID: 902, Lemma: "kw00", Pos: "verb", Payload: payload}}, e.store.textTargets...)
	text, code, _ := nextText(t, e, token, "")
	if code != http.StatusOK || slices.Contains(text.Targets, "kw00") {
		t.Fatalf("известная форма не бывает целевым словом: %d %v", code, text.Targets)
	}
}

func TestTextNotEnoughSentencesIsNoText(t *testing.T) {
	e, token := textsEnv(t, 25)
	e.store.textTargets = e.store.textTargets[:1] // одно слово с тремя предложениями: меньше шести
	if _, code, body := nextText(t, e, token, ""); code != http.StatusNotFound || errorCode(t, body) != api.ErrorErrorCodeNoText {
		t.Fatalf("мало предложений: %d %s", code, body)
	}
}
