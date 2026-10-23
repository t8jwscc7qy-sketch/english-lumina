package octanove

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func sourceFor(t *testing.T, content, wantSum string) *Source {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)
	return newSource(srv.URL+"/octanove.csv", wantSum, 1<<20, srv.Client())
}

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunImportsSample(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	sample := readSample(t)
	cache := t.TempDir()

	res, err := importer.Run(ctx, pool, sourceFor(t, sample, sha(sample)), cache)
	if err != nil {
		t.Fatal(err)
	}
	// row_count совпадает с тем, что реально лежит в базе (повторы пар уже склеены в Parse).
	if res.Rows != 20 || count(t, pool, "lexemes") != 20 || count(t, pool, "lexeme_cefr") != 20 {
		t.Fatalf("Rows=%d lexemes=%d cefr=%d, ожидали 20", res.Rows, count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
	if _, err := os.Stat(filepath.Join(cache, "sources", "octanove", "octanove-vocabulary-profile-c1c2-1.0.csv")); err != nil {
		t.Fatalf("файл не лежит в кэше: %v", err)
	}
	ds, err := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "octanove", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.RowCount == nil || *ds.RowCount != 20 || !ds.ImportedAt.Valid || !strings.Contains(ds.Attribution, "Octanove Labs") {
		t.Fatalf("data_sources: %+v", ds)
	}

	var level, extra string
	err = pool.QueryRow(ctx, `SELECT c.level, c.extra::text FROM lexeme_cefr c JOIN lexemes l ON l.id = c.lexeme_id WHERE l.lemma = 'cast' AND l.pos = 'verb'`).Scan(&level, &extra)
	if err != nil || level != "C1" || !strings.Contains(extra, "hire actors") || !strings.Contains(extra, "say or suggest something") {
		t.Fatalf("cast/verb: %s %s (%v)", level, extra, err)
	}
	err = pool.QueryRow(ctx, `SELECT c.level FROM lexeme_cefr c JOIN lexemes l ON l.id = c.lexeme_id WHERE l.lemma = 'relinquish'`).Scan(&level)
	if err != nil || level != "C1" {
		t.Fatalf("relinquish: %s (%v)", level, err)
	}
}

func TestRunTwiceKeepsCounts(t *testing.T) {
	pool := testdb.Pool(t)
	sample := readSample(t)
	for i := 0; i < 2; i++ {
		if _, err := importer.Run(context.Background(), pool, sourceFor(t, sample, sha(sample)), t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, pool, "data_sources") != 1 || count(t, pool, "lexemes") != 20 || count(t, pool, "lexeme_cefr") != 20 {
		t.Fatalf("sources=%d lexemes=%d cefr=%d", count(t, pool, "data_sources"), count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
}

// Слово, которое уже есть от другого источника (например CEFR-J): остаётся одно слово с прежним владельцем,
// а у него появляется второй уровень от этого источника. Выбирать «уровень слова» будет читающая сторона.
func TestRunSharedLexemeWithAnotherSource(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	first, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1.5", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "complexity", Pos: "noun", SourceID: first.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: first.ID, Level: "B2", Extra: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}

	content := "headword,pos,CEFR,notes\r\ncomplexity,noun,C1,\r\nexterior,noun,C1,\r\n"
	if _, err := importer.Run(ctx, pool, sourceFor(t, content, sha(content)), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, "lexemes"); n != 2 {
		t.Fatalf("lexemes %d, ожидали 2 (общее слово не должно дублироваться)", n)
	}
	var owner int64
	if err := pool.QueryRow(ctx, `SELECT source_id FROM lexemes WHERE lemma = 'complexity'`).Scan(&owner); err != nil || owner != first.ID {
		t.Fatalf("владелец слова %d (%v), ожидали первый источник %d", owner, err, first.ID)
	}
	rows, err := pool.Query(ctx, `SELECT s.name, c.level FROM lexeme_cefr c JOIN data_sources s ON s.id = c.source_id WHERE c.lexeme_id = $1 ORDER BY s.name`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name, level string
		if err := rows.Scan(&name, &level); err != nil {
			t.Fatal(err)
		}
		got = append(got, name+":"+level)
	}
	if strings.Join(got, ",") != "cefrj:B2,octanove:C1" {
		t.Fatalf("уровни слова: %v", got)
	}
}

func TestRunBadRowRollsBackEverything(t *testing.T) {
	pool := testdb.Pool(t)
	broken := readSample(t) + "oops,noun,B2,\r\n"
	_, err := importer.Run(context.Background(), pool, sourceFor(t, broken, sha(broken)), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "B2") {
		t.Fatalf("ожидали ошибку про уровень B2, получили %v", err)
	}
	for _, table := range []string{"data_sources", "lexemes", "lexeme_cefr"} {
		if n := count(t, pool, table); n != 0 {
			t.Fatalf("после ошибки в %s осталось %d строк", table, n)
		}
	}
}

func TestRunFileWithoutDataIsAnError(t *testing.T) {
	pool := testdb.Pool(t)
	headerOnly := "headword,pos,CEFR,notes\r\n"
	_, err := importer.Run(context.Background(), pool, sourceFor(t, headerOnly, sha(headerOnly)), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "нет строк") {
		t.Fatalf("ожидали «нет строк», получили %v", err)
	}
	if n := count(t, pool, "data_sources"); n != 0 {
		t.Fatalf("data_sources: %d", n)
	}
}

func TestRunChecksumMismatchTouchesNothing(t *testing.T) {
	pool := testdb.Pool(t)
	cache := t.TempDir()
	sample := readSample(t)
	_, err := importer.Run(context.Background(), pool, sourceFor(t, sample, sha("другое")), cache)
	if err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("ожидали ошибку суммы, получили %v", err)
	}
	if n := count(t, pool, "data_sources"); n != 0 {
		t.Fatalf("data_sources: %d", n)
	}
	if _, err := os.Stat(filepath.Join(cache, "sources", "octanove", "octanove-vocabulary-profile-c1c2-1.0.csv")); err == nil {
		t.Fatal("файл с неверной суммой остался в кэше")
	}
}
