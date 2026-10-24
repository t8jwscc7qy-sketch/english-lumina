package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) ListExamples(_ context.Context, arg db.ListExamplesParams) ([]db.ListExamplesRow, error) {
	f.exampleCalls++
	f.lastExamples = &arg
	if f.examplesFor != nil {
		return f.examplesFor(arg.Query), nil
	}
	return f.examples, nil
}

func examplesPath(lemma, pos string) string {
	return "/api/v1/words/examples?lemma=" + url.QueryEscape(lemma) + "&pos=" + url.QueryEscape(pos)
}

func TestExamplesNeedSession(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(http.MethodGet, examplesPath("run", "verb"), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", rec.Code)
	}
	if e.store.exampleCalls != 0 {
		t.Fatal("запрос выполнился без сессии")
	}
}

func TestExamplesRejectBadInput(t *testing.T) {
	tests := map[string]string{
		"нет леммы":            "/api/v1/words/examples?pos=verb",
		"пустая лемма":         examplesPath("   ", "verb"),
		"лемма из 65 символов": examplesPath(strings.Repeat("x", 65), "verb"),
		"нулевой байт":         examplesPath("ru\x00n", "verb"),
		"не UTF-8":             examplesPath("ru\xffn", "verb"),
		"часть речи из 33":     examplesPath("run", strings.Repeat("x", 33)),
	}
	for name, path := range tests {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			token := login(t, e)
			rec := e.do(http.MethodGet, path, "", withCookie(token))
			if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeInvalidRequest {
				t.Fatalf("статус %d, тело %s; ожидали 400 invalid_request", rec.Code, rec.Body)
			}
			if e.store.exampleCalls != 0 {
				t.Fatal("плохой запрос дошёл до базы")
			}
		})
	}
}

func TestExamplesWithoutSearchableWordsIsEmptyWithoutQuery(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, examplesPath("-- ..", "noun"), "", withCookie(token))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"examples":[]`) {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if e.store.exampleCalls != 0 {
		t.Fatal("для леммы без слов запрос не нужен")
	}
}

func TestExamplesPassesFormsAndLimit(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	if rec := e.do(http.MethodGet, examplesPath("Run", "verb"), "", withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	want := db.ListExamplesParams{Query: `'ran' | 'run' | 'running' | 'runs'`, MaxRows: 5}
	if e.store.lastExamples == nil || *e.store.lastExamples != want {
		t.Fatalf("параметры %+v, ожидали %+v", e.store.lastExamples, want)
	}
}

// Попытка вставить синтаксис tsquery через лемму не ломает запрос: из неё берутся только слова.
func TestExamplesNeutralizeQuerySyntax(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	if rec := e.do(http.MethodGet, examplesPath("run' | 'x & !y:*", "noun"), "", withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if got := e.store.lastExamples.Query; got != `'run' <-> 'x' <-> 'y'` {
		t.Fatalf("запрос %q: знаки синтаксиса должны уйти, остаться фраза из слов", got)
	}
}

func TestExamplesMapsRowsAndHighlight(t *testing.T) {
	e := newEnv(t)
	e.store.examples = []db.ListExamplesRow{
		{EnTatoebaID: 11, EnText: "I like to run every morning.", EnAuthor: "CK", RuTatoebaID: 22, RuText: "Я люблю бегать по утрам.", RuAuthor: "marafon"},
		{EnTatoebaID: 12, EnText: "Nothing matches here at all.", EnAuthor: "CM", RuTatoebaID: 23, RuText: "Ничего не совпадает.", RuAuthor: "ksenija"},
	}
	token := login(t, e)
	rec := e.do(http.MethodGet, examplesPath("run", "verb"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var list api.ExampleList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Examples) != 2 {
		t.Fatalf("тело %s (%v)", rec.Body, err)
	}
	first := list.Examples[0]
	if first.En.Text != "I like to run every morning." || first.En.Author != "CK" || first.En.TatoebaId != 11 ||
		first.Ru.Text != "Я люблю бегать по утрам." || first.Ru.Author != "marafon" || first.Ru.TatoebaId != 22 {
		t.Fatalf("пример: %+v", first)
	}
	if first.Highlight == nil || first.Highlight.Start != 10 || first.Highlight.End != 13 {
		t.Fatalf("подсветка: %+v, ожидали 10–13", first.Highlight)
	}
	if list.Examples[1].Highlight != nil {
		t.Fatalf("без совпадения подсветки быть не должно: %+v", list.Examples[1].Highlight)
	}
}
