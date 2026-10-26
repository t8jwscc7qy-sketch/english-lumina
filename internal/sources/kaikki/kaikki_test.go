package kaikki

import (
	"strings"
	"testing"
)

func TestDescribe(t *testing.T) {
	info := Describe()
	if info.Name != "kaikki-wiktextract" || info.Version == "" {
		t.Fatalf("имя и версия: %+v", info)
	}
	if !strings.Contains(info.License, "CC BY-SA") {
		t.Fatalf("лицензия: %q", info.License)
	}
	for _, want := range []string{"Wiktionary", "Wiktextract", "Ylonen", "kaikki.org"} {
		if !strings.Contains(info.Attribution, want) {
			t.Fatalf("в атрибуции %q нет %q", info.Attribution, want)
		}
	}
	if !strings.HasPrefix(info.URL, "https://kaikki.org/") {
		t.Fatalf("URL: %s", info.URL)
	}
}
