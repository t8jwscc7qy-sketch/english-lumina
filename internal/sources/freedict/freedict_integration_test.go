package freedict

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

func requireTarXZ(t *testing.T) {
	t.Helper()
	if err := exec.Command("tar", "-tJf", "testdata/sample.tar.xz").Run(); err != nil {
		t.Skipf("tar без поддержки xz: %v", err)
	}
}

func sampleArchive(t *testing.T) ([]byte, string) {
	t.Helper()
	data, err := os.ReadFile("testdata/sample.tar.xz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:])
}

func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	return srv
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestFetchExtractsDictionaryFromArchive(t *testing.T) {
	requireTarXZ(t)
	data, sum := sampleArchive(t)
	srv := serve(t, data)
	cache := t.TempDir()
	path, err := newSource(srv.URL+"/a.tar.xz", sum, 1<<20, 1<<20, srv.Client()).Fetch(context.Background(), cache)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(cache, "sources", name) {
		t.Fatalf("файл вне кэша: %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(got), "<orth>apple</orth>") {
		t.Fatalf("извлечённый файл неверен: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".tei-*")); len(left) != 0 {
		t.Fatalf("остался временный файл: %v", left)
	}
}

func TestFetchRejectsWrongChecksum(t *testing.T) {
	requireTarXZ(t)
	data, _ := sampleArchive(t)
	srv := serve(t, data)
	_, err := newSource(srv.URL+"/a.tar.xz", strings.Repeat("0", 64), 1<<20, 1<<20, srv.Client()).Fetch(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("чужая сумма должна давать ошибку")
	}
}

func TestFetchFailsWhenArchiveLacksDictionary(t *testing.T) {
	requireTarXZ(t)
	// архив с другим файлом внутри: контрольная сумма верна, но словаря в нём нет
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "o.tar.xz")
	if err := exec.Command("tar", "-cJf", out, "-C", dir, "other.txt").Run(); err != nil {
		t.Skipf("не удалось собрать архив: %v", err)
	}
	data, _ := os.ReadFile(out)
	sum := sha256.Sum256(data)
	srv := serve(t, data)
	_, err := newSource(srv.URL+"/a.tar.xz", hex.EncodeToString(sum[:]), 1<<20, 1<<20, srv.Client()).Fetch(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("архив без словаря должен давать ошибку")
	}
}

func TestFetchEnforcesDictionarySizeLimit(t *testing.T) {
	requireTarXZ(t)
	data, sum := sampleArchive(t)
	srv := serve(t, data)
	_, err := newSource(srv.URL+"/a.tar.xz", sum, 1<<20, 100, srv.Client()).Fetch(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "больше") {
		t.Fatalf("ждали ошибку про размер, получили %v", err)
	}
}

func TestRunImportsSampleIdempotently(t *testing.T) {
	requireTarXZ(t)
	data, sum := sampleArchive(t) // до testdb.Pool: тот меняет рабочий каталог
	pool := testdb.Pool(t)
	ctx := context.Background()
	srv := serve(t, data)
	src := newSource(srv.URL+"/a.tar.xz", sum, 1<<20, 1<<20, srv.Client())
	cache := t.TempDir()

	res, err := importer.Run(ctx, pool, src, cache)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 12 || count(t, pool, `SELECT count(*) FROM word_translations`) != 12 {
		t.Fatalf("Rows=%d, в таблице %d; ждали 12", res.Rows, count(t, pool, `SELECT count(*) FROM word_translations`))
	}
	ds, err := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: name, Version: version})
	if err != nil || ds.RowCount == nil || *ds.RowCount != 12 || !strings.Contains(ds.Attribution, "Karl Bartel") || !strings.Contains(ds.License, "CC BY-SA 3.0") {
		t.Fatalf("data_sources: %+v (%v)", ds, err)
	}
	if n := count(t, pool, `SELECT count(*) FROM word_translations WHERE lemma = 'bank' AND pos = 'n' AND sense_no = 2 AND ru = 'берег'`); n != 1 {
		t.Fatalf("омонимы должны продолжать нумерацию: %d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM word_translations WHERE definition IS NULL`); n != 0 {
		t.Fatalf("у всех примеров есть пояснение, без него %d", n)
	}

	if _, err := importer.Run(ctx, pool, src, cache); err != nil {
		t.Fatalf("повторный импорт: %v", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM word_translations`); n != 12 {
		t.Fatalf("повторный импорт дал %d строк, ждали те же 12", n)
	}
}
