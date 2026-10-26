package sheets

import (
	"strings"
	"testing"
	"testing/fstest"
)

const valid = `{"id":"to-be","title":"Глагол to be","summary":"Когда нужен be","level":"A1","tags":["глаголы"],
"blocks":[{"type":"text","text":"Короткий текст."},
 {"type":"table","bind":"word","columns":[{"title":"Слово"},{"title":"Перевод"}],"rows":[{"key":"go","cells":["go","идти"]}]}],
"sources":["собственные знания автора"],"status":"draft"}`

func TestParseValid(t *testing.T) {
	s, err := Parse([]byte(valid))
	if err != nil || s.ID != "to-be" || len(s.Blocks) != 2 {
		t.Fatalf("%+v %v", s, err)
	}
	if keys := s.WordKeys(); len(keys) != 1 || keys[0] != "go" {
		t.Fatalf("ключи: %v", keys)
	}
}

func TestParseRejects(t *testing.T) {
	long := strings.Repeat("слово ", 61)
	cases := map[string]string{
		"id с заглавной":          strings.Replace(valid, `"id":"to-be"`, `"id":"To-Be"`, 1),
		"пустой title":            strings.Replace(valid, `"title":"Глагол to be"`, `"title":" "`, 1),
		"уровень вне шкалы":       strings.Replace(valid, `"level":"A1"`, `"level":"C2"`, 1),
		"нет источников":          strings.Replace(valid, `"sources":["собственные знания автора"]`, `"sources":[]`, 1),
		"статус reviewed в файле": strings.Replace(valid, `"status":"draft"`, `"status":"reviewed"`, 1),
		"неизвестный тип блока":   strings.Replace(valid, `"type":"text"`, `"type":"video"`, 1),
		"длинный текст":           strings.Replace(valid, "Короткий текст.", long, 1),
		"неизвестная привязка":    strings.Replace(valid, `"bind":"word"`, `"bind":"stars"`, 1),
		"ячеек меньше столбцов":   strings.Replace(valid, `"cells":["go","идти"]`, `"cells":["go"]`, 1),
		"привязка без key":        strings.Replace(valid, `"key":"go",`, ``, 1),
		"повторный key":           strings.Replace(valid, `"rows":[{"key":"go","cells":["go","идти"]}]`, `"rows":[{"key":"go","cells":["go","идти"]},{"key":"go","cells":["go","ехать"]}]`, 1),
		"лишнее поле":             strings.Replace(valid, `"status":"draft"`, `"status":"draft","extra":1`, 1),
		"не JSON":                 `{`,
		"хвост после объекта":     valid + `{}`,
	}
	for name, data := range cases {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
}

func TestBlockRules(t *testing.T) {
	good := map[string]Block{
		"tip":      {Type: BlockTip, Text: "Be нельзя опустить."},
		"mistakes": {Type: BlockMistakes, Pairs: []Pair{{Wrong: "I am agree", Right: "I agree", Note: "agree — глагол"}}},
		"examples": {Type: BlockExamples, Examples: []Example{{"a", "а"}, {"b", "б"}, {"c", "в"}}},
		"mine":     {Type: BlockTable, Bind: BindMistake, Columns: []Column{{"Тема"}, {"Ошибка"}}},
	}
	for name, b := range good {
		if err := b.validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := map[string]Block{
		"tip в две строки":   {Type: BlockTip, Text: "один\nдва"},
		"пустые пары":        {Type: BlockMistakes},
		"пара без пояснения": {Type: BlockMistakes, Pairs: []Pair{{Wrong: "a", Right: "b"}}},
		"два примера":        {Type: BlockExamples, Examples: []Example{{"a", "а"}, {"b", "б"}}},
		"таблица без строк":  {Type: BlockTable, Columns: []Column{{"a"}, {"b"}}},
		"один столбец":       {Type: BlockTable, Columns: []Column{{"a"}}, Rows: []Row{{Cells: []string{"x"}}}},
	}
	for name, b := range bad {
		if err := b.validate(); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
}

func TestLoadAll(t *testing.T) {
	fsys := fstest.MapFS{
		"sheets/b.json": {Data: []byte(strings.Replace(valid, `"id":"to-be"`, `"id":"b"`, 1))},
		"sheets/a.json": {Data: []byte(strings.Replace(valid, `"id":"to-be"`, `"id":"a"`, 1))},
	}
	got, err := LoadAll(fsys)
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("%+v %v", got, err)
	}
	fsys["sheets/c.json"] = &fstest.MapFile{Data: []byte(strings.Replace(valid, `"id":"to-be"`, `"id":"a"`, 1))}
	if _, err := LoadAll(fsys); err == nil || !strings.Contains(err.Error(), "уже есть") {
		t.Fatalf("повторный id: %v", err)
	}
	fsys = fstest.MapFS{"sheets/x.json": {Data: []byte(`{}`)}}
	if _, err := LoadAll(fsys); err == nil || !strings.Contains(err.Error(), "sheets/x.json") {
		t.Fatalf("ошибка называет файл: %v", err)
	}
}
