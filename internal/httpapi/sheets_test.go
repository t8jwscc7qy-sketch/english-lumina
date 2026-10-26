package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	sheetsPath    = "/api/v1/sheets"
	sheetOpenPath = "/api/v1/sheets/open"
	sheetFavPath  = "/api/v1/sheets/favorite"
	sheetRevPath  = "/api/v1/sheets/review"
)

func sheetsEnv(t *testing.T) (env, string) {
	t.Helper()
	e := newExerciseEnv(t)
	return e, login(t, e)
}

func openSheet(t *testing.T, e env, token, id string) api.Sheet {
	t.Helper()
	rec := e.do(http.MethodPost, sheetOpenPath, `{"id":"`+id+`"}`, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("open %s: статус %d, тело %s", id, rec.Code, rec.Body)
	}
	var s api.Sheet
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func listSheets(t *testing.T, e env, token string) map[string]api.SheetSummary {
	t.Helper()
	rec := e.do(http.MethodGet, sheetsPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("список: статус %d, тело %s", rec.Code, rec.Body)
	}
	var l api.SheetList
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	out := map[string]api.SheetSummary{}
	for _, s := range l.Sheets {
		out[s.Id] = s
	}
	return out
}

func tableRows(t *testing.T, s api.Sheet, bind api.SheetBlockBind) []api.SheetRow {
	t.Helper()
	for _, b := range s.Blocks {
		if b.Bind != nil && *b.Bind == bind && b.Rows != nil {
			return *b.Rows
		}
	}
	t.Fatalf("в шпаргалке %s нет таблицы с привязкой %s", s.Id, bind)
	return nil
}

func rowStatus(rows []api.SheetRow, key string) string {
	for _, r := range rows {
		if r.Key != nil && *r.Key == key && r.Status != nil {
			return string(*r.Status)
		}
	}
	return "нет строки"
}

func TestSheetsNeedSession(t *testing.T) {
	e, _ := sheetsEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, sheetsPath, ""}, {http.MethodPost, sheetOpenPath, `{"id":"to-be"}`},
		{http.MethodPut, sheetFavPath, `{"id":"to-be","favorite":true}`}, {http.MethodPost, sheetRevPath, `{"id":"to-be"}`},
	} {
		if rec := e.do(c.method, c.path, c.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s без сессии: %d", c.method, c.path, rec.Code)
		}
	}
}

func TestSheetsListHasEmbeddedSheets(t *testing.T) {
	e, token := sheetsEnv(t)
	l := listSheets(t, e, token)
	for _, id := range []string{"to-be", "irregular-verbs", "articles", "mine-mistakes"} {
		s, ok := l[id]
		if !ok || s.Favorite || s.Status != "draft" || s.WeakCount != 0 || s.Title == "" || s.Tags == nil {
			t.Errorf("%s: %+v (ok=%v)", id, s, ok)
		}
	}
}

func TestSheetOpenColorsWordsByUserData(t *testing.T) {
	e, token := sheetsEnv(t)
	e.store.addLexeme(1, "go", "verb", "A1")
	e.store.addLexeme(2, "see", "verb", "A1")
	e.store.addLexeme(3, "take", "verb", "A1")
	e.store.addLexeme(4, "give", "verb", "A1")
	e.store.wl = map[int64]fakeWord{
		1: {status: "learned", source: "lesson"},
		3: {status: "known_at_start", source: "placement"},
		4: {status: "introduced", source: "lesson"},
	}
	cardGo := seed(t, e, 1, time.Now().Add(time.Hour))
	c := e.store.cards[cardGo]
	c.Lapses = 3 // go изучено, но забывали 3 раза: трудное
	e.store.cards[cardGo] = c
	seed(t, e, 4, time.Now().Add(time.Hour)) // give: есть карточка, введено

	s := openSheet(t, e, token, "irregular-verbs")
	rows := tableRows(t, s, api.SheetBlockBindWord)
	want := map[string]string{"go": "hard", "see": "unseen", "take": "known_at_start", "give": "introduced", "run": "unseen", "be": "unseen"}
	for key, st := range want {
		if got := rowStatus(rows, key); got != st {
			t.Errorf("%s: %q, ждали %q", key, got, st)
		}
	}
	if s.WeakCount != 1 || s.Status != "draft" || len(s.Sources) == 0 {
		t.Fatalf("слабых %d, статус %s: %+v", s.WeakCount, s.Status, s)
	}
	if got := listSheets(t, e, token)["irregular-verbs"].WeakCount; got != 1 {
		t.Fatalf("в списке слабых %d", got)
	}
}

func TestSheetOpenTopicRatingAndWeakness(t *testing.T) {
	e, token := sheetsEnv(t)
	rate := func(topic string, r float64) {
		if e.store.topicRatings == nil {
			e.store.topicRatings = map[topicKey]db.TopicRating{}
		}
		e.store.topicRatings[topicKey{1, topic}] = db.TopicRating{UserID: 1, Topic: topic, Rating: r, Deviation: 100}
	}
	if st := rowStatus(tableRows(t, openSheet(t, e, token, "articles"), api.SheetBlockBindTopic), "grammar:articles"); st != "unseen" {
		t.Fatalf("без рейтинга: %s", st)
	}
	rate("grammar:articles", 1200)
	rate("grammar:prepositions", 1500)
	rate("skill:order", 1700)
	s := openSheet(t, e, token, "articles")
	row := tableRows(t, s, api.SheetBlockBindTopic)[0]
	if row.Status == nil || *row.Status != "weak" || row.Rating == nil || *row.Rating != 1200 || s.WeakCount != 1 {
		t.Fatalf("слабая тема: %+v, слабых %d", row, s.WeakCount)
	}
	rate("grammar:articles", 1600)
	if st := rowStatus(tableRows(t, openSheet(t, e, token, "articles"), api.SheetBlockBindTopic), "grammar:articles"); st != "ok" {
		t.Fatalf("сильная тема: %s", st)
	}
}

func TestSheetMineMistakesFromBank(t *testing.T) {
	e, token := sheetsEnv(t)
	s := openSheet(t, e, token, "mine-mistakes")
	if rows := tableRows(t, s, api.SheetBlockBindMistake); len(rows) != 0 || rows == nil {
		t.Fatalf("пустой банк: %#v", rows)
	}
	seedMistake(e, 1, time.Now().Add(time.Hour), 0)
	s = openSheet(t, e, token, "mine-mistakes")
	rows := tableRows(t, s, api.SheetBlockBindMistake)
	if len(rows) != 1 || !slices.Equal(rows[0].Cells, []string{"I have _____ dog.", "the", "a"}) || s.WeakCount != 1 {
		t.Fatalf("строки: %+v, слабых %d", rows, s.WeakCount)
	}
}

func TestSheetFavoriteIsIdempotentAndWritesOneEvent(t *testing.T) {
	e, token := sheetsEnv(t)
	put := func(fav bool) *struct{ code int } {
		body := `{"id":"to-be","favorite":false}`
		if fav {
			body = `{"id":"to-be","favorite":true}`
		}
		rec := e.do(http.MethodPut, sheetFavPath, body, withCookie(token))
		return &struct{ code int }{rec.Code}
	}
	for range 2 {
		if put(true).code != http.StatusOK {
			t.Fatal("PUT должен проходить дважды")
		}
	}
	if !listSheets(t, e, token)["to-be"].Favorite || len(eventsOfType(e, "sheet.favorited")) != 1 {
		t.Fatalf("после двух PUT: событий %d", len(eventsOfType(e, "sheet.favorited")))
	}
	for range 2 {
		if put(false).code != http.StatusOK {
			t.Fatal("снятие дважды")
		}
	}
	if listSheets(t, e, token)["to-be"].Favorite || len(eventsOfType(e, "sheet.favorited")) != 2 {
		t.Fatalf("после снятия: событий %d", len(eventsOfType(e, "sheet.favorited")))
	}
}

func TestSheetReviewMarksOnceAndShowsReviewed(t *testing.T) {
	e, token := sheetsEnv(t)
	for i := 0; i < 2; i++ {
		rec := e.do(http.MethodPost, sheetRevPath, `{"id":"articles"}`, withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
		}
	}
	if len(eventsOfType(e, "sheet.reviewed")) != 1 {
		t.Fatalf("событий %d, ждали 1", len(eventsOfType(e, "sheet.reviewed")))
	}
	if got := listSheets(t, e, token)["articles"].Status; got != "reviewed" {
		t.Fatalf("статус в списке %s", got)
	}
	if s := openSheet(t, e, token, "articles"); s.Status != "reviewed" {
		t.Fatalf("статус в шпаргалке %s", s.Status)
	}
	if got := listSheets(t, e, token)["to-be"].Status; got != "draft" {
		t.Fatalf("другая шпаргалка остаётся черновиком: %s", got)
	}
}

func TestSheetOpenRemembersTime(t *testing.T) {
	e, token := sheetsEnv(t)
	if listSheets(t, e, token)["to-be"].OpenedAt != nil {
		t.Fatal("до открытия времени нет")
	}
	openSheet(t, e, token, "to-be")
	if listSheets(t, e, token)["to-be"].OpenedAt == nil {
		t.Fatal("после открытия время есть")
	}
}

func TestSheetUnknownAndBadInput(t *testing.T) {
	e, token := sheetsEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, sheetOpenPath, `{"id":"nope"}`}, {http.MethodPut, sheetFavPath, `{"id":"nope","favorite":true}`}, {http.MethodPost, sheetRevPath, `{"id":"nope"}`},
	} {
		rec := e.do(c.method, c.path, c.body, withCookie(token))
		if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNotFound {
			t.Errorf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}
	for _, body := range []string{``, `{}`, `не json`, `{"id":"to-be","extra":1}`, `{"id":""}`} {
		if rec := e.do(http.MethodPost, sheetOpenPath, body, withCookie(token)); rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%q: %d", body, rec.Code)
		}
	}
	if len(e.store.sheetStates) != 0 {
		t.Fatalf("после отказов ничего не записано: %+v", e.store.sheetStates)
	}
	if rec := e.do(http.MethodPut, sheetFavPath, `{"id":"to-be","favorite":true}`, withHeader("Origin", "https://evil.example"), withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("чужой источник: %d", rec.Code)
	}
}
