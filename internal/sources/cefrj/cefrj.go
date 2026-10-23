// Package cefrj — источник «CEFR-J Wordlist»: слова с уровнями CEFR A1–B2.
// Данные: Tono Laboratory, Tokyo University of Foreign Studies; зеркало Open Language Profiles.
package cefrj

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "cefrj"
	version = "1.5"
	// Ссылка закреплена на коммит зеркала, а не на master: файл не изменится у нас под ногами.
	downloadURL = "https://raw.githubusercontent.com/openlanguageprofiles/olp-en-cefrj/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/cefrj-vocabulary-profile-1.5.csv"
	fileName    = "cefrj-vocabulary-profile-1.5.csv"
	fileSHA256  = "b0dd3c635f1c9a4fdf1490c7e5b7c48e8bbe55b652ad0c9860a95f98e10ae498"
	maxFileSize = 10 << 20 // файл ~233 КБ; лимит с большим запасом

	attribution = "The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies."
	license     = "Бесплатно для исследований и коммерции с указанием источника (Tono Laboratory, TUFS); при коммерческом использовании условия согласуются отдельно"

	maxHeadword = 64 // как CHECK в lexemes.lemma
	maxPOS      = 32 // как CHECK в lexemes.pos
)

var (
	header = []string{"headword", "pos", "CEFR", "CoreInventory 1", "CoreInventory 2", "Threshold"}
	levels = []string{"A1", "A2", "B1", "B2", "C1", "C2"}
	// Колонки CEFR-J, которых нет в единой схеме, уходят в extra под этими ключами (только непустые).
	extraKeys = map[int]string{3: "coreInventory1", 4: "coreInventory2", 5: "threshold"}
)

// Row — одна строка списка.
type Row struct {
	Headword string // как в файле, включая варианты через слэш
	POS      string
	Level    string
	Extra    map[string]string
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

func (s *Source) Parse(ctx context.Context, r io.Reader, emit func(row any) error) error {
	br := bufio.NewReader(r)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte("\xef\xbb\xbf")) {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1 // заголовок проверяем сами, потом включаем строгую проверку числа полей
	cr.ReuseRecord = true

	first, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return errors.New("в файле нет строки заголовка")
	}
	if err != nil {
		return fmt.Errorf("чтение заголовка: %w", err)
	}
	if !slices.Equal(first, header) {
		return fmt.Errorf("неожиданный заголовок %q, ожидали %q", first, header)
	}
	cr.FieldsPerRecord = len(header)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				return fmt.Errorf("строка %d: %w", pe.Line, err)
			}
			return err
		}
		line, _ := cr.FieldPos(0)
		row, err := toRow(rec, line)
		if err != nil {
			return err
		}
		if err := emit(row); err != nil {
			return err
		}
	}
}

func toRow(rec []string, line int) (Row, error) {
	headword, pos, level := rec[0], rec[1], rec[2]
	switch {
	case headword == "":
		return Row{}, fmt.Errorf("строка %d: пустой headword", line)
	case pos == "":
		return Row{}, fmt.Errorf("строка %d: пустая часть речи у %q", line, headword)
	case utf8.RuneCountInString(headword) > maxHeadword:
		return Row{}, fmt.Errorf("строка %d: headword длиннее %d символов", line, maxHeadword)
	case utf8.RuneCountInString(pos) > maxPOS:
		return Row{}, fmt.Errorf("строка %d: часть речи длиннее %d символов", line, maxPOS)
	case !slices.Contains(levels, level):
		return Row{}, fmt.Errorf("строка %d: неизвестный уровень %q у %q", line, level, headword)
	}
	extra := map[string]string{}
	for col, key := range extraKeys {
		if v := strings.TrimSpace(rec[col]); v != "" {
			extra[key] = v
		}
	}
	return Row{Headword: headword, POS: pos, Level: level, Extra: extra}, nil
}

func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	r, ok := row.(Row)
	if !ok {
		return fmt.Errorf("cefrj: ожидали Row, получили %T", row)
	}
	id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: r.Headword, Pos: r.POS, SourceID: sourceID})
	if err != nil {
		return fmt.Errorf("слово %q: %w", r.Headword, err)
	}
	extra, err := json.Marshal(r.Extra)
	if err != nil {
		return err
	}
	if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: sourceID, Level: r.Level, Extra: extra}); err != nil {
		return fmt.Errorf("уровень слова %q: %w", r.Headword, err)
	}
	return nil
}

var _ importer.Source = (*Source)(nil)
