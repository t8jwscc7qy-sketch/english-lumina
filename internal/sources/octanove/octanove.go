// Package octanove — источник «Octanove Vocabulary Profile C1/C2»: слова с уровнями C1 и C2
// (в CEFR-J их нет). Автор данных Octanove Labs, лицензия CC BY-SA 4.0; файл лежит в зеркале Open Language Profiles.
package octanove

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

const (
	name    = "octanove"
	version = "1.0"
	// Ссылка закреплена на тот же коммит зеркала, что и у CEFR-J, а не на master.
	downloadURL = "https://raw.githubusercontent.com/openlanguageprofiles/olp-en-cefrj/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/octanove-vocabulary-profile-c1c2-1.0.csv"
	fileName    = "octanove-vocabulary-profile-c1c2-1.0.csv"
	fileSHA256  = "18c33a407f2f89f7b8de9671c6d45fe3ea0bce45e7d2d7dcaab48d73e0f7b380"
	maxFileSize = 10 << 20 // файл ~46 КБ; лимит с большим запасом

	attribution = "Octanove Vocabulary Profile C1/C2 (ver 1.0), created by Octanove Labs; distributed via Open Language Profiles (github.com/openlanguageprofiles/olp-en-cefrj). Licensed under CC BY-SA 4.0."
	license     = "CC BY-SA 4.0: можно использовать с указанием автора; если распространять данные или производные от них, они тоже должны идти под CC BY-SA 4.0"

	maxHeadword = 64 // как CHECK в lexemes.lemma
	maxPOS      = 32 // как CHECK в lexemes.pos
)

var (
	header = []string{"headword", "pos", "CEFR", "notes"}
	levels = []string{"C1", "C2"} // этот файл содержит только их

	// Известные опечатки в самих данных (проверены на версии 1.0): слово → исправленная часть речи.
	posFixes = map[string]string{"vern": "verb"}
)

// Row — одна пара (слово, часть речи) после склейки повторов.
type Row struct {
	Headword string // как в файле, без пробелов по краям; варианты через слэш сохранены
	POS      string
	Level    string              // самый низкий из уровней повторов
	Extra    map[string][]string // "notes": заметки о значениях слова, если они есть
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

// Parse читает файл и склеивает повторы пар (слово, часть речи): в файле у разных значений слова
// отдельные строки, а схема хранит одну строку на слово и источник. Берётся самый низкий уровень,
// заметки собираются. Файл небольшой (тысячи строк), поэтому склейка держит пары в памяти
// и отдаёт их по порядку первого появления.
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

	type key struct{ headword, pos string }
	var order []key
	rows := map[key]*Row{}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				return fmt.Errorf("строка %d: %w", pe.Line, err)
			}
			return err
		}
		line, _ := cr.FieldPos(0)
		headword, pos, level, note := strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1]), rec[2], strings.TrimSpace(rec[3])

		switch {
		case headword == "":
			return fmt.Errorf("строка %d: пустой headword", line)
		case !slices.Contains(levels, level):
			return fmt.Errorf("строка %d: неожиданный уровень %q у %q (в этом файле только %v)", line, level, headword, levels)
		case utf8.RuneCountInString(headword) > maxHeadword:
			return fmt.Errorf("строка %d: headword длиннее %d символов", line, maxHeadword)
		case utf8.RuneCountInString(pos) > maxPOS:
			return fmt.Errorf("строка %d: часть речи длиннее %d символов", line, maxPOS)
		}
		if pos == "" {
			// Ошибка в данных (в версии 1.0 такая строка одна): без части речи слово не отличить от других.
			slog.Warn("octanove: строка без части речи пропущена", "line", line, "headword", headword)
			continue
		}
		if fixed, ok := posFixes[pos]; ok {
			pos = fixed
		}

		k := key{headword, pos}
		row, seen := rows[k]
		if !seen {
			row = &Row{Headword: headword, POS: pos, Level: level, Extra: map[string][]string{}}
			rows[k] = row
			order = append(order, k)
		}
		if level < row.Level { // "C1" < "C2"
			row.Level = level
		}
		if note != "" && !slices.Contains(row.Extra["notes"], note) {
			row.Extra["notes"] = append(row.Extra["notes"], note)
		}
	}

	for _, k := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(*rows[k]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Source) Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error {
	r, ok := row.(Row)
	if !ok {
		return fmt.Errorf("octanove: ожидали Row, получили %T", row)
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
