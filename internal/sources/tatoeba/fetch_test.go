package tatoeba

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

type entry struct {
	name string
	data string
}

// makeZip собирает zip в памяти: в репозитории бинарных файлов нет.
func makeZip(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// serve отдаёт zip с TLS-сервера и настраивает источник на него.
func serve(t *testing.T, zipBytes []byte, wantSum string, maxExtract int64) (*Source, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(zipBytes)
	}))
	t.Cleanup(srv.Close)
	return newSource(srv.URL+"/rus-eng.zip", wantSum, 1<<20, maxExtract, srv.Client()), &hits
}

func cachePaths(cache string) (dir, zipPath, txt string) {
	dir = filepath.Join(cache, "sources", "tatoeba")
	return dir, filepath.Join(dir, "rus-eng.zip"), filepath.Join(dir, "rus.txt")
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}

func TestFetchExtractsRusTxt(t *testing.T) {
	content := readSample(t)
	z := makeZip(t, entry{"rus.txt", content}, entry{"_about.txt", "about"})
	src, _ := serve(t, z, sum(z), 1<<20)
	cache := t.TempDir()

	path, err := src.Fetch(context.Background(), cache)
	if err != nil {
		t.Fatal(err)
	}
	dir, zipPath, txt := cachePaths(cache)
	if path != txt {
		t.Fatalf("путь %s, ожидали %s", path, txt)
	}
	if got, _ := os.ReadFile(txt); string(got) != content {
		t.Fatal("rus.txt в кэше не совпадает с исходным")
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("zip не лежит в кэше: %v", err)
	}
	// _about.txt не извлекается, временных файлов нет.
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip", "rus.txt"}) {
		t.Fatalf("в кэше: %v", got)
	}
}

func TestFetchSecondRunSkipsDownloadAndRestoresFile(t *testing.T) {
	content := readSample(t)
	z := makeZip(t, entry{"rus.txt", content})
	src, hits := serve(t, z, sum(z), 1<<20)
	cache := t.TempDir()
	_, _, txt := cachePaths(cache)

	if _, err := src.Fetch(context.Background(), cache); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(txt, []byte("испорчено"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Fetch(context.Background(), cache); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("zip скачан %d раз, ожидали 1", hits.Load())
	}
	if got, _ := os.ReadFile(txt); string(got) != content {
		t.Fatalf("rus.txt не пересоздан: %q", got)
	}
}

func TestFetchRejectsArchivesWithoutExactRusTxt(t *testing.T) {
	tests := map[string][]entry{
		"только _about.txt":  {{"_about.txt", "x"}},
		"../rus.txt":         {{"../rus.txt", "x"}},
		"/rus.txt":           {{"/rus.txt", "x"}},
		"dir/rus.txt":        {{"dir/rus.txt", "x"}},
		"другой регистр":     {{"RUS.TXT", "x"}},
		"каталог rus.txt/":   {{"rus.txt/", ""}},
		"пустой архив":       {},
		"имя с продолжением": {{"rus.txt.bak", "x"}},
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			z := makeZip(t, entries...)
			src, _ := serve(t, z, sum(z), 1<<20)
			root := t.TempDir()
			cache := filepath.Join(root, "cache")

			_, err := src.Fetch(context.Background(), cache)
			if err == nil || !strings.Contains(err.Error(), "rus.txt") {
				t.Fatalf("ожидали ошибку про rus.txt, получили %v", err)
			}
			dir, _, _ := cachePaths(cache)
			if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip"}) {
				t.Fatalf("в кэше после ошибки: %v", got)
			}
			// Ничего не записано за пределами каталога кэша.
			if got := names(t, root); !slices.Equal(got, []string{"cache"}) {
				t.Fatalf("за пределами кэша появилось: %v", got)
			}
		})
	}
}

func TestFetchRejectsNonRegularFile(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "rus.txt"}
	h.SetMode(os.ModeDir | 0o755)
	if _, err := zw.CreateHeader(h); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	src, _ := serve(t, buf.Bytes(), sum(buf.Bytes()), 1<<20)
	cache := t.TempDir()
	if _, err := src.Fetch(context.Background(), cache); err == nil {
		t.Fatal("каталог под именем rus.txt должен отклоняться")
	}
	dir, _, _ := cachePaths(cache)
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip"}) {
		t.Fatalf("в кэше: %v", got)
	}
}

func TestFetchRejectsOversizedMember(t *testing.T) {
	z := makeZip(t, entry{"rus.txt", strings.Repeat("a", 2000)})
	src, _ := serve(t, z, sum(z), 1000)
	cache := t.TempDir()
	if _, err := src.Fetch(context.Background(), cache); err == nil || !strings.Contains(err.Error(), "лимит") {
		t.Fatalf("ожидали ошибку лимита, получили %v", err)
	}
	dir, _, _ := cachePaths(cache)
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip"}) {
		t.Fatalf("после ошибки в кэше: %v", got)
	}
}

// Заявленный в заголовке размер нельзя считать правдой: данные больше заявленного не извлекаются.
func TestFetchDoesNotTrustDeclaredSize(t *testing.T) {
	payload := []byte(strings.Repeat("a", 5000))
	var comp bytes.Buffer
	fw, err := flate.NewWriter(&comp, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(payload)
	_ = fw.Close()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name: "rus.txt", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(payload),
		CompressedSize64: uint64(comp.Len()), UncompressedSize64: 10, // врёт: на деле 5000
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(comp.Bytes())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	src, _ := serve(t, buf.Bytes(), sum(buf.Bytes()), 1000)
	cache := t.TempDir()
	// Подделку ловит уже сам archive/zip (ErrFormat, если данных больше заявленного); наш лимит чтения
	// остаётся второй линией защиты. Важно, что файл не извлечён и кэш чист.
	if _, err := src.Fetch(context.Background(), cache); err == nil || !strings.Contains(err.Error(), "распаковка") {
		t.Fatalf("ожидали ошибку распаковки (фактический размер больше заявленного), получили %v", err)
	}
	dir, _, _ := cachePaths(cache)
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip"}) {
		t.Fatalf("после ошибки в кэше: %v", got)
	}
}

func TestFetchRejectsWrongChecksumBeforeUnpacking(t *testing.T) {
	z := makeZip(t, entry{"rus.txt", "x"})
	src, _ := serve(t, z, sum([]byte("другое")), 1<<20)
	cache := t.TempDir()
	if _, err := src.Fetch(context.Background(), cache); err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("ожидали ошибку суммы, получили %v", err)
	}
	if got := names(t, filepath.Join(cache, "sources", "tatoeba")); len(got) != 0 {
		t.Fatalf("в кэше после ошибки суммы: %v", got)
	}
}

func TestFetchRejectsNotAZip(t *testing.T) {
	junk := []byte("это не zip")
	src, _ := serve(t, junk, sum(junk), 1<<20)
	cache := t.TempDir()
	if _, err := src.Fetch(context.Background(), cache); err == nil {
		t.Fatal("ожидали ошибку чтения архива")
	}
	dir, _, _ := cachePaths(cache)
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip"}) {
		t.Fatalf("в кэше: %v", got)
	}
}

func TestFetchFailureKeepsExistingRusTxt(t *testing.T) {
	z := makeZip(t, entry{"_about.txt", "x"}) // нет rus.txt
	src, _ := serve(t, z, sum(z), 1<<20)
	cache := t.TempDir()
	dir, _, txt := cachePaths(cache)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(txt, []byte("старый файл"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Fetch(context.Background(), cache); err == nil {
		t.Fatal("ожидали ошибку")
	}
	if got, _ := os.ReadFile(txt); string(got) != "старый файл" {
		t.Fatalf("существующий rus.txt испорчен: %q", got)
	}
	if got := names(t, dir); !slices.Equal(got, []string{"rus-eng.zip", "rus.txt"}) {
		t.Fatalf("в кэше: %v", got)
	}
}
