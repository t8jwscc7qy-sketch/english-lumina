// Package tatoeba — источник «Tatoeba»: пары предложений «английское — русский перевод».
// Данные: проект Tatoeba (tatoeba.org), лицензия CC BY 2.0 FR; готовый файл пар с ManyThings (manythings.org/anki).
package tatoeba

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "tatoeba"
	version = "2026-02-13" // дата снимка ManyThings
	// Файл статический: адрес не перезаписывается, как у официальных еженедельных выгрузок Tatoeba.
	downloadURL = "https://www.manythings.org/anki/rus-eng.zip"
	zipSHA256   = "1534e267976f43ae97e966d2ac9dc1e9128fdd0efdf08eceee21cd88ff20682c"
	maxZipSize  = 40 << 20  // файл ~17 МБ
	maxExtract  = 200 << 20 // rus.txt ~89 МБ

	license     = "CC BY 2.0 FR: нужно указывать авторов (у каждого предложения хранится автор и номер на tatoeba.org) и лицензию"
	attribution = "Предложения: Tatoeba (tatoeba.org), лицензия CC BY 2.0 FR; пары англ.–рус. собраны на manythings.org/anki (снимок 13.02.2026); автор и номер каждого предложения сохранены."

	maxEnglishWords = 30   // более длинные английские предложения для учёбы не годятся
	maxTextLen      = 1000 // как CHECK в sentences.text
	maxAuthorLen    = 64   // как CHECK в sentences.author
	maxLineBytes    = 1 << 20
)

// Строка атрибуции ManyThings: номера и авторы обеих сторон пары.
var attributionRe = regexp.MustCompile(`^CC-BY 2\.0 \(France\) Attribution: tatoeba\.org #(\d+) \((.+?)\) & #(\d+) \((.+?)\)$`)

// Row — одна пара: английское предложение и его русский перевод.
type Row struct {
	EnID     int64 // номер на tatoeba.org
	EnText   string
	EnAuthor string
	RuID     int64
	RuText   string
	RuAuthor string
}

// Source реализует importer.Source.
type Source struct {
	url        string
	sha256     string
	maxZip     int64
	maxExtract int64
	client     *http.Client
}

// New возвращает источник с боевой ссылкой.
func New() *Source {
	return newSource(downloadURL, zipSHA256, maxZipSize, maxExtract, importer.NewHTTPClient())
}

func newSource(url, sha256 string, maxZip, maxExtract int64, client *http.Client) *Source {
	return &Source{url: url, sha256: sha256, maxZip: maxZip, maxExtract: maxExtract, client: client}
}

func (s *Source) Describe() importer.Info {
	return importer.Info{Name: name, Version: version, License: license, URL: downloadURL, Attribution: attribution}
}

// Fetch будет скачивать zip и доставать rus.txt (Task 3).
func (s *Source) Fetch(ctx context.Context, cacheDir string) (string, error) {
	return "", errors.New("tatoeba: Fetch не реализован")
}

// Parse читает rus.txt построчно: «английское<TAB>русское<TAB>атрибуция». Пары, где английское длиннее
// 30 слов, пропускаются. Любая испорченная строка — ошибка с номером строки.
func (s *Source) Parse(ctx context.Context, r io.Reader, emit func(row any) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	lineNo, skipped := 0, 0
	for sc.Scan() {
		lineNo++
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := parseLine(sc.Text())
		if err != nil {
			return fmt.Errorf("строка %d: %w", lineNo, err)
		}
		if len(strings.Fields(row.EnText)) > maxEnglishWords {
			skipped++
			continue
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("строка %d: длиннее %d байт", lineNo+1, maxLineBytes)
		}
		return fmt.Errorf("чтение файла: %w", err)
	}
	if skipped > 0 {
		slog.Info("tatoeba: пропущены пары с длинным английским предложением", "pairs", skipped, "max_words", maxEnglishWords)
	}
	return nil
}

func parseLine(line string) (Row, error) {
	if !utf8.ValidString(line) {
		return Row{}, errors.New("недопустимый UTF-8")
	}
	if strings.ContainsRune(line, 0) {
		return Row{}, errors.New("нулевой байт")
	}
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		return Row{}, fmt.Errorf("ожидали 3 поля через табуляцию, получили %d", len(fields))
	}
	m := attributionRe.FindStringSubmatch(fields[2])
	if m == nil {
		return Row{}, fmt.Errorf("атрибуция не по шаблону ManyThings: %q", fields[2])
	}
	enID, err1 := strconv.ParseInt(m[1], 10, 64)
	ruID, err2 := strconv.ParseInt(m[3], 10, 64)
	if err1 != nil || err2 != nil || enID <= 0 || ruID <= 0 {
		return Row{}, fmt.Errorf("номер предложения должен быть положительным числом: #%s и #%s", m[1], m[3])
	}
	row := Row{
		EnID: enID, EnText: strings.TrimSpace(fields[0]), EnAuthor: m[2],
		RuID: ruID, RuText: strings.TrimSpace(fields[1]), RuAuthor: m[4],
	}
	if row.EnText == "" || row.RuText == "" {
		return Row{}, errors.New("пустое предложение")
	}
	for _, text := range []string{row.EnText, row.RuText} {
		if utf8.RuneCountInString(text) > maxTextLen {
			return Row{}, fmt.Errorf("предложение длиннее %d символов", maxTextLen)
		}
	}
	for _, author := range []string{row.EnAuthor, row.RuAuthor} {
		if utf8.RuneCountInString(author) > maxAuthorLen {
			return Row{}, fmt.Errorf("автор длиннее %d символов", maxAuthorLen)
		}
	}
	return row, nil
}

// Load будет записывать пару в базу (Task 4).
func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	return errors.New("tatoeba: Load не реализован")
}

var _ importer.Source = (*Source)(nil)
