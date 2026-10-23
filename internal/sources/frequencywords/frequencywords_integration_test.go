package frequencywords

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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(content)) }))
	t.Cleanup(srv.Close)
	return newSource(srv.URL+"/en_50k.txt", wantSum, 1<<20, srv.Client())
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
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
	if res.Rows != 24 || count(t, pool, `SELECT count(*) FROM word_frequencies`) != 24 {
		t.Fatalf("Rows=%d, в таблице %d; ожидали 24", res.Rows, count(t, pool, `SELECT count(*) FROM word_frequencies`))
	}
	if _, err := os.Stat(filepath.Join(cache, "sources", "frequencywords", "en_50k-2018.txt")); err != nil {
		t.Fatalf("файл не лежит в кэше: %v", err)
	}
	ds, err := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "frequencywords", Version: "2018"})
	if err != nil || ds.RowCount == nil || *ds.RowCount != 24 || !ds.ImportedAt.Valid || !strings.Contains(ds.Attribution, "Hermit Dave") {
		t.Fatalf("data_sources: %+v (%v)", ds, err)
	}
	var rank int
	var cnt int64
	if err := pool.QueryRow(ctx, `SELECT rank, count FROM word_frequencies WHERE word = 'run'`).Scan(&rank, &cnt); err != nil || rank != 17 || cnt != 244200 {
		t.Fatalf("run: ранг %d, счёт %d (%v)", rank, cnt, err)
	}
	// Слова из таблицы не создают слов в словаре и уровней: источник только про частоты.
	if count(t, pool, `SELECT count(*) FROM lexemes`) != 0 {
		t.Fatal("частотный список не должен добавлять слова")
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
	if count(t, pool, `SELECT count(*) FROM data_sources`) != 1 || count(t, pool, `SELECT count(*) FROM word_frequencies`) != 24 {
		t.Fatalf("sources=%d rows=%d", count(t, pool, `SELECT count(*) FROM data_sources`), count(t, pool, `SELECT count(*) FROM word_frequencies`))
	}
}

func TestRunBadRowRollsBackEverything(t *testing.T) {
	pool := testdb.Pool(t)
	broken := readSample(t) + "oops 1000\n" // счёт больше предыдущего: список перестал убывать
	_, err := importer.Run(context.Background(), pool, sourceFor(t, broken, sha(broken)), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "строка 25") {
		t.Fatalf("ожидали ошибку про строку 25, получили %v", err)
	}
	for _, table := range []string{"data_sources", "word_frequencies"} {
		if n := count(t, pool, "SELECT count(*) FROM "+table); n != 0 {
			t.Fatalf("после ошибки в %s осталось %d строк", table, n)
		}
	}
}

func TestRunEmptyFileIsAnError(t *testing.T) {
	pool := testdb.Pool(t)
	if _, err := importer.Run(context.Background(), pool, sourceFor(t, "", sha("")), t.TempDir()); err == nil || !strings.Contains(err.Error(), "нет строк") {
		t.Fatalf("ожидали «нет строк», получили %v", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM data_sources`); n != 0 {
		t.Fatalf("data_sources: %d", n)
	}
}

func TestRunChecksumMismatchTouchesNothing(t *testing.T) {
	pool := testdb.Pool(t)
	cache := t.TempDir()
	sample := readSample(t)
	if _, err := importer.Run(context.Background(), pool, sourceFor(t, sample, sha("другое")), cache); err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("ожидали ошибку суммы, получили %v", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM data_sources`); n != 0 {
		t.Fatalf("data_sources: %d", n)
	}
	if _, err := os.Stat(filepath.Join(cache, "sources", "frequencywords", "en_50k-2018.txt")); err == nil {
		t.Fatal("файл с неверной суммой остался в кэше")
	}
}

// ListWordFrequencies берёт частоты из последнего завершённого импорта источника по имени.
func TestListWordFrequenciesUsesLatestFinishedVersion(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	mk := func(version string, finished bool, rank int32) {
		ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "frequencywords", Version: version, License: "l", Url: "u", Attribution: "a"})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: ds.ID, Word: "run", Count: 100, Rank: rank}); err != nil {
			t.Fatal(err)
		}
		if finished {
			n := int32(1)
			if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: ds.ID, RowCount: &n}); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("2018", true, 17)
	mk("2020", false, 5) // недогруженная версия не учитывается

	rows, err := q.ListWordFrequencies(ctx, db.ListWordFrequenciesParams{SourceName: "frequencywords", Words: []string{"run", "nothing"}})
	if err != nil || len(rows) != 1 || rows[0].Word != "run" || rows[0].Rank != 17 {
		t.Fatalf("строки %+v (%v)", rows, err)
	}
	rows, err = q.ListWordFrequencies(ctx, db.ListWordFrequenciesParams{SourceName: "unknown", Words: []string{"run"}})
	if err != nil || len(rows) != 0 {
		t.Fatalf("неизвестный источник: %+v (%v)", rows, err)
	}
}
