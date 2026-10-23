package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) ListDataSources(context.Context) ([]db.DataSource, error) {
	return f.sources, nil
}

const sourcesPath = "/api/v1/sources"

func rowCount(n int32) *int32 { return &n }

func TestSourcesNeedSession(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(http.MethodGet, sourcesPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("статус %d, ожидали 401", rec.Code)
	}
}

func TestListSourcesEmptyIsAnEmptyArray(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, sourcesPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"sources":[]`) {
		t.Fatalf("пустой список должен быть [], а не null: %s", rec.Body)
	}
}

func TestListSourcesReturnsFields(t *testing.T) {
	e := newEnv(t)
	imported := time.Date(2026, 10, 23, 9, 25, 15, 0, time.UTC)
	e.store.sources = []db.DataSource{
		{
			ID: 1, Name: "cefrj", Version: "1.5", License: "Бесплатно с указанием источника",
			Url: "https://example.com/cefrj.csv", Attribution: "The CEFR-J Wordlist Version 1.5.",
			RowCount: rowCount(7799), ImportedAt: pgtype.Timestamptz{Time: imported, Valid: true},
		},
	}
	token := login(t, e)
	rec := e.do(http.MethodGet, sourcesPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var got api.SourceList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("источников %d", len(got.Sources))
	}
	s := got.Sources[0]
	if s.Name != "cefrj" || s.Version != "1.5" || s.License != "Бесплатно с указанием источника" ||
		s.Url != "https://example.com/cefrj.csv" || s.Attribution != "The CEFR-J Wordlist Version 1.5." ||
		s.RowCount != 7799 || !s.ImportedAt.Equal(imported) {
		t.Fatalf("источник: %+v", s)
	}
}

func TestListSourcesSkipsUnfinished(t *testing.T) {
	e := newEnv(t)
	e.store.sources = []db.DataSource{
		{ID: 1, Name: "half", Version: "1"}, // импорт не завершён: imported_at пуст
		{ID: 2, Name: "done", Version: "1", RowCount: rowCount(3), ImportedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}
	token := login(t, e)
	rec := e.do(http.MethodGet, sourcesPath, "", withCookie(token))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "half") || !strings.Contains(rec.Body.String(), "done") {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
}
