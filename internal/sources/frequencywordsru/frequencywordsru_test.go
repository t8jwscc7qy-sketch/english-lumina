package frequencywordsru

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
	if info.Name != "frequencywords-ru" || info.Version != "2018" {
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
	if !strings.Contains(info.URL, "/525f9b560de45753a5ea01069454e72e9aa541c6/") || !strings.HasSuffix(info.URL, "/ru_50k.txt") {
		t.Fatalf("URL не закреплён на коммит: %s", info.URL)
	}
}

func TestParseSample(t *testing.T) {
	rows, err := parseAll(t, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 {
		t.Fatalf("строк %d, ожидали 13", len(rows))
	}
	// Ранг — номер строки, начиная с 1.
	for i, r := range rows {
		if r.Rank != i+1 {
			t.Fatalf("строка %d: ранг %d", i+1, r.Rank)
		}
	}
	if rows[0] != (Row{Word: "я", Count: 5081568, Rank: 1}) {
		t.Fatalf("первая строка: %+v", rows[0])
	}
	if last := rows[len(rows)-1]; last != (Row{Word: "одежда", Count: 4303, Rank: 13}) {
		t.Fatalf("последняя строка: %+v", last)
	}
	byWord := map[string]Row{}
	for _, r := range rows {
		byWord[r.Word] = r
	}
	for _, w := range []string{"что-то", "отец", "папа", "ребёнок", "одежда"} {
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
		if err != nil || len(rows) != 13 || rows[0].Word != "я" {
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
	rows, err := parseAll(t, "а 5\nб 5\nв 4\n")
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
		{"одно поле", "я 10\nword\n", []string{"строка 2", "поля"}},
		{"три поля", "я 10\nне very 9\n", []string{"строка 2", "поля"}},
		{"табуляция вместо пробела", "я\t10\n", []string{"строка 1"}},
		{"счёт не число", "я ten\n", []string{"строка 1", "счёт"}},
		{"счёт ноль", "я 10\nне 0\n", []string{"строка 2", "счёт"}},
		{"счёт отрицательный", "я -3\n", []string{"строка 1", "счёт"}},
		{"счёт с плюсом и дробью", "я 1.5\n", []string{"строка 1", "счёт"}},
		{"счёт не помещается в int64", "я 99999999999999999999\n", []string{"строка 1", "счёт"}},
		{"счёт растёт", "я 10\nне 11\n", []string{"строка 2", "убыв"}},
		{"повтор слова", "я 10\nне 9\nя 8\n", []string{"строка 3", "повтор"}},
		{"пустое слово", " 10\n", []string{"строка 1"}},
		{"слово из 65 символов", strings.Repeat("я", 65) + " 10\n", []string{"строка 1", "64"}},
		{"недопустимый UTF-8", "я\xffя 10\n", []string{"строка 1", "UTF-8"}},
		{"нулевой байт", "я\x00я 10\n", []string{"строка 1", "нулев"}},
		{"повтор слова после пропущенной латиницы", "я 10\ncat 9\ncat 8\n", []string{"строка 3", "повтор"}},
		{"счёт растёт после пропущенной латиницы", "я 10\ncat 9\nа 11\n", []string{"строка 3", "убыв"}},
		{"пустая строка посреди файла", "я 10\n\nне 9\n", []string{"строка 2"}},
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

// Шум субтитров (латиница, смесь букв, цифры, точки) пропускается, а место остальных слов остаётся номером строки файла.
func TestParseSkipsNonCyrillicKeepingRanks(t *testing.T) {
	rows, err := parseAll(t, "я 100\nthe 90\nчтo 80\nа1 70\nо. 60\nне 50\nв-в 40\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{{"я", 100, 1}, {"не", 50, 6}, {"в-в", 40, 7}}
	if len(rows) != len(want) {
		t.Fatalf("строки %+v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("строка %d: %+v, ждали %+v", i, rows[i], want[i])
		}
	}
}
