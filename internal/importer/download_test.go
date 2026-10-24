package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const body = "headword,pos,CEFR\na,determiner,A1\n"

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// files перечисляет всё, что лежит в каталоге: после неудачи там не должно быть ни файла, ни временных остатков.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func tlsServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func serve(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(content)) }
}

func TestDownloadSuccess(t *testing.T) {
	srv, _ := tlsServer(t, serve(body))
	dest := filepath.Join(t.TempDir(), "sub", "f.csv")

	if err := Download(context.Background(), srv.Client(), srv.URL+"/f.csv", dest, sum(body), 1<<20); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != body {
		t.Fatalf("файл: %q, ошибка %v", got, err)
	}
}

func TestDownloadSkipsNetworkWhenFileIsGood(t *testing.T) {
	srv, hits := tlsServer(t, serve(body))
	dest := filepath.Join(t.TempDir(), "f.csv")
	for i := 0; i < 2; i++ {
		if err := Download(context.Background(), srv.Client(), srv.URL, dest, sum(body), 1<<20); err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("запросов к серверу %d, ожидали 1", hits.Load())
	}
}

func TestDownloadFailuresLeaveNoFiles(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
		max     int64
	}{
		{"неверная сумма", serve(body), sum("другое"), 1 << 20},
		{"404", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }, sum(body), 1 << 20},
		{"500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, sum(body), 1 << 20},
		{"тело больше лимита", serve(body), sum(body), int64(len(body) - 1)},
		{"тело без длины больше лимита", func(w http.ResponseWriter, _ *http.Request) {
			w.(http.Flusher).Flush() // без Content-Length: длину заранее не узнать
			_, _ = w.Write([]byte(body))
		}, sum(body), int64(len(body) - 1)},
		{"обрыв посреди тела", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte("abc"))
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}, sum(body), 1 << 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := tlsServer(t, tc.handler)
			dir := t.TempDir()
			err := Download(context.Background(), srv.Client(), srv.URL, filepath.Join(dir, "f.csv"), tc.want, tc.max)
			if err == nil {
				t.Fatal("ожидали ошибку")
			}
			if left := files(t, dir); len(left) != 0 {
				t.Fatalf("после ошибки в каталоге осталось %v", left)
			}
		})
	}
}

func TestDownloadRejectsPlainHTTP(t *testing.T) {
	var hits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(plain.Close)

	err := Download(context.Background(), plain.Client(), plain.URL, filepath.Join(t.TempDir(), "f.csv"), sum(body), 1<<20)
	if err == nil {
		t.Fatal("http:// должен отклоняться")
	}
	if hits.Load() != 0 {
		t.Fatal("запрос на http:// всё же ушёл")
	}
}

func TestDownloadRejectsRedirectToHTTP(t *testing.T) {
	var plainHits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plainHits.Add(1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(plain.Close)
	srv, _ := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	})

	dir := t.TempDir()
	if err := Download(context.Background(), srv.Client(), srv.URL, filepath.Join(dir, "f.csv"), sum(body), 1<<20); err == nil {
		t.Fatal("редирект на http:// должен отклоняться")
	}
	if plainHits.Load() != 0 || len(files(t, dir)) != 0 {
		t.Fatalf("по редиректу ушёл запрос (%d) или остались файлы %v", plainHits.Load(), files(t, dir))
	}
}

func TestDownloadReplacesCorruptFile(t *testing.T) {
	srv, _ := tlsServer(t, serve(body))
	dest := filepath.Join(t.TempDir(), "f.csv")
	if err := os.WriteFile(dest, []byte("повреждено"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), srv.Client(), srv.URL, dest, sum(body), 1<<20); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != body {
		t.Fatalf("файл не заменён: %q", got)
	}
}

func TestDownloadFailureKeepsExistingFile(t *testing.T) {
	srv, _ := tlsServer(t, serve("совсем другое содержимое"))
	dest := filepath.Join(t.TempDir(), "f.csv")
	if err := os.WriteFile(dest, []byte("старый файл"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), srv.Client(), srv.URL, dest, sum(body), 1<<20); err == nil {
		t.Fatal("ожидали ошибку суммы")
	}
	if got, _ := os.ReadFile(dest); string(got) != "старый файл" {
		t.Fatalf("существующий файл испорчен: %q", got)
	}
	if left := files(t, filepath.Dir(dest)); len(left) != 1 {
		t.Fatalf("в каталоге лишнее: %v", left)
	}
}

func TestDownloadCanceledContext(t *testing.T) {
	srv, _ := tlsServer(t, serve(body))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if err := Download(ctx, srv.Client(), srv.URL, filepath.Join(dir, "f.csv"), sum(body), 1<<20); err == nil {
		t.Fatal("ожидали ошибку отменённого контекста")
	}
	if len(files(t, dir)) != 0 {
		t.Fatal("остались файлы")
	}
}

func TestNewHTTPClientHasTimeout(t *testing.T) {
	if NewHTTPClient().Timeout <= 0 {
		t.Fatal("у клиента нет таймаута")
	}
}

// Некоторые сайты (например manythings.org) отвечают 406 на стандартный User-Agent Go: представляемся честно.
func TestDownloadSendsDescriptiveUserAgent(t *testing.T) {
	var got atomic.Value
	srv, _ := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.UserAgent())
		_, _ = w.Write([]byte(body))
	})
	dest := filepath.Join(t.TempDir(), "f.csv")
	if err := Download(context.Background(), srv.Client(), srv.URL, dest, sum(body), 1<<20); err != nil {
		t.Fatal(err)
	}
	ua, _ := got.Load().(string)
	if !strings.HasPrefix(ua, "english-pass-importer/") || strings.Contains(ua, "Go-http-client") {
		t.Fatalf("User-Agent %q", ua)
	}
}
