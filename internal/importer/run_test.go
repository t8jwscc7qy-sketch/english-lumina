package importer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

type fakeRow struct{ Lemma, Pos, Level string }

// fakeSource пишет строки так же, как настоящий модуль: слово + уровень.
type fakeSource struct {
	info     Info
	rows     []fakeRow
	failAt   int // номер вызова Load (с 1), на котором Load падает; 0 — не падать
	fetchErr error
	onLoad   func(n int) // вызывается перед записью строки n (с 1)
	loads    int
}

func newFake(name string, rows ...fakeRow) *fakeSource {
	return &fakeSource{info: Info{Name: name, Version: "1", License: "test", URL: "https://example.com/" + name, Attribution: "attr " + name}, rows: rows}
}

func (f *fakeSource) Describe() Info { return f.info }

func (f *fakeSource) Fetch(_ context.Context, cacheDir string) (string, error) {
	if f.fetchErr != nil {
		return "", f.fetchErr
	}
	p := filepath.Join(cacheDir, f.info.Name+".txt")
	return p, os.WriteFile(p, []byte("x"), 0o600)
}

func (f *fakeSource) Parse(_ context.Context, _ io.Reader, emit func(any) error) error {
	for _, r := range f.rows {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeSource) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	f.loads++
	if f.onLoad != nil {
		f.onLoad(f.loads)
	}
	if f.failAt != 0 && f.loads == f.failAt {
		return errors.New("сломано специально")
	}
	r := row.(fakeRow)
	id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: r.Lemma, Pos: r.Pos, SourceID: sourceID})
	if err != nil {
		return err
	}
	return q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: sourceID, Level: r.Level, Extra: []byte(`{}`)})
}

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func three() []fakeRow {
	return []fakeRow{{"run", "verb", "A1"}, {"book", "noun", "A2"}, {"quick", "adjective", "B1"}}
}

func TestRunLoadsRowsAndFillsSource(t *testing.T) {
	pool := testdb.Pool(t)
	res, err := Run(context.Background(), pool, newFake("fake", three()...), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 3 || res.Info.Name != "fake" || res.Duration <= 0 {
		t.Fatalf("Result = %+v", res)
	}
	if count(t, pool, "lexemes") != 3 || count(t, pool, "lexeme_cefr") != 3 {
		t.Fatalf("lexemes=%d lexeme_cefr=%d", count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
	ds, err := db.New(pool).GetDataSource(context.Background(), db.GetDataSourceParams{Name: "fake", Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.RowCount == nil || *ds.RowCount != 3 || !ds.ImportedAt.Valid || ds.Attribution != "attr fake" {
		t.Fatalf("data_sources = %+v", ds)
	}
}

func TestRunTwiceSameVersionDoesNotDuplicate(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := Run(ctx, pool, newFake("fake", three()...), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	first, _ := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "fake", Version: "1"})
	if _, err := Run(ctx, pool, newFake("fake", three()...), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	second, _ := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "fake", Version: "1"})

	if count(t, pool, "data_sources") != 1 || count(t, pool, "lexemes") != 3 || count(t, pool, "lexeme_cefr") != 3 {
		t.Fatalf("после второго запуска: sources=%d lexemes=%d cefr=%d",
			count(t, pool, "data_sources"), count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
	if second.ID != first.ID || !second.ImportedAt.Time.After(first.ImportedAt.Time) {
		t.Fatalf("id %d→%d, imported_at %v→%v: ожидали тот же источник и свежее время", first.ID, second.ID, first.ImportedAt.Time, second.ImportedAt.Time)
	}
}

func TestRunFailureRollsBackEverything(t *testing.T) {
	pool := testdb.Pool(t)
	src := newFake("fake", three()...)
	src.failAt = 3
	_, err := Run(context.Background(), pool, src, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "fake") {
		t.Fatalf("ожидали ошибку с именем источника, получили %v", err)
	}
	for _, table := range []string{"data_sources", "lexemes", "lexeme_cefr"} {
		if n := count(t, pool, table); n != 0 {
			t.Fatalf("после ошибки в %s осталось %d строк", table, n)
		}
	}
}

func TestRunWithoutRowsIsAnError(t *testing.T) {
	pool := testdb.Pool(t)
	_, err := Run(context.Background(), pool, newFake("fake"), t.TempDir())
	if err == nil {
		t.Fatal("источник без строк должен давать ошибку")
	}
	if n := count(t, pool, "data_sources"); n != 0 {
		t.Fatalf("data_sources: %d строк, ожидали 0", n)
	}
}

func TestRunSharedLexemeKeepsFirstSource(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := Run(ctx, pool, newFake("first", fakeRow{"run", "verb", "A1"}), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, pool, newFake("second", fakeRow{"run", "verb", "B2"}), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	first, _ := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "first", Version: "1"})
	var lexemes int
	var owner int64
	if err := pool.QueryRow(ctx, "SELECT count(*), max(source_id) FROM lexemes").Scan(&lexemes, &owner); err != nil {
		t.Fatal(err)
	}
	if lexemes != 1 || owner != first.ID {
		t.Fatalf("lexemes=%d, source_id=%d, ожидали 1 слово от первого источника (%d)", lexemes, owner, first.ID)
	}
	if n := count(t, pool, "lexeme_cefr"); n != 2 {
		t.Fatalf("lexeme_cefr: %d, ожидали по строке на источник", n)
	}
}

func TestRunFetchErrorDoesNotTouchDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	src := newFake("fake", three()...)
	src.fetchErr = errors.New("нет сети")
	if _, err := Run(context.Background(), pool, src, t.TempDir()); err == nil || !strings.Contains(err.Error(), "нет сети") {
		t.Fatalf("ожидали ошибку Fetch, получили %v", err)
	}
	if n := count(t, pool, "data_sources"); n != 0 {
		t.Fatalf("data_sources: %d", n)
	}
}

func TestRunCanceledContextRollsBack(t *testing.T) {
	pool := testdb.Pool(t)
	ctx, cancel := context.WithCancel(context.Background())
	src := newFake("fake", three()...)
	src.onLoad = func(n int) {
		if n == 2 {
			cancel()
		}
	}
	if _, err := Run(ctx, pool, src, t.TempDir()); err == nil {
		t.Fatal("ожидали ошибку отмены")
	}
	for _, table := range []string{"data_sources", "lexemes", "lexeme_cefr"} {
		if n := count(t, pool, table); n != 0 {
			t.Fatalf("после отмены в %s осталось %d строк", table, n)
		}
	}
}
