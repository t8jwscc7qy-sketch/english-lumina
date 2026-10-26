// Package sheets описывает шпаргалки (справочные таблицы и правила), проверяет их схему и загружает встроенные тексты.
// Шпаргалки не хранятся в базе: правка текста не требует миграций. В базе лежит только состояние пользователя.
package sheets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Типы блоков.
const (
	BlockText     = "text"
	BlockTable    = "table"
	BlockTip      = "tip"
	BlockMistakes = "mistakes"
	BlockExamples = "examples"
)

// Привязки таблицы к данным пользователя.
const (
	BindWord    = "word"    // ключ строки — лемма
	BindTopic   = "topic"   // ключ строки — тема упражнений (grammar:articles)
	BindMistake = "mistake" // строки собирает сервер из банка ошибок
)

const (
	maxTextWords   = 60
	maxTipWords    = 25
	maxTags        = 6
	minExamples    = 3
	maxExamples    = 5
	StatusDraft    = "draft"
	StatusReviewed = "reviewed"
	MineMistakesID = "mine-mistakes"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var levels = []string{"A1", "A2", "B1", "B2", "C1"}

// Column — столбец таблицы.
type Column struct {
	Title string `json:"title"`
}

// Row — строка таблицы: ключ привязки (может быть пустым у обычной таблицы), ячейки и пометка «проверить».
type Row struct {
	Key        string   `json:"key,omitempty"`
	Cells      []string `json:"cells"`
	NeedsCheck bool     `json:"needsCheck,omitempty"`
}

// Pair — пара «неверно → верно» с пояснением.
type Pair struct {
	Wrong string `json:"wrong"`
	Right string `json:"right"`
	Note  string `json:"note"`
}

// Example — пример: английский и русский. Изучаемая форма выделяется как **форма**.
type Example struct {
	En string `json:"en"`
	Ru string `json:"ru"`
}

// Block — блок шпаргалки; заполнены только поля, нужные его типу.
type Block struct {
	Type     string    `json:"type"`
	Title    string    `json:"title,omitempty"`
	Text     string    `json:"text,omitempty"`
	Bind     string    `json:"bind,omitempty"`
	Columns  []Column  `json:"columns,omitempty"`
	Rows     []Row     `json:"rows,omitempty"`
	Pairs    []Pair    `json:"pairs,omitempty"`
	Examples []Example `json:"examples,omitempty"`
}

// Sheet — шпаргалка. Status в файле всегда draft: «проверено» — отметка пользователя в базе.
type Sheet struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Level   string   `json:"level"`
	Tags    []string `json:"tags"`
	Blocks  []Block  `json:"blocks"`
	Sources []string `json:"sources"`
	Status  string   `json:"status"`
}

func words(s string) int { return len(strings.Fields(s)) }

// Validate проверяет шпаргалку по схеме. Ошибка называет первое нарушение.
func (s Sheet) Validate() error {
	switch {
	case !idPattern.MatchString(s.ID):
		return fmt.Errorf("id %q: латиница в нижнем регистре, цифры и дефисы", s.ID)
	case strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.Summary) == "":
		return errors.New("нужны title и summary")
	case !slices.Contains(levels, s.Level):
		return fmt.Errorf("уровень %q: A1…C1", s.Level)
	case len(s.Tags) > maxTags:
		return fmt.Errorf("тегов больше %d", maxTags)
	case len(s.Sources) == 0:
		return errors.New("нужен хотя бы один источник (sources)")
	case s.Status != StatusDraft:
		return fmt.Errorf("в файле status = draft (проверено ставит пользователь), а не %q", s.Status)
	case len(s.Blocks) == 0:
		return errors.New("нет блоков")
	}
	for _, src := range s.Sources {
		if strings.TrimSpace(src) == "" {
			return errors.New("пустой источник")
		}
	}
	for i, b := range s.Blocks {
		if err := b.validate(); err != nil {
			return fmt.Errorf("блок %d (%s): %w", i+1, b.Type, err)
		}
	}
	return nil
}

func (b Block) validate() error {
	switch b.Type {
	case BlockText:
		if strings.TrimSpace(b.Text) == "" || words(b.Text) > maxTextWords {
			return fmt.Errorf("текст: от 1 до %d слов", maxTextWords)
		}
	case BlockTip:
		if strings.TrimSpace(b.Text) == "" || words(b.Text) > maxTipWords || strings.Contains(b.Text, "\n") {
			return fmt.Errorf("«Запомни»: одна строка до %d слов", maxTipWords)
		}
	case BlockTable:
		return b.validateTable()
	case BlockMistakes:
		if len(b.Pairs) == 0 {
			return errors.New("нет пар")
		}
		for _, p := range b.Pairs {
			if strings.TrimSpace(p.Wrong) == "" || strings.TrimSpace(p.Right) == "" || strings.TrimSpace(p.Note) == "" {
				return errors.New("в паре нужны wrong, right и note")
			}
		}
	case BlockExamples:
		if len(b.Examples) < minExamples || len(b.Examples) > maxExamples {
			return fmt.Errorf("примеров от %d до %d", minExamples, maxExamples)
		}
		for _, e := range b.Examples {
			if strings.TrimSpace(e.En) == "" || strings.TrimSpace(e.Ru) == "" {
				return errors.New("в примере нужны en и ru")
			}
		}
	default:
		return fmt.Errorf("неизвестный тип блока %q", b.Type)
	}
	return nil
}

func (b Block) validateTable() error {
	if b.Bind != "" && !slices.Contains([]string{BindWord, BindTopic, BindMistake}, b.Bind) {
		return fmt.Errorf("привязка %q: word, topic или mistake", b.Bind)
	}
	if len(b.Columns) < 2 {
		return errors.New("столбцов не меньше двух")
	}
	if len(b.Rows) == 0 && b.Bind != BindMistake { // строки «Моих ошибок» собирает сервер
		return errors.New("нет строк")
	}
	seen := map[string]bool{}
	for i, r := range b.Rows {
		if len(r.Cells) != len(b.Columns) {
			return fmt.Errorf("строка %d: ячеек %d, столбцов %d", i+1, len(r.Cells), len(b.Columns))
		}
		if b.Bind == BindWord || b.Bind == BindTopic {
			if strings.TrimSpace(r.Key) == "" {
				return fmt.Errorf("строка %d: у привязанной таблицы нужен key", i+1)
			}
			if seen[r.Key] {
				return fmt.Errorf("строка %d: повторный key %q", i+1, r.Key)
			}
			seen[r.Key] = true
		}
	}
	return nil
}

// Parse разбирает и проверяет одну шпаргалку; неизвестные поля — ошибка.
func Parse(data []byte) (Sheet, error) {
	var s Sheet
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Sheet{}, fmt.Errorf("разбор: %w", err)
	}
	if dec.More() {
		return Sheet{}, errors.New("после шпаргалки в файле есть лишнее")
	}
	if err := s.Validate(); err != nil {
		return Sheet{}, fmt.Errorf("шпаргалка %q: %w", s.ID, err)
	}
	return s, nil
}

// LoadAll читает все файлы *.json каталога sheets из fsys, проверяет их и уникальность id. Порядок — по id.
func LoadAll(fsys fs.FS) ([]Sheet, error) {
	files, err := fs.Glob(fsys, "sheets/*.json")
	if err != nil {
		return nil, err
	}
	var out []Sheet
	ids := map[string]string{}
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		s, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if prev, dup := ids[s.ID]; dup {
			return nil, fmt.Errorf("%s: id %q уже есть в %s", f, s.ID, prev)
		}
		ids[s.ID] = f
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// WordKeys — все ключи привязки word (леммы) шпаргалки.
func (s Sheet) WordKeys() []string {
	var keys []string
	for _, b := range s.Blocks {
		if b.Type == BlockTable && b.Bind == BindWord {
			for _, r := range b.Rows {
				keys = append(keys, r.Key)
			}
		}
	}
	return keys
}
