// Package freedict — источник «FreeDict English–Русский» (WikDict): русские переводы английских слов.
// Данные собраны WikDict из Викисловаря через DBnary, лицензия CC BY-SA 3.0. Из словаря берутся только
// слова, пригодные для учёбы: одно слово (без пробелов), часть речи «сущ., глагол, прил., нареч.».
package freedict

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "freedict-eng-rus"
	version = "2025.11.23"
	// Адрес закреплён на версию словаря: файл версии не меняется.
	downloadURL  = "https://download.freedict.org/dictionaries/eng-rus/2025.11.23/freedict-eng-rus-2025.11.23.src.tar.xz"
	archiveName  = "freedict-eng-rus-2025.11.23.src.tar.xz"
	archiveSHA   = "f3dadcc29d7df6e77edb13b98a2ba2c3fac002852eddd8b557637f0b13d39c39"
	teiInArchive = "eng-rus/eng-rus.tei"
	teiName      = "eng-rus-2025.11.23.tei"
	maxArchive   = 30 << 20  // архив ~4 МБ
	maxTEI       = 200 << 20 // TEI ~36 МБ

	license     = "CC BY-SA 3.0: нужно указывать авторов и лицензию; если распространять данные или производные от них, они тоже должны идти под CC BY-SA"
	attribution = "FreeDict English–Русский (2025.11.23), составитель Karl Bartel, автоматически собран WikDict (wikdict.com) из Викисловаря (wiktionary.org) через DBnary (kaiko.getalp.org/about-dbnary); лицензия CC BY-SA 3.0."

	maxLemmaLen      = 64  // как CHECK в word_translations.lemma
	maxRuLen         = 64  // как CHECK в word_translations.ru
	maxDefinitionLen = 400 // как CHECK в word_translations.definition
	maxPerSense      = 8   // переводов одного значения не больше
)

// Части речи словаря, которые берём: существительное, глагол, прилагательное, наречие.
var posAllowed = map[string]bool{"n": true, "v": true, "adj": true, "adv": true}

var lemmaRe = regexp.MustCompile(`^[A-Za-z][A-Za-z'’-]*$`)

// Row — один перевод значения слова.
type Row struct {
	Lemma      string
	Pos        string
	SenseNo    int // номер значения среди значений этой пары «слово + часть речи», с 1
	Ru         string
	Definition string // английское пояснение значения; может быть пустым
}

// Source реализует importer.Source.
type Source struct {
	url        string
	sha256     string
	maxArchive int64
	maxTEI     int64
	client     *http.Client
}

// New возвращает источник с боевой ссылкой.
func New() *Source {
	return newSource(downloadURL, archiveSHA, maxArchive, maxTEI, importer.NewHTTPClient())
}

func newSource(url, sha256 string, maxArchive, maxTEI int64, client *http.Client) *Source {
	return &Source{url: url, sha256: sha256, maxArchive: maxArchive, maxTEI: maxTEI, client: client}
}

func (s *Source) Describe() importer.Info {
	return importer.Info{Name: name, Version: version, License: license, URL: downloadURL, Attribution: attribution}
}

// Fetch скачивает архив с проверкой суммы и достаёт из него один файл словаря системной командой tar (в Go нет xz
// в стандартной библиотеке, а новые зависимости не добавляем). Файл извлекается в стандартный вывод и пишется в кэш
// нами же, поэтому имена внутри архива на путь записи не влияют.
func (s *Source) Fetch(ctx context.Context, cacheDir string) (string, error) {
	dir := filepath.Join(cacheDir, "sources", name)
	archive := filepath.Join(dir, archiveName)
	if err := importer.Download(ctx, s.client, s.url, archive, s.sha256, s.maxArchive); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, teiName)
	if err := extractTEI(ctx, archive, dest, s.maxTEI); err != nil {
		return "", err
	}
	return dest, nil
}

func extractTEI(ctx context.Context, archive, dest string, limit int64) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".tei-*")
	if err != nil {
		return fmt.Errorf("временный файл словаря: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	cmd := exec.CommandContext(ctx, "tar", "-xJOf", archive, teiInArchive) //nolint:gosec // аргументы постоянные, путь архива наш
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("tar: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить tar (нужен tar с поддержкой xz): %w", err)
	}
	n, copyErr := io.Copy(tmp, io.LimitReader(out, limit+1))
	if n > limit {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("файл словаря больше %d байт", limit)
	}
	if copyErr != nil {
		_ = cmd.Wait()
		return fmt.Errorf("чтение из tar: %w", copyErr)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("tar не распаковал %s: %w: %s", teiInArchive, err, strings.TrimSpace(stderr.String()))
	}
	if n == 0 {
		return fmt.Errorf("в архиве нет файла %s", teiInArchive)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// Разбор TEI: нужны слово, часть речи, переводы и пояснение каждого значения.
type teiEntry struct {
	Orth   []string   `xml:"form>orth"`
	Pos    string     `xml:"gramGrp>pos"`
	Senses []teiSense `xml:"sense"`
}

type teiSense struct {
	Cits []teiCit `xml:"cit"`
	Def  string   `xml:"sense>def"`
}

type teiCit struct {
	Type   string   `xml:"type,attr"`
	Quotes []string `xml:"quote"`
}

// Parse читает файл потоком, по одной статье, и отдаёт переводы. Нумерация значений идёт по паре «слово + часть речи»
// сквозь все статьи с таким написанием (омонимы стоят в словаре отдельными статьями).
func (s *Source) Parse(ctx context.Context, r io.Reader, emit func(row any) error) error {
	dec := xml.NewDecoder(r)
	senseCount := map[string]int{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("разбор TEI: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "entry" {
			continue
		}
		var e teiEntry
		if err := dec.DecodeElement(&e, &start); err != nil {
			return fmt.Errorf("статья словаря: %w", err)
		}
		for _, row := range convertEntry(e, senseCount) {
			if err := emit(row); err != nil {
				return err
			}
		}
	}
}

func convertEntry(e teiEntry, senseCount map[string]int) []Row {
	if len(e.Orth) == 0 || !posAllowed[e.Pos] {
		return nil
	}
	lemma := strings.TrimSpace(e.Orth[0])
	if !validLemma(lemma) {
		return nil
	}
	key := lemma + "\x00" + e.Pos
	var rows []Row
	for _, sense := range e.Senses {
		seen := map[string]bool{}
		var ru []string
		for _, cit := range sense.Cits {
			if cit.Type != "trans" {
				continue
			}
			for _, q := range cit.Quotes {
				t := cleanTranslation(q)
				if t == "" || seen[t] || len(ru) >= maxPerSense {
					continue
				}
				seen[t] = true
				ru = append(ru, t)
			}
		}
		if len(ru) == 0 {
			continue
		}
		senseCount[key]++
		def := cleanDefinition(sense.Def)
		for _, t := range ru {
			rows = append(rows, Row{Lemma: lemma, Pos: e.Pos, SenseNo: senseCount[key], Ru: t, Definition: def})
		}
	}
	return rows
}

func validLemma(l string) bool {
	return utf8.RuneCountInString(l) <= maxLemmaLen && lemmaRe.MatchString(l)
}

// cleanTranslation убирает знак ударения и пробелы по краям; пустое, без кириллицы или слишком длинное — "".
func cleanTranslation(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '́' || r == '̀' {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || utf8.RuneCountInString(s) > maxRuLen || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return ""
	}
	if strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) < 0 {
		return ""
	}
	return s
}

func cleanDefinition(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return ""
	}
	if utf8.RuneCountInString(s) > maxDefinitionLen {
		s = string([]rune(s)[:maxDefinitionLen])
	}
	return s
}

func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	r, ok := row.(Row)
	if !ok {
		return fmt.Errorf("freedict: ожидали Row, получили %T", row)
	}
	var def *string
	if r.Definition != "" {
		def = &r.Definition
	}
	err := q.UpsertWordTranslation(ctx, db.UpsertWordTranslationParams{
		SourceID: sourceID, Lemma: r.Lemma, Pos: r.Pos, SenseNo: int16(r.SenseNo), Ru: r.Ru, Definition: def, //nolint:gosec // значений слова немного
	})
	if err != nil {
		return fmt.Errorf("слово %q (%s): %w", r.Lemma, r.Pos, err)
	}
	return nil
}

var _ importer.Source = (*Source)(nil)
