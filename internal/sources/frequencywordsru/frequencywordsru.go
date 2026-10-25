// Package frequencywordsru — источник «FrequencyWords ru»: 50 000 самых частых русских слов по субтитрам
// OpenSubtitles 2018 (список Hermit Dave, лицензия данных CC BY-SA 4.0). Хранится как есть, по написанию;
// места русских вариантов перевода читает «Сопоставление».
package frequencywordsru

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "frequencywords-ru"
	version = "2018" // год корпуса OpenSubtitles, из которого собран список
	// Ссылка закреплена на коммит репозитория, а не на master: файл не изменится у нас под ногами.
	downloadURL = "https://raw.githubusercontent.com/hermitdave/FrequencyWords/525f9b560de45753a5ea01069454e72e9aa541c6/content/2018/ru/ru_50k.txt"
	fileName    = "ru_50k-2018.txt"
	fileSHA256  = "6095f507cc167488ec66ada5a85ac50433503a08ad24a07c6eabdf54352c4e7f"
	maxFileSize = 10 << 20 // файл ~700 КБ; лимит с большим запасом

	license     = "CC BY-SA 4.0: нужно указывать автора списка и корпус; если распространять данные или производные от них, они тоже должны идти под CC BY-SA 4.0"
	attribution = "FrequencyWords (ru_50k, 2018), Hermit Dave, github.com/hermitdave/FrequencyWords, лицензия CC BY-SA 4.0. Счёт употреблений по корпусу OpenSubtitles2018 (opus.nlpl.eu/OpenSubtitles2018.php): P. Lison and J. Tiedemann, 2016, OpenSubtitles2016: Extracting Large Parallel Corpora from Movie and TV Subtitles, LREC 2016."

	maxWordLen = 64 // как CHECK в word_frequencies.word
)

// Row — одна строка списка: слово, число употреблений и место (1 — самое частое).
type Row struct {
	Word  string
	Count int64
	Rank  int
}

// Source реализует importer.Source.
type Source struct {
	url      string
	sha256   string
	maxBytes int64
	client   *http.Client
}

// New возвращает источник с боевой ссылкой.
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

// Parse читает строки «слово счёт» и присваивает ранг по порядку строк (строки не из кириллицы пропускаются, но ранг
// у них занят: у оставшихся слов он остаётся номером строки файла). Список должен не возрастать по счёту,
// слова не повторяются; любая испорченная строка — ошибка с номером.
func (s *Source) Parse(ctx context.Context, r io.Reader, emit func(row any) error) error {
	br := bufio.NewReader(r)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte("\xef\xbb\xbf")) {
		_, _ = br.Discard(3)
	}
	sc := bufio.NewScanner(br)

	seen := map[string]bool{}
	var prev int64 = -1
	lineNo, rank := 0, 0
	for sc.Scan() {
		lineNo++
		if err := ctx.Err(); err != nil {
			return err
		}
		word, count, err := parseLine(sc.Text())
		if err != nil {
			return fmt.Errorf("строка %d: %w", lineNo, err)
		}
		if prev >= 0 && count > prev {
			return fmt.Errorf("строка %d: счёт %d больше предыдущего %d, список должен убывать", lineNo, count, prev)
		}
		if seen[word] {
			return fmt.Errorf("строка %d: повтор слова %q", lineNo, word)
		}
		seen[word] = true
		prev = count
		rank++
		if !isCyrillicWord(word) {
			continue // шум субтитров (латиница, смесь букв, цифры); место остальных слов остаётся номером строки
		}
		if err := emit(Row{Word: word, Count: count, Rank: rank}); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}
	return nil
}

// isCyrillicWord: слово только из кириллических букв и дефиса (смесь вроде «чтo» с латинской «o» не годится).
func isCyrillicWord(w string) bool {
	return strings.IndexFunc(w, func(r rune) bool { return !unicode.Is(unicode.Cyrillic, r) && r != '-' }) < 0
}

func parseLine(line string) (string, int64, error) {
	if !utf8.ValidString(line) {
		return "", 0, errors.New("недопустимый UTF-8")
	}
	if strings.ContainsRune(line, 0) {
		return "", 0, errors.New("нулевой байт")
	}
	fields := strings.Split(line, " ")
	if len(fields) != 2 {
		return "", 0, fmt.Errorf("ожидали 2 поля через пробел, получили %d", len(fields))
	}
	word, countText := fields[0], fields[1]
	if word == "" || strings.IndexFunc(word, unicode.IsSpace) >= 0 {
		return "", 0, fmt.Errorf("пустое слово или пробелы внутри: %q", word)
	}
	if utf8.RuneCountInString(word) > maxWordLen {
		return "", 0, fmt.Errorf("слово длиннее %d символов", maxWordLen)
	}
	count, err := strconv.ParseInt(countText, 10, 64)
	if err != nil || count <= 0 || strings.TrimLeft(countText, "0123456789") != "" {
		return "", 0, fmt.Errorf("счёт должен быть положительным целым числом, получили %q", countText)
	}
	return word, count, nil
}

func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	r, ok := row.(Row)
	if !ok {
		return fmt.Errorf("frequencywordsru: ожидали Row, получили %T", row)
	}
	err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{
		SourceID: sourceID, Word: r.Word, Count: r.Count, Rank: int32(r.Rank), //nolint:gosec // ранг не больше числа строк списка
	})
	if err != nil {
		return fmt.Errorf("слово %q: %w", r.Word, err)
	}
	return nil
}

var _ importer.Source = (*Source)(nil)
