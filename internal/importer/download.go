package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const downloadTimeout = 2 * time.Minute

// userAgent представляет импортёр честно: часть сайтов отвечает 406 на стандартный Go-http-client.
const userAgent = "english-pass-importer/1.0 (personal study tool)"

// NewHTTPClient — клиент для скачивания баз: с общим таймаутом. Правила про HTTPS и редиректы
// действуют внутри Download, какой бы клиент ему ни передали.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: downloadTimeout}
}

// Download скачивает файл по HTTPS в dest и проверяет его SHA-256. Если dest уже лежит и сумма верна,
// сеть не трогается. Файл сначала пишется во временный рядом, переименовывается только после проверки:
// при любой ошибке dest остаётся как был, а временный файл удаляется. Скачанное считается недоверенным.
func Download(ctx context.Context, client *http.Client, rawURL, dest, wantSHA256 string, maxBytes int64) (err error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("скачивать можно только по https: %q", rawURL)
	}
	if fileHasSum(dest, wantSHA256, maxBytes) {
		return nil
	}

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("создание каталога кэша: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return fmt.Errorf("временный файл: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	c := *client // копия, чтобы не менять чужой клиент
	c.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("редирект на не-https адрес %q отклонён", req.URL)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("скачивание: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("скачивание: сервер ответил %s", resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("файл больше лимита %d байт", maxBytes)
	}

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return fmt.Errorf("скачивание: %w", err)
	}
	if n > maxBytes {
		return fmt.Errorf("файл больше лимита %d байт", maxBytes)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, wantSHA256) {
		return fmt.Errorf("контрольная сумма не совпала: получили %s, ожидали %s", got, wantSHA256)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// fileHasSum сообщает, что файл есть, не больше лимита и его сумма верна.
func fileHasSum(path, wantSHA256 string, maxBytes int64) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxBytes+1))
	if err != nil || n > maxBytes {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), wantSHA256)
}
