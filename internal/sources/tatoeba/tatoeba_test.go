package tatoeba

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

const attr = "CC-BY 2.0 (France) Attribution: tatoeba.org #1 (CM) & #2 (shanghainese)"

func line(en, ru, attribution string) string { return en + "\t" + ru + "\t" + attribution + "\n" }

func TestDescribe(t *testing.T) {
	info := New().Describe()
	if info.Name != "tatoeba" || info.Version != "2026-02-13" {
		t.Fatalf("имя и версия: %+v", info)
	}
	if !strings.Contains(info.License, "CC BY 2.0 FR") {
		t.Fatalf("лицензия: %q", info.License)
	}
	for _, want := range []string{"Tatoeba", "manythings.org", "CC BY 2.0 FR"} {
		if !strings.Contains(info.Attribution, want) {
			t.Fatalf("в атрибуции %q нет %q", info.Attribution, want)
		}
	}
	if info.URL != "https://www.manythings.org/anki/rus-eng.zip" {
		t.Fatalf("URL: %s", info.URL)
	}
}

func TestParseSample(t *testing.T) {
	rows, err := parseAll(t, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	// 34 строки в файле, 2 английских длиннее 30 слов пропущены.
	if len(rows) != 32 {
		t.Fatalf("пар %d, ожидали 32", len(rows))
	}
	en, ru := map[int64]bool{}, map[int64]int{}
	for _, r := range rows {
		en[r.EnID] = true
		ru[r.RuID]++
		if len(strings.Fields(r.EnText)) > maxEnglishWords {
			t.Fatalf("длинное английское не пропущено: %+v", r)
		}
		if r.EnID == 2675809 || r.EnID == 3693622 {
			t.Fatalf("пропущенное длинное предложение попало в разбор: %+v", r)
		}
	}
	if len(en) != 27 || len(ru) != 29 {
		t.Fatalf("уникальных английских %d, русских %d; ожидали 27 и 29", len(en), len(ru))
	}
	if ru[402127] < 2 {
		t.Fatalf("русское предложение 402127 должно быть общим для нескольких английских, пар с ним %d", ru[402127])
	}

	want := Row{EnID: 2877272, EnText: "Go.", EnAuthor: "CM", RuID: 1159202, RuText: "Марш!", RuAuthor: "shanghainese"}
	if rows[0] != want {
		t.Fatalf("первая пара %+v, ожидали %+v", rows[0], want)
	}
	// Последняя строка образца синтетическая: автор с точкой и дефисом (в настоящих данных таких нет).
	last := Row{EnID: 2951771, EnText: "Do you still play golf?", EnAuthor: "a.b-c", RuID: 3544517, RuText: "Ты всё ещё играешь в гольф?", RuAuthor: "d.e-f"}
	if rows[len(rows)-1] != last {
		t.Fatalf("последняя пара %+v, ожидали %+v", rows[len(rows)-1], last)
	}
	var quoted bool
	for _, r := range rows {
		if r.EnID == 2852741 && r.EnText == `Say "aah."` && r.RuText == `Скажи "а-а-а".` {
			quoted = true
		}
	}
	if !quoted {
		t.Fatal(`предложение с кавычками разобрано неверно`)
	}
}

func TestParseCRLF(t *testing.T) {
	rows, err := parseAll(t, strings.ReplaceAll(readSample(t), "\n", "\r\n"))
	if err != nil || len(rows) != 32 {
		t.Fatalf("пар %d, ошибка %v", len(rows), err)
	}
	for _, r := range rows {
		if strings.ContainsAny(r.EnText+r.RuText+r.EnAuthor+r.RuAuthor, "\r\n") {
			t.Fatalf("\\r попал в поле: %+v", r)
		}
	}
}

func TestParseEmptyFileGivesNoRows(t *testing.T) {
	rows, err := parseAll(t, "")
	if err != nil || len(rows) != 0 {
		t.Fatalf("пар %d, ошибка %v", len(rows), err)
	}
}

func TestParseErrors(t *testing.T) {
	good := line("Go.", "Марш!", attr)
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"две колонки", good + "Go.\tМарш!\n", []string{"строка 2", "поля"}},
		{"четыре колонки", good + line("Go.", "Марш!", attr) + "x\ty\tz\tw\n", []string{"строка 3", "поля"}},
		{"пустая строка посреди файла", good + "\n" + good, []string{"строка 2"}},
		{"чужая лицензия", good + line("Go.", "Марш!", "CC0 Attribution: tatoeba.org #1 (CM) & #2 (a)"), []string{"строка 2", "атрибуц"}},
		{"нет второго номера", good + line("Go.", "Марш!", "CC-BY 2.0 (France) Attribution: tatoeba.org #1 (CM)"), []string{"строка 2", "атрибуц"}},
		{"номер не число", good + line("Go.", "Марш!", "CC-BY 2.0 (France) Attribution: tatoeba.org #x (CM) & #2 (a)"), []string{"строка 2", "атрибуц"}},
		{"номер равен нулю", good + line("Go.", "Марш!", "CC-BY 2.0 (France) Attribution: tatoeba.org #0 (CM) & #2 (a)"), []string{"строка 2", "номер"}},
		{"пустой автор", good + line("Go.", "Марш!", "CC-BY 2.0 (France) Attribution: tatoeba.org #1 () & #2 (a)"), []string{"строка 2", "атрибуц"}},
		{"пустое английское", good + line("", "Марш!", attr), []string{"строка 2", "пуст"}},
		{"пустое русское", good + line("Go.", "  ", attr), []string{"строка 2", "пуст"}},
		{"недопустимый UTF-8", good + line("Go\xff.", "Марш!", attr), []string{"строка 2", "UTF-8"}},
		{"нулевой байт", good + line("Go\x00.", "Марш!", attr), []string{"строка 2", "нулев"}},
		{"текст из 1001 символа", good + line(strings.Repeat("a ", 500)+"b", "Марш!", attr), []string{"строка 2", "1000"}},
		{"автор из 65 символов", good + line("Go.", "Марш!", "CC-BY 2.0 (France) Attribution: tatoeba.org #1 ("+strings.Repeat("a", 65)+") & #2 (b)"), []string{"строка 2", "64"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := parseAll(t, tc.content)
			if err == nil {
				t.Fatalf("ожидали ошибку, разобрано %d пар", len(rows))
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
		t.Fatalf("ошибка %v, эмитировано %d; ожидали остановку на 3-й паре", err, n)
	}
}

func TestParseStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := New().Parse(ctx, strings.NewReader(readSample(t)), func(any) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка %v, ожидали context.Canceled", err)
	}
}
