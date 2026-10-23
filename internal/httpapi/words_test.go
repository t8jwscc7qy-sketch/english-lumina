package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) SearchLexemes(_ context.Context, arg db.SearchLexemesParams) ([]db.SearchLexemesRow, error) {
	f.searches++
	f.lastSearch = &arg
	return f.lexemes, nil
}

// Как настоящий запрос: только строки запрошенных слов.
func (f *fakeStore) ListLexemeLevels(_ context.Context, ids []int64) ([]db.ListLexemeLevelsRow, error) {
	var out []db.ListLexemeLevelsRow
	for _, row := range f.levels {
		if slices.Contains(ids, row.LexemeID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func wordsPath(q string) string { return "/api/v1/words?q=" + url.QueryEscape(q) }

func decodeWords(t *testing.T, body string) api.WordList {
	t.Helper()
	var list api.WordList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("тело %q: %v", body, err)
	}
	return list
}

func TestWordsNeedSession(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(http.MethodGet, wordsPath("run"), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", rec.Code)
	}
	if e.store.searches != 0 {
		t.Fatal("поиск выполнился без сессии")
	}
}

func TestWordsEmptyQueryReturnsEmptyArrayWithoutSearching(t *testing.T) {
	for _, path := range []string{"/api/v1/words", wordsPath(""), wordsPath("   ")} {
		e := newEnv(t)
		token := login(t, e)
		rec := e.do(http.MethodGet, path, "", withCookie(token))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"words":[]`) {
			t.Fatalf("%s: статус %d, тело %s", path, rec.Code, rec.Body)
		}
		if e.store.searches != 0 {
			t.Fatalf("%s: пустой запрос дошёл до базы", path)
		}
	}
}

func TestWordsRejectsBadQueries(t *testing.T) {
	tests := map[string]string{
		"65 символов":  strings.Repeat("x", 65),
		"нулевой байт": "ru\x00n",
		"не UTF-8":     "ru\xffn",
	}
	for name, q := range tests {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			token := login(t, e)
			rec := e.do(http.MethodGet, wordsPath(q), "", withCookie(token))
			if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeInvalidRequest {
				t.Fatalf("статус %d, тело %s; ожидали 400 invalid_request", rec.Code, rec.Body)
			}
			if e.store.searches != 0 {
				t.Fatal("плохой запрос дошёл до базы")
			}
		})
	}
}

func TestWordsPassesPatternsAndLimit(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	if rec := e.do(http.MethodGet, wordsPath("Ru_n%"), "", withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	want := db.SearchLexemesParams{Exact: "ru_n%", Prefix: `ru\_n\%%`, Variant: `%/ru\_n\%%`, MaxRows: 50}
	if e.store.lastSearch == nil || *e.store.lastSearch != want {
		t.Fatalf("параметры поиска %+v, ожидали %+v", e.store.lastSearch, want)
	}
}

func TestWordsAssemblesLevelsAndNotes(t *testing.T) {
	e := newEnv(t)
	e.store.lexemes = []db.SearchLexemesRow{
		{ID: 1, Lemma: "complexity", Pos: "noun", Level: "B2"},
		{ID: 2, Lemma: "cast", Pos: "verb", Level: "C1"},
		{ID: 3, Lemma: "bad", Pos: "adjective", Level: "A2"},
	}
	e.store.levels = []db.ListLexemeLevelsRow{
		{LexemeID: 1, Source: "cefrj", Level: "B2", Extra: []byte(`{"coreInventory1":"Education"}`)},
		{LexemeID: 1, Source: "octanove", Level: "C1", Extra: []byte(`{"notes":["x"]}`)},
		{LexemeID: 2, Source: "octanove", Level: "C1", Extra: []byte(`{"notes":["hire actors","say or suggest"]}`)},
		{LexemeID: 3, Source: "cefrj", Level: "A2", Extra: []byte(`не json`)},
	}
	token := login(t, e)
	rec := e.do(http.MethodGet, wordsPath("c"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	list := decodeWords(t, rec.Body.String())
	if len(list.Words) != 3 {
		t.Fatalf("слов %d", len(list.Words))
	}
	// Порядок как в ответе запроса; уровень слова — тот, что вернул запрос (самый низкий).
	w := list.Words[0]
	if w.Lemma != "complexity" || w.Pos != "noun" || w.Level != api.CefrLevelB2 || len(w.Levels) != 2 {
		t.Fatalf("complexity: %+v", w)
	}
	if w.Levels[0].Source != "cefrj" || w.Levels[0].Level != api.CefrLevelB2 || len(w.Levels[0].Notes) != 0 {
		t.Fatalf("оценка cefrj: %+v", w.Levels[0])
	}
	if w.Levels[1].Source != "octanove" || w.Levels[1].Level != api.CefrLevelC1 || strings.Join(w.Levels[1].Notes, "|") != "x" {
		t.Fatalf("оценка octanove: %+v", w.Levels[1])
	}
	if got := list.Words[1].Levels[0].Notes; strings.Join(got, "|") != "hire actors|say or suggest" {
		t.Fatalf("заметки cast: %v", got)
	}
	// Битый extra не ломает ответ: заметок нет.
	if bad := list.Words[2]; bad.Lemma != "bad" || len(bad.Levels) != 1 || len(bad.Levels[0].Notes) != 0 {
		t.Fatalf("bad: %+v", bad)
	}
	// Без заметок это пустой массив, а не null.
	if !strings.Contains(rec.Body.String(), `"notes":[]`) || strings.Contains(rec.Body.String(), `"notes":null`) {
		t.Fatalf("notes должен быть [], тело %s", rec.Body)
	}
}

func TestWordsMarkWordsThatAreInCards(t *testing.T) {
	e := newEnv(t)
	e.store.lexemes = []db.SearchLexemesRow{
		{ID: 1, Lemma: "run", Pos: "verb", Level: "A1"},
		{ID: 2, Lemma: "runner", Pos: "noun", Level: "B1"},
		{ID: 3, Lemma: "runway", Pos: "noun", Level: "B2"},
	}
	e.store.levels = []db.ListLexemeLevelsRow{
		{LexemeID: 1, Source: "cefrj", Level: "A1", Extra: []byte(`{}`)},
		{LexemeID: 2, Source: "cefrj", Level: "B1", Extra: []byte(`{}`)},
		{LexemeID: 3, Source: "cefrj", Level: "B2", Extra: []byte(`{}`)},
	}
	e.store.cards = map[int64]db.Card{
		1: {ID: 1, UserID: 1, LexemeID: 1},
		2: {ID: 2, UserID: 2, LexemeID: 2}, // карточка другого пользователя не считается
	}
	token := login(t, e)
	rec := e.do(http.MethodGet, wordsPath("run"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	got := map[string]bool{}
	for _, w := range decodeWords(t, rec.Body.String()).Words {
		got[w.Lemma] = w.InCards
	}
	if !got["run"] || got["runner"] || got["runway"] {
		t.Fatalf("inCards: %v, ожидали только run", got)
	}
	if !strings.Contains(rec.Body.String(), `"inCards":false`) {
		t.Fatalf("inCards должен быть в ответе всегда, в том числе false: %s", rec.Body)
	}
}

func TestWordsWithoutResultsDoNotAskForCards(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	for _, q := range []string{"zebra", ""} {
		if rec := e.do(http.MethodGet, wordsPath(q), "", withCookie(token)); rec.Code != http.StatusOK {
			t.Fatalf("q=%q: статус %d", q, rec.Code)
		}
	}
	if e.store.listCardCalls != 0 {
		t.Fatalf("запрос карточек выполнен %d раз без найденных слов", e.store.listCardCalls)
	}
}
