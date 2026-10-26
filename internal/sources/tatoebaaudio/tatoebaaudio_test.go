package tatoebaaudio

import (
	"context"
	"os"
	"strings"
	"testing"
)

func parseAll(t *testing.T, path string) ([]Row, error) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var rows []Row
	err = New().Parse(context.Background(), f, func(row any) error {
		rows = append(rows, row.(Row))
		return nil
	})
	return rows, err
}

func TestParseKeepsOnlyAllowedLicenses(t *testing.T) {
	rows, err := parseAll(t, "testdata/sample.tar.bz2")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("записей %d, ждали 3 (CC BY 4.0, CC0, CC BY-SA 4.0): %+v", len(rows), rows)
	}
	want := []Row{
		{SentenceID: 1, AudioID: 100, Username: "alice", License: "CC BY 4.0", AttributionURL: "https://example.org/alice"},
		{SentenceID: 4, AudioID: 103, Username: "dave", License: "CC0 1.0"},
		{SentenceID: 5, AudioID: 104, Username: "erin", License: "CC BY-SA 4.0"},
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("запись %d: %+v, ждали %+v", i, rows[i], want[i])
		}
	}
}

func TestParseLine(t *testing.T) {
	bad := []string{
		"", "1\t2\tbob", "x\t2\tbob\tCC BY 4.0", "1\t0\tbob\tCC BY 4.0", "-1\t2\tbob\tCC BY 4.0", "1\t2\t\tCC BY 4.0",
		"1\t2\t" + strings.Repeat("я", 65) + "\tCC BY 4.0", "1\t2\tbob\tCC BY 4.0\ta\tb", "1\t2\tbo\x00b\tCC BY 4.0",
	}
	for _, line := range bad {
		if _, ok, err := parseLine(line); err == nil && ok {
			t.Errorf("%q: должна быть ошибка", line)
		}
	}
	// Не разрешённые лицензии — не ошибка, а пропуск.
	for _, lic := range []string{"CC BY-NC 4.0", "CC BY-NC-ND 3.0", "", `\N`, "Public domain"} {
		if _, ok, err := parseLine("1\t2\tbob\t" + lic); ok || err != nil {
			t.Errorf("лицензия %q: ok=%v err=%v, ждали пропуск", lic, ok, err)
		}
	}
}

func TestParseRejectsBrokenArchiveAndDuplicates(t *testing.T) {
	if _, err := parseAll(t, "tatoebaaudio.go"); err == nil {
		t.Fatal("не архив — ошибка")
	}
}

func TestDescribeNamesLicensePolicy(t *testing.T) {
	info := New().Describe()
	if info.Name != "tatoeba-audio" || !strings.Contains(info.License, "CC BY") || !strings.Contains(info.Attribution, "Tatoeba") {
		t.Fatalf("%+v", info)
	}
	for _, l := range AllowedLicenses {
		if strings.Contains(l, "NC") || strings.Contains(l, "ND") {
			t.Fatalf("в списке разрешённых не должно быть NC и ND: %s", l)
		}
	}
}
