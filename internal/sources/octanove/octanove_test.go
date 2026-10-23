package octanove

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Путь считается при старте пакета: testdb.Pool меняет рабочий каталог, и относительный путь потом ломается.
var samplePath, _ = filepath.Abs("testdata/sample.csv")

func readSample(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parseAll(t *testing.T, content string) ([]Row, error) {
	t.Helper()
	var rows []Row
	err := New().Parse(context.Background(), strings.NewReader(content), func(r any) error {
		rows = append(rows, r.(Row))
		return nil
	})
	return rows, err
}

func find(t *testing.T, rows []Row, headword, pos string) Row {
	t.Helper()
	for _, r := range rows {
		if r.Headword == headword && r.POS == pos {
			return r
		}
	}
	t.Fatalf("в разборе нет пары %q/%q", headword, pos)
	return Row{}
}

func TestDescribe(t *testing.T) {
	info := New().Describe()
	if info.Name != "octanove" || info.Version != "1.0" {
		t.Fatalf("имя и версия: %+v", info)
	}
	if !strings.Contains(info.License, "CC BY-SA 4.0") {
		t.Fatalf("лицензия: %q", info.License)
	}
	for _, want := range []string{"Octanove Labs", "CC BY-SA 4.0", "Octanove Vocabulary Profile C1/C2"} {
		if !strings.Contains(info.Attribution, want) {
			t.Fatalf("в атрибуции %q нет %q", info.Attribution, want)
		}
	}
	if !strings.Contains(info.URL, "/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/") || !strings.HasSuffix(info.URL, "/octanove-vocabulary-profile-c1c2-1.0.csv") {
		t.Fatalf("URL не закреплён на коммит: %s", info.URL)
	}
}

func TestParseSample(t *testing.T) {
	rows, err := parseAll(t, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	// 24 строки в файле: пустая часть речи пропущена, три пары повторяются → 20 строк.
	if len(rows) != 20 {
		t.Fatalf("строк %d, ожидали 20", len(rows))
	}
	if rows[0].Headword != "exterior" || rows[1].Headword != "cast" || rows[1].POS != "noun" {
		t.Fatalf("порядок первых строк: %+v %+v", rows[0], rows[1])
	}

	// Повтор с разными уровнями: берётся самый низкий.
	if got := find(t, rows, "relinquish", "verb"); got.Level != "C1" {
		t.Fatalf("relinquish: %+v", got)
	}
	if got := find(t, rows, "exhibitionist", "noun"); got.Level != "C1" {
		t.Fatalf("exhibitionist: %+v", got)
	}
	// Повтор с разными заметками: заметки собираются по порядку.
	cast := find(t, rows, "cast", "verb")
	if !slices.Equal(cast.Extra["notes"], []string{"hire actors", "say or suggest something (e.g. doubt)"}) {
		t.Fatalf("cast verb notes: %v", cast.Extra)
	}
	if got := find(t, rows, "cast", "noun"); !slices.Equal(got.Extra["notes"], []string{"plaster cast, mold"}) {
		t.Fatalf("cast noun: %+v", got) // запятая внутри значения в кавычках
	}
	// Без заметок extra пустой.
	if got := find(t, rows, "exterior", "noun"); len(got.Extra) != 0 {
		t.Fatalf("exterior: %+v", got)
	}
	// Опечатка источника исправлена; пробелы по краям обрезаны; слэш-варианты сохранены.
	if got := find(t, rows, "remonstrate", "verb"); got.Level != "C2" {
		t.Fatalf("remonstrate: %+v", got)
	}
	find(t, rows, "turn to", "verb")
	find(t, rows, "laud", "verb")
	find(t, rows, "favorably/favourably", "adverb")
	for _, r := range rows {
		if r.POS == "" || r.POS == "vern" || r.Headword != strings.TrimSpace(r.Headword) {
			t.Fatalf("грязная строка в разборе: %+v", r)
		}
		if r.Level != "C1" && r.Level != "C2" {
			t.Fatalf("уровень %q", r.Level)
		}
	}
	for _, r := range rows {
		if r.Headword == "batter" {
			t.Fatal("строка без части речи не должна попадать в разбор")
		}
	}
}

func TestParseDedupesRepeatedNotes(t *testing.T) {
	rows, err := parseAll(t, "headword,pos,CEFR,notes\r\na,noun,C1,x\r\na,noun,C1,x\r\na,noun,C2,y\r\n")
	if err != nil || len(rows) != 1 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
	if !slices.Equal(rows[0].Extra["notes"], []string{"x", "y"}) || rows[0].Level != "C1" {
		t.Fatalf("%+v", rows[0])
	}
}

func TestParseIgnoresBOM(t *testing.T) {
	rows, err := parseAll(t, "\ufeff"+readSample(t))
	if err != nil || len(rows) != 20 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseHeaderOnlyGivesNoRows(t *testing.T) {
	rows, err := parseAll(t, "headword,pos,CEFR,notes\r\n")
	if err != nil || len(rows) != 0 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseErrors(t *testing.T) {
	const header = "headword,pos,CEFR,notes\n"
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"пустой файл", "", []string{"заголов"}},
		{"заголовок CEFR-J", "headword,pos,CEFR,CoreInventory 1,CoreInventory 2,Threshold\n", []string{"заголов"}},
		{"пустое слово", header + "a,noun,C1,\n,noun,C1,\n", []string{"строка 3", "headword"}},
		{"слово из пробелов", header + "   ,noun,C1,\n", []string{"строка 2"}},
		{"уровень B2 не из этого файла", header + "a,noun,B2,\n", []string{"строка 2", "B2"}},
		{"уровень в нижнем регистре", header + "a,noun,c1,\n", []string{"строка 2", "c1"}},
		{"слово из 65 символов", header + strings.Repeat("x", 65) + ",noun,C1,\n", []string{"строка 2", "64"}},
		{"часть речи из 33 символов", header + "a," + strings.Repeat("x", 33) + ",C1,\n", []string{"строка 2", "32"}},
		{"не хватает полей", header + "a,noun,C1\n", []string{"строка 2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAll(t, tc.content)
			if err == nil {
				t.Fatal("ожидали ошибку")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("в ошибке %q нет %q", err, w)
				}
			}
		})
	}
}

func TestParseStopsWhenEmitFails(t *testing.T) {
	stop := errors.New("стоп")
	n := 0
	err := New().Parse(context.Background(), strings.NewReader(readSample(t)), func(any) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 2 {
		t.Fatalf("ошибка %v, эмитировано %d; ожидали остановку на 2-й строке", err, n)
	}
}
