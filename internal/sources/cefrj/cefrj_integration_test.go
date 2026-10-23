package cefrj

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

// sourceFor отдаёт content с TLS-сервера и настраивает источник на него; сумму считает по content.
func sourceFor(t *testing.T, content, wantSum string) *Source {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)
	return newSource(srv.URL+"/cefrj-vocabulary-profile-1.5.csv", wantSum, 1<<20, srv.Client())
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
	if res.Rows != 32 || count(t, pool, "lexemes") != 32 || count(t, pool, "lexeme_cefr") != 32 {
		t.Fatalf("Rows=%d lexemes=%d cefr=%d, ожидали 32", res.Rows, count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
	if _, err := os.Stat(filepath.Join(cache, "sources", "cefrj", "cefrj-vocabulary-profile-1.5.csv")); err != nil {
		t.Fatalf("файл не лежит в кэше: %v", err)
	}

	ds, err := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "cefrj", Version: "1.5"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.Attribution != New().Describe().Attribution || ds.RowCount == nil || *ds.RowCount != 32 || !ds.ImportedAt.Valid {
		t.Fatalf("data_sources: %+v", ds)
	}
	var wrongOwner int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM lexemes WHERE source_id <> $1", ds.ID).Scan(&wrongOwner); err != nil || wrongOwner != 0 {
		t.Fatalf("слов с чужим source_id: %d (%v)", wrongOwner, err)
	}

	var level, pos, extra string
	err = pool.QueryRow(ctx, `SELECT c.level, l.pos, c.extra::text FROM lexeme_cefr c JOIN lexemes l ON l.id = c.lexeme_id WHERE l.lemma = 'abuse'`).Scan(&level, &pos, &extra)
	if err != nil || level != "B2" || pos != "noun" || !strings.Contains(extra, "News, lifestyles and current affairs") {
		t.Fatalf("abuse: %s %s %s (%v)", level, pos, extra, err)
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
	if count(t, pool, "data_sources") != 1 || count(t, pool, "lexemes") != 32 || count(t, pool, "lexeme_cefr") != 32 {
		t.Fatalf("sources=%d lexemes=%d cefr=%d", count(t, pool, "data_sources"), count(t, pool, "lexemes"), count(t, pool, "lexeme_cefr"))
	}
}

func TestRunBadRowAtEndRollsBackEverything(t *testing.T) {
	pool := testdb.Pool(t)
	broken := readSample(t) + "oops,noun,D1,,,\n"
	_, err := importer.Run(context.Background(), pool, sourceFor(t, broken, sha(broken)), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "D1") {
		t.Fatalf("ожидали ошибку про уровень D1, получили %v", err)
	}
	for _, table := range []string{"data_sources", "lexemes", "lexeme_cefr"} {
		if n := count(t, pool, table); n != 0 {
			t.Fatalf("после ошибки в %s осталось %d строк", table, n)
		}
	}
}

func TestRunFileWithoutDataIsAnError(t *testing.T) {
	pool := testdb.Pool(t)
	headerOnly := "headword,pos,CEFR,CoreInventory 1,CoreInventory 2,Threshold\n"
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
	if _, err := os.Stat(filepath.Join(cache, "sources", "cefrj", "cefrj-vocabulary-profile-1.5.csv")); err == nil {
		t.Fatal("файл с неверной суммой остался в кэше")
	}
}
