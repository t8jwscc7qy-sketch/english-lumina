package kb

import (
	"path/filepath"
	"testing"
)

func TestLemmaSetExpandsVariants(t *testing.T) {
	set := LemmaSet([]string{"colour/color", "a.m.", "dad", ""})
	for _, want := range []string{"colour", "color", "a.m.", "dad"} {
		if !set[want] {
			t.Errorf("в наборе нет %q", want)
		}
	}
	if set[""] || len(set) != 4 {
		t.Fatalf("набор: %v", set)
	}
}

func TestLemmasHashIsOrderIndependent(t *testing.T) {
	a := LemmasHash([]string{"b", "a", "c"})
	if a != LemmasHash([]string{"c", "b", "a"}) || a == LemmasHash([]string{"a", "b"}) || a == "" {
		t.Fatal("хеш не зависит от порядка и различает наборы")
	}
}

func TestMetaRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kaikki-en.jsonl.meta")
	in := Meta{FileSHA256: "abc", LemmasHash: "def", Selected: 7}
	if err := WriteMeta(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := ReadMeta(path)
	if err != nil || out != in {
		t.Fatalf("%+v %v", out, err)
	}
	if _, err := ReadMeta(filepath.Join(t.TempDir(), "нет")); err == nil {
		t.Fatal("нет файла — ошибка")
	}
}
