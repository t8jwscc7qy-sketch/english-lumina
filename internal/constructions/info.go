package constructions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

const maxExplanationWords = 60

// Info — пояснение к конструкции: форма, короткое объяснение по-русски, пример и (если есть) шпаргалка по теме.
type Info struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Form        string   `json:"form"`
	Explanation string   `json:"explanation"`
	ExampleEn   string   `json:"exampleEn"`
	ExampleRu   string   `json:"exampleRu"`
	SheetID     string   `json:"sheetId,omitempty"`
	Sources     []string `json:"sources"`
}

func (i Info) validate() error {
	switch {
	case strings.TrimSpace(i.ID) == "":
		return errors.New("нет id")
	case strings.TrimSpace(i.Title) == "" || strings.TrimSpace(i.Form) == "":
		return errors.New("нужны title и form")
	case strings.TrimSpace(i.ExampleEn) == "" || strings.TrimSpace(i.ExampleRu) == "":
		return errors.New("нужен пример на двух языках")
	case len(i.Sources) == 0:
		return errors.New("нужен хотя бы один источник")
	case strings.TrimSpace(i.Explanation) == "" || len(strings.Fields(i.Explanation)) > maxExplanationWords:
		return fmt.Errorf("пояснение: от 1 до %d слов", maxExplanationWords)
	}
	return nil
}

// LoadAll читает пояснения из constructions/*.json, проверяет их и уникальность id; результат — по id.
func LoadAll(fsys fs.FS) (map[string]Info, error) {
	files, err := fs.Glob(fsys, "constructions/*.json")
	if err != nil {
		return nil, err
	}
	out := map[string]Info{}
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var info Info
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&info); err != nil || dec.More() {
			return nil, fmt.Errorf("%s: разбор: %v", f, err)
		}
		if err := info.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if _, dup := out[info.ID]; dup {
			return nil, fmt.Errorf("%s: id %q повторяется", f, info.ID)
		}
		out[info.ID] = info
	}
	return out, nil
}
