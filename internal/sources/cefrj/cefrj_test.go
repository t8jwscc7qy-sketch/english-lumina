package cefrj

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func find(t *testing.T, rows []Row, headword string) Row {
	t.Helper()
	for _, r := range rows {
		if r.Headword == headword {
			return r
		}
	}
	t.Fatalf("в разборе нет слова %q", headword)
	return Row{}
}

func TestDescribe(t *testing.T) {
	info := New().Describe()
	if info.Name != "cefrj" || info.Version != "1.5" {
		t.Fatalf("имя и версия: %+v", info)
	}
	if info.Attribution != "The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies." {
		t.Fatalf("атрибуция: %q", info.Attribution)
	}
	if !strings.Contains(info.URL, "/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/") || !strings.HasSuffix(info.URL, "/cefrj-vocabulary-profile-1.5.csv") {
		t.Fatalf("URL не закреплён на коммит: %s", info.URL)
	}
	if info.License == "" {
		t.Fatal("не указана лицензия")
	}
}

func TestParseSample(t *testing.T) {
	rows, err := parseAll(t, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 32 {
		t.Fatalf("строк %d, ожидали 32", len(rows))
	}
	levels := map[string]int{}
	for _, r := range rows {
		levels[r.Level]++
	}
	if levels["A1"] != 12 || levels["A2"] != 6 || levels["B1"] != 9 || levels["B2"] != 5 {
		t.Fatalf("уровни: %v", levels)
	}

	if a := rows[0]; a.Headword != "a" || a.POS != "determiner" || a.Level != "A1" || len(a.Extra) != 0 {
		t.Fatalf("первая строка: %+v", a)
	}
	if got := find(t, rows, "cafe/café"); got.Level != "A1" || got.Extra["threshold"] != "Food and drink" {
		t.Fatalf("cafe/café: %+v", got)
	}
	if got := find(t, rows, "a.m./A.M./am/AM"); got.POS != "adverb" {
		t.Fatalf("вариант через слэш: %+v", got)
	}
	if got := find(t, rows, "can"); got.POS != "modal auxiliary" || len(got.Extra) != 0 {
		t.Fatalf("can: %+v", got)
	}
	adviser := find(t, rows, "adviser/advisor")
	want := map[string]string{"coreInventory1": "Education", "coreInventory2": "Work and Jobs", "threshold": "Personal identification"}
	if len(adviser.Extra) != 3 {
		t.Fatalf("extra: %v", adviser.Extra)
	}
	for k, v := range want {
		if adviser.Extra[k] != v {
			t.Fatalf("extra[%s] = %q, ожидали %q", k, adviser.Extra[k], v)
		}
	}
	// Запятая внутри значения в кавычках не делит поле; пустые поля в extra не попадают.
	abuse := find(t, rows, "abuse")
	if len(abuse.Extra) != 1 || abuse.Extra["coreInventory1"] != "News, lifestyles and current affairs" {
		t.Fatalf("abuse.extra = %v", abuse.Extra)
	}
}

func TestParseIgnoresBOM(t *testing.T) {
	rows, err := parseAll(t, "\ufeff"+readSample(t))
	if err != nil || len(rows) != 32 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseHeaderOnlyGivesNoRows(t *testing.T) {
	rows, err := parseAll(t, "headword,pos,CEFR,CoreInventory 1,CoreInventory 2,Threshold\n")
	if err != nil || len(rows) != 0 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseErrors(t *testing.T) {
	const header = "headword,pos,CEFR,CoreInventory 1,CoreInventory 2,Threshold\n"
	tests := []struct {
		name    string
		content string
		want    []string // подстроки, которые должны быть в ошибке
	}{
		{"пустой файл", "", []string{"заголов"}},
		{"чужой заголовок", "word,pos,level\na,noun,A1\n", []string{"заголов"}},
		{"заголовок с лишней колонкой", "headword,pos,CEFR,CoreInventory 1,CoreInventory 2,Threshold,Extra\n", []string{"заголов"}},
		{"пустое слово", header + "a,noun,A1,,,\n,noun,A1,,,\n", []string{"строка 3", "headword"}},
		{"пустая часть речи", header + ",,A1,,,\nb,,A1,,,\n", []string{"строка 2"}},
		{"неизвестный уровень", header + "a,noun,A1,,,\nb,noun,D1,,,\n", []string{"строка 3", "D1"}},
		{"уровень в нижнем регистре", header + "a,noun,a1,,,\n", []string{"строка 2", "a1"}},
		{"слово из 65 символов", header + strings.Repeat("x", 65) + ",noun,A1,,,\n", []string{"строка 2", "64"}},
		{"часть речи из 33 символов", header + "a," + strings.Repeat("x", 33) + ",A1,,,\n", []string{"строка 2", "32"}},
		{"не хватает полей", header + "a,noun,A1\n", []string{"строка 2"}},
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
		if n == 3 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 3 {
		t.Fatalf("ошибка %v, эмитировано %d; ожидали остановку на 3-й строке", err, n)
	}
}
