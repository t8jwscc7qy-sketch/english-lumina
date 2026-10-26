package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
)

// Путь считается при старте пакета: тесты не должны зависеть от рабочего каталога.
var samplePath, _ = filepath.Abs("../../internal/kb/testdata/kaikki-sample.jsonl")

func gz(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sample(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPrepareFiltersByLanguageAndLemma(t *testing.T) {
	var out bytes.Buffer
	st, err := Prepare(context.Background(), bytes.NewReader(gz(t, sample(t))), map[string]bool{"dad": true, "clothes": true}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if st.Lines != 10 || st.English != 9 || st.Selected != 2 {
		t.Fatalf("счётчики: %+v", st)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"dad"`) || !strings.Contains(lines[1], `"clothes"`) {
		t.Fatalf("отобрано: %q", out.String())
	}
}

func TestPrepareLongLine(t *testing.T) {
	long := `{"word":"dad","pos":"noun","lang_code":"en","senses":[{"glosses":["` + strings.Repeat("x", 5<<20) + `"]}]}` + "\n"
	var out bytes.Buffer
	st, err := Prepare(context.Background(), bytes.NewReader(gz(t, []byte(long))), map[string]bool{"dad": true}, &out)
	if err != nil || st.Selected != 1 || out.Len() < 5<<20 {
		t.Fatalf("длинная строка: %+v %v (%d байт)", st, err, out.Len())
	}
}

func TestPrepareStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Prepare(ctx, bytes.NewReader(gz(t, sample(t))), map[string]bool{"dad": true}, &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка %v, ждали context.Canceled", err)
	}
}

func TestPrepareCorruptGzipIsError(t *testing.T) {
	data := gz(t, sample(t))
	if _, err := Prepare(context.Background(), bytes.NewReader(data[:len(data)/2]), map[string]bool{"dad": true}, &bytes.Buffer{}); err == nil {
		t.Fatal("оборванный архив должен давать ошибку")
	}
	if _, err := Prepare(context.Background(), strings.NewReader("не gzip"), map[string]bool{"dad": true}, &bytes.Buffer{}); err == nil {
		t.Fatal("не gzip должен давать ошибку")
	}
}

func TestPrepareSelectsVariantsOfLemma(t *testing.T) {
	var out bytes.Buffer
	st, err := Prepare(context.Background(), bytes.NewReader(gz(t, sample(t))), kb.LemmaSet([]string{"colour/color"}), &out)
	if err != nil || st.Selected != 2 {
		t.Fatalf("обе записи colour и color должны отобраться: %+v %v", st, err)
	}
}
