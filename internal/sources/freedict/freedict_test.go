package freedict

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func parseSample(t *testing.T) []Row {
	t.Helper()
	f, err := os.Open("testdata/eng-rus/eng-rus.tei")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	var rows []Row
	err = New().Parse(context.Background(), f, func(row any) error {
		rows = append(rows, row.(Row))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func find(rows []Row, lemma, pos string, sense int, ru string) *Row {
	for i := range rows {
		r := rows[i]
		if r.Lemma == lemma && r.Pos == pos && r.SenseNo == sense && r.Ru == ru {
			return &rows[i]
		}
	}
	return nil
}

func TestParseSampleRowCount(t *testing.T) {
	// apple 1, run n 3, run v 2, beautiful 2, quickly 1, bank 2, salt 1
	if rows := parseSample(t); len(rows) != 12 {
		t.Fatalf("строк %d, ждали 12: %+v", len(rows), rows)
	}
}

func TestParseStripsStressAndDedupes(t *testing.T) {
	rows := parseSample(t)
	if r := find(rows, "apple", "n", 1, "яблоко"); r == nil || r.Definition != "A tree of the genus Malus." {
		t.Fatalf("apple: %+v", r)
	}
	n := 0
	for _, r := range rows {
		if r.Lemma == "apple" {
			n++
		}
		if strings.ContainsRune(r.Ru, '́') {
			t.Fatalf("знак ударения остался: %q", r.Ru)
		}
	}
	if n != 1 {
		t.Fatalf("«яблоко» и «я́блоко» — один перевод, получили %d строк", n)
	}
	if find(rows, "run", "n", 1, "пробежка") == nil || find(rows, "quickly", "adv", 1, "быстро") == nil {
		t.Fatal("переводы без ударения не найдены")
	}
}

func TestParseSenseNumbersPerLemmaAndPos(t *testing.T) {
	rows := parseSample(t)
	for _, c := range []struct {
		lemma, pos, ru string
		sense          int
	}{
		{"run", "n", "бег", 1}, {"run", "n", "стрелка", 2}, {"run", "v", "бежать", 1},
		{"bank", "n", "банк", 1}, {"bank", "n", "берег", 2}, // два одинаковых слова подряд продолжают нумерацию
	} {
		if find(rows, c.lemma, c.pos, c.sense, c.ru) == nil {
			t.Errorf("нет %s/%s #%d %q", c.lemma, c.pos, c.sense, c.ru)
		}
	}
}

func TestParseSkipsWhatExerciseCannotUse(t *testing.T) {
	for _, r := range parseSample(t) {
		switch r.Lemma {
		case "Paris", "ice cream", "in a nutshell", "x1":
			t.Fatalf("лишнее слово %q (%s)", r.Lemma, r.Pos)
		}
		if r.Ru == "ok" || r.Ru == "" {
			t.Fatalf("перевод без кириллицы или пустой: %+v", r)
		}
	}
}

func TestParseSenseWithoutTranslationsDoesNotConsumeNumber(t *testing.T) {
	rows := parseSample(t)
	if find(rows, "beautiful", "adj", 1, "красивый") == nil || find(rows, "beautiful", "adj", 1, "прекрасный") == nil {
		t.Fatal("beautiful: первое значение")
	}
	for _, r := range rows {
		if r.Lemma == "beautiful" && r.SenseNo != 1 {
			t.Fatalf("значение без переводов пропускается: %+v", r)
		}
	}
}

func TestParseDecodesEntities(t *testing.T) {
	if r := find(parseSample(t), "salt", "n", 1, "соль"); r == nil || r.Definition != "Salt & pepper, a seasoning." {
		t.Fatalf("salt: %+v", r)
	}
}

func TestParseBrokenXMLIsAnError(t *testing.T) {
	err := New().Parse(context.Background(), strings.NewReader(`<TEI><text><body><entry><form><orth>a`), func(any) error { return nil })
	if err == nil {
		t.Fatal("оборванный XML должен давать ошибку")
	}
}

func TestParseStopsOnEmitError(t *testing.T) {
	f, err := os.Open("testdata/eng-rus/eng-rus.tei")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	boom := errors.New("стоп")
	n := 0
	err = New().Parse(context.Background(), f, func(any) error {
		n++
		return boom
	})
	if !errors.Is(err, boom) || n != 1 {
		t.Fatalf("ждали остановку после первой строки: err=%v n=%d", err, n)
	}
}

func TestParseRespectsCancelledContext(t *testing.T) {
	f, err := os.Open("testdata/eng-rus/eng-rus.tei")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New().Parse(ctx, f, func(any) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали context.Canceled, получили %v", err)
	}
}

func TestCleanTranslation(t *testing.T) {
	for in, want := range map[string]string{
		"  я́блоко ":            "яблоко",
		"ходи́ть пешко́м":       "ходить пешком",
		"ok":                    "",
		"":                      "",
		"   ":                   "",
		strings.Repeat("я", 65): "",
	} {
		if got := cleanTranslation(in); got != want {
			t.Errorf("cleanTranslation(%q) = %q, ждали %q", in, got, want)
		}
	}
}

func TestValidLemma(t *testing.T) {
	for in, want := range map[string]bool{
		"apple": true, "mother-in-law": true, "don't": true, "April": true,
		"ice cream": false, "x1": false, "": false, "-a": false, strings.Repeat("a", 65): false,
	} {
		if got := validLemma(in); got != want {
			t.Errorf("validLemma(%q) = %v, ждали %v", in, got, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	info := New().Describe()
	if info.Name != "freedict-eng-rus" || info.Version == "" || !strings.Contains(info.License, "CC BY-SA 3.0") ||
		!strings.Contains(info.Attribution, "wiktionary.org") || !strings.HasPrefix(info.URL, "https://") {
		t.Fatalf("Describe: %+v", info)
	}
}
