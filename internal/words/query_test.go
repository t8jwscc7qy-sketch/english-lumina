package words

import (
	"errors"
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Query
	}{
		{"обычное слово", "Run", Query{Exact: "run", Prefix: "run%", Variant: "%/run%"}},
		{"пробелы по краям", "  run  ", Query{Exact: "run", Prefix: "run%", Variant: "%/run%"}},
		{"регистр и диакритика", "CAFÉ", Query{Exact: "café", Prefix: "café%", Variant: "%/café%"}},
		{"пробел внутри (фразовый глагол)", "Turn To", Query{Exact: "turn to", Prefix: "turn to%", Variant: "%/turn to%"}},
		// Точное совпадение сравнивается через =, поэтому не экранируется; шаблоны LIKE экранируются.
		{"спецсимволы LIKE", `a%_\`, Query{Exact: `a%_\`, Prefix: `a\%\_\\%`, Variant: `%/a\%\_\\%`}},
		{"только процент", "%", Query{Exact: "%", Prefix: `\%%`, Variant: `%/\%%`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseQuery(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ParseQuery(%q) = %+v, ожидали %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseQueryErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"пусто", "", ErrEmpty},
		{"одни пробелы", " \t ", ErrEmpty},
		{"65 символов", strings.Repeat("x", 65), ErrTooLong},
		{"нулевой байт", "ru\x00n", ErrInvalid},
		{"не UTF-8", "ru\xffn", ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseQuery(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("ошибка %v, ожидали %v", err, tc.want)
			}
		})
	}
}

func TestParseQueryLimitCountsCharactersNotBytes(t *testing.T) {
	if _, err := ParseQuery(strings.Repeat("é", 64)); err != nil {
		t.Fatalf("64 символа (128 байт) должны проходить: %v", err)
	}
	if _, err := ParseQuery(strings.Repeat("é", 65)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("65 символов должны отклоняться: %v", err)
	}
}
