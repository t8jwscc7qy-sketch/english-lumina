package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
)

type fakeSource struct{ name, version string }

func (f fakeSource) Describe() importer.Info {
	return importer.Info{Name: f.name, Version: f.version}
}
func (fakeSource) Fetch(context.Context, string) (string, error)           { return "", nil }
func (fakeSource) Parse(context.Context, io.Reader, func(any) error) error { return nil }
func (fakeSource) Load(context.Context, *db.Queries, int64, any) error     { return nil }

func registry() *importer.Registry {
	r := importer.NewRegistry()
	r.Register(fakeSource{"zeta", "2"})
	r.Register(fakeSource{"alpha", "1"})
	return r
}

type call struct {
	name     string
	cacheDir string
}

func runWith(t *testing.T, args []string, result importer.Result, runErr error) (string, []call, error) {
	t.Helper()
	var calls []call
	var out bytes.Buffer
	err := run(context.Background(), args, &out, registry(), func(_ context.Context, src importer.Source, cacheDir string) (importer.Result, error) {
		calls = append(calls, call{src.Describe().Name, cacheDir})
		return result, runErr
	})
	return out.String(), calls, err
}

func TestListPrintsNamesSorted(t *testing.T) {
	out, calls, err := runWith(t, []string{"list"}, importer.Result{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "alpha\nzeta\n" {
		t.Fatalf("вывод %q", out)
	}
	if len(calls) != 0 {
		t.Fatal("list не должен запускать импорт")
	}
}

func TestUnknownSourceListsAvailable(t *testing.T) {
	_, calls, err := runWith(t, []string{"nope"}, importer.Result{}, nil)
	if err == nil || !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "alpha, zeta") {
		t.Fatalf("ошибка %v", err)
	}
	if len(calls) != 0 {
		t.Fatal("импорт неизвестного источника запущен")
	}
}

func TestWrongArgumentCountShowsUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"alpha", "zeta"}} {
		_, calls, err := runWith(t, args, importer.Result{}, nil)
		if err == nil || !strings.Contains(err.Error(), "использование") {
			t.Fatalf("args=%v: ошибка %v", args, err)
		}
		if len(calls) != 0 {
			t.Fatal("импорт запущен при неверных аргументах")
		}
	}
}

func TestRunPrintsSummary(t *testing.T) {
	res := importer.Result{Info: importer.Info{Name: "alpha", Version: "1"}, Rows: 7799, Duration: 1500 * time.Millisecond}
	out, calls, err := runWith(t, []string{"alpha"}, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha", "1", "7799", "1.5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("в выводе %q нет %q", out, want)
		}
	}
	if len(calls) != 1 || calls[0] != (call{"alpha", ".cache"}) {
		t.Fatalf("вызовы: %+v", calls)
	}
}

func TestSourceErrorIsReturned(t *testing.T) {
	boom := errors.New("импорт alpha: сломалось")
	_, _, err := runWith(t, []string{"alpha"}, importer.Result{}, boom)
	if !errors.Is(err, boom) {
		t.Fatalf("ошибка %v", err)
	}
}
