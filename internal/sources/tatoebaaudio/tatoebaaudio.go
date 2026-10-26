// Package tatoebaaudio — источник «Tatoeba audio»: список записей произношения предложений (кто и под какой лицензией
// записал). Сами файлы не скачиваются: плеер берёт их с tatoeba.org по номеру записи. Лицензия записи выбирается её
// автором, поэтому загружаются только записи с разрешённой лицензией (см. AllowedLicenses).
package tatoebaaudio

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "tatoeba-audio"
	version = "2026-10-26" // дата выгрузки (еженедельная; при обновлении пересчитывается сумма)
	// Выгрузка Tatoeba перезаписывается каждую неделю: сумма закреплена за снимком от даты version. При смене файла загрузка
	// падает на сумме, и снимок нужно закрепить заново (как у Kaikki).
	downloadURL = "https://downloads.tatoeba.org/exports/sentences_with_audio.tar.bz2"
	fileName    = "sentences_with_audio-2026-10-26.tar.bz2"
	fileSHA256  = "6af74c8b9c0d7cc1b5ebb0add143f4a4ea3371f12665f9c35520c543eff7d7e7"
	maxFileSize = 60 << 20 // файл ~6 МБ
	maxCSVBytes = 200 << 20

	license     = "У каждой записи своя лицензия, выбранная автором записи (CC BY, CC BY-SA, CC0 и для личного некоммерческого использования CC BY-NC и CC BY-NC-ND); записи без лицензии не берутся. Нужно указывать автора записи и лицензию. Какие лицензии показывать, решает настройка сервера AUDIO_NONCOMMERCIAL."
	attribution = "Озвучка предложений: Tatoeba (tatoeba.org); автор и лицензия каждой записи сохранены и показываются рядом с воспроизведением."

	maxUserLen = 64
	maxLicLen  = 32
)

// OpenLicenses — открытые лицензии записей: можно использовать и в коммерческом проекте с указанием автора.
var OpenLicenses = []string{"CC BY 4.0", "CC BY 3.0", "CC BY-SA 4.0", "CC BY-SA 3.0", "CC0 1.0"}

// NonCommercialLicenses — лицензии с запретом коммерческого использования (и переделки). В личном некоммерческом проекте
// записи проигрываются без изменений с указанием автора; при коммерческом использовании их нужно отключить настройкой
// сервера AUDIO_NONCOMMERCIAL=false (повторный импорт не нужен).
var NonCommercialLicenses = []string{"CC BY-NC-ND 3.0", "CC BY-NC-ND 4.0", "CC BY-NC 4.0", "CC BY-NC 3.0", "CC BY-NC-SA 4.0", "CC BY-NC-SA 3.0"}

// AllowedLicenses — лицензии, которые загружает импорт: открытые и некоммерческие (какие из них показывать, решает сервер).
func AllowedLicenses() []string {
	return append(append([]string{}, OpenLicenses...), NonCommercialLicenses...)
}

// ServedLicenses — лицензии, записи которых сервер отдаёт: открытые и, если nonCommercial, некоммерческие.
func ServedLicenses(nonCommercial bool) []string {
	if nonCommercial {
		return AllowedLicenses()
	}
	return append([]string{}, OpenLicenses...)
}

// Row — одна запись: предложение, запись, автор, лицензия и ссылка автора для атрибуции.
type Row struct {
	SentenceID     int64
	AudioID        int64
	Username       string
	License        string
	AttributionURL string
}

// Source реализует importer.Source.
type Source struct {
	url      string
	sha256   string
	maxBytes int64
	client   *http.Client
}

func New() *Source {
	return newSource(downloadURL, fileSHA256, maxFileSize, importer.NewHTTPClient())
}

func newSource(url, sha256 string, maxBytes int64, client *http.Client) *Source {
	return &Source{url: url, sha256: sha256, maxBytes: maxBytes, client: client}
}

func (s *Source) Describe() importer.Info {
	return importer.Info{Name: name, Version: version, License: license, URL: downloadURL, Attribution: attribution}
}

func (s *Source) Fetch(ctx context.Context, cacheDir string) (string, error) {
	path := filepath.Join(cacheDir, "sources", name, fileName)
	if err := importer.Download(ctx, s.client, s.url, path, s.sha256, s.maxBytes); err != nil {
		return "", err
	}
	return path, nil
}

func allowed(lic string) bool {
	for _, a := range AllowedLicenses() {
		if lic == a {
			return true
		}
	}
	return false
}

// parseLine разбирает строку «номер предложения, номер записи, автор, лицензия, ссылка» (через табуляцию). ok = false —
// запись не нужна (нет лицензии или лицензия не из списка); err — строка испорчена.
func parseLine(line string) (Row, bool, error) {
	if !utf8.ValidString(line) || strings.ContainsRune(line, 0) {
		return Row{}, false, errors.New("недопустимые символы")
	}
	f := strings.Split(line, "\t")
	if len(f) < 4 || len(f) > 5 {
		return Row{}, false, fmt.Errorf("ожидали 4–5 полей через табуляцию, получили %d", len(f))
	}
	sid, err1 := strconv.ParseInt(f[0], 10, 64)
	aid, err2 := strconv.ParseInt(f[1], 10, 64)
	if err1 != nil || err2 != nil || sid <= 0 || aid <= 0 {
		return Row{}, false, fmt.Errorf("номера должны быть положительными целыми: %q, %q", f[0], f[1])
	}
	user, lic := f[2], strings.TrimSpace(f[3])
	if !allowed(lic) {
		return Row{}, false, nil
	}
	if user == "" || utf8.RuneCountInString(user) > maxUserLen || utf8.RuneCountInString(lic) > maxLicLen {
		return Row{}, false, fmt.Errorf("автор пуст или слишком длинный: %q", user)
	}
	url := ""
	if len(f) == 5 && f[4] != `\N` {
		url = f[4]
	}
	return Row{SentenceID: sid, AudioID: aid, Username: user, License: lic, AttributionURL: url}, true, nil
}

// Parse читает tar.bz2 с одним файлом sentences_with_audio.csv и отдаёт записи с разрешённой лицензией.
func (s *Source) Parse(ctx context.Context, r io.Reader, emit func(row any) error) error {
	tr := tar.NewReader(bzip2.NewReader(r))
	hdr, err := tr.Next()
	if err != nil {
		return fmt.Errorf("архив: %w", err)
	}
	if !strings.HasSuffix(hdr.Name, ".csv") {
		return fmt.Errorf("в архиве ожидали .csv, а там %q", hdr.Name)
	}
	sc := bufio.NewScanner(io.LimitReader(tr, maxCSVBytes))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	seen := map[int64]bool{}
	for n := 1; sc.Scan(); n++ {
		if n%10000 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		line := sc.Text()
		if line == "" {
			continue
		}
		row, ok, err := parseLine(line)
		if err != nil {
			return fmt.Errorf("строка %d: %w", n, err)
		}
		if !ok {
			continue
		}
		if seen[row.AudioID] {
			return fmt.Errorf("строка %d: запись %d повторяется", n, row.AudioID)
		}
		seen[row.AudioID] = true
		if err := emit(row); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}
	return nil
}

func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	r, ok := row.(Row)
	if !ok {
		return fmt.Errorf("tatoeba-audio: ожидали Row, получили %T", row)
	}
	var url *string
	if r.AttributionURL != "" {
		url = &r.AttributionURL
	}
	if _, err := q.UpsertSentenceAudio(ctx, db.UpsertSentenceAudioParams{
		AudioID: r.AudioID, SentenceTatoebaID: r.SentenceID, Username: r.Username, License: r.License, AttributionUrl: url, SourceID: sourceID,
	}); err != nil {
		return fmt.Errorf("запись %d: %w", r.AudioID, err)
	}
	return nil
}

var _ importer.Source = (*Source)(nil)
