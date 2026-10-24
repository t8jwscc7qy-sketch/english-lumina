package frequencywords

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Путь считается при старте пакета: testdb.Pool меняет рабочий каталог, и относительный путь потом ломается.
var samplePath, _ = filepath.Abs("testdata/sample.txt")

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

func TestDescribe(t *testing.T) {
	info := New().Describe()
	if info.Name != "frequencywords" || info.Version != "2018" {
		t.Fatalf("имя и версия: %+v", info)
	}
	if !strings.Contains(info.License, "CC BY-SA 4.0") {
		t.Fatalf("лицензия: %q", info.License)
	}
	for _, want := range []string{"Hermit Dave", "CC BY-SA 4.0", "OpenSubtitles", "Lison", "Tiedemann"} {
		if !strings.Contains(info.Attribution, want) {
			t.Fatalf("в атрибуции %q нет %q", info.Attribution, want)
		}
	}
	if !strings.Contains(info.URL, "/525f9b560de45753a5ea01069454e72e9aa541c6/") || !strings.HasSuffix(info.URL, "/en_50k.txt") {
		t.Fatalf("URL не закреплён на коммит: %s", info.URL)
	}
}

func TestParseSample(t *testing.T) {
	rows, err := parseAll(t, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 24 {
		t.Fatalf("строк %d, ожидали 24", len(rows))
	}
	// Ранг — номер строки, начиная с 1.
	for i, r := range rows {
		if r.Rank != i+1 {
			t.Fatalf("строка %d: ранг %d", i+1, r.Rank)
		}
	}
	if rows[0] != (Row{Word: "you", Count: 28787591, Rank: 1}) {
		t.Fatalf("первая строка: %+v", rows[0])
	}
	if last := rows[len(rows)-1]; last != (Row{Word: "buddy-buddy", Count: 159, Rank: 24}) {
		t.Fatalf("последняя строка: %+v", last)
	}
	byWord := map[string]Row{}
	for _, r := range rows {
		byWord[r.Word] = r
	}
	for _, w := range []string{"'s", "'t", "café", "run", "running", "ran"} {
		if _, ok := byWord[w]; !ok {
			t.Fatalf("в разборе нет %q", w)
		}
	}
}

func TestParseCRLFAndBOM(t *testing.T) {
	for name, content := range map[string]string{
		"CRLF": strings.ReplaceAll(readSample(t), "\n", "\r\n"),
		"BOM":  "\ufeff" + readSample(t),
	} {
		rows, err := parseAll(t, content)
		if err != nil || len(rows) != 24 || rows[0].Word != "you" {
			t.Fatalf("%s: строк %d, ошибка %v", name, len(rows), err)
		}
	}
}

func TestParseEmptyFileGivesNoRows(t *testing.T) {
	rows, err := parseAll(t, "")
	if err != nil || len(rows) != 0 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseEqualCountsAreAllowed(t *testing.T) {
	rows, err := parseAll(t, "a 5\nb 5\nc 4\n")
	if err != nil || len(rows) != 3 || rows[1].Rank != 2 {
		t.Fatalf("строк %d, ошибка %v", len(rows), err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"одно поле", "you 10\nword\n", []string{"строка 2", "поля"}},
		{"три поля", "you 10\nthe very 9\n", []string{"строка 2", "поля"}},
		{"табуляция вместо пробела", "you\t10\n", []string{"строка 1"}},
		{"счёт не число", "you ten\n", []string{"строка 1", "счёт"}},
		{"счёт ноль", "you 10\nthe 0\n", []string{"строка 2", "счёт"}},
		{"счёт отрицательный", "you -3\n", []string{"строка 1", "счёт"}},
		{"счёт с плюсом и дробью", "you 1.5\n", []string{"строка 1", "счёт"}},
		{"счёт не помещается в int64", "you 99999999999999999999\n", []string{"строка 1", "счёт"}},
		{"счёт растёт", "you 10\nthe 11\n", []string{"строка 2", "убыв"}},
		{"повтор слова", "you 10\nthe 9\nyou 8\n", []string{"строка 3", "повтор"}},
		{"пустое слово", " 10\n", []string{"строка 1"}},
		{"слово из 65 символов", strings.Repeat("x", 65) + " 10\n", []string{"строка 1", "64"}},
		{"недопустимый UTF-8", "yo\xffu 10\n", []string{"строка 1", "UTF-8"}},
		{"нулевой байт", "yo\x00u 10\n", []string{"строка 1", "нулев"}},
		{"пустая строка посреди файла", "you 10\n\nthe 9\n", []string{"строка 2"}},
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
		t.Fatalf("ошибка %v, эмитировано %d", err, n)
	}
}

func TestParseStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New().Parse(ctx, strings.NewReader(readSample(t)), func(any) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка %v, ожидали context.Canceled", err)
	}
}
