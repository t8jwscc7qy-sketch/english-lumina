package importer

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

type stubSource struct{ name string }

func (s stubSource) Describe() Info                                        { return Info{Name: s.name, Version: "1"} }
func (stubSource) Fetch(context.Context, string) (string, error)           { return "", nil }
func (stubSource) Parse(context.Context, io.Reader, func(any) error) error { return nil }
func (stubSource) Load(context.Context, *db.Queries, int64, any) error     { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(stubSource{"zeta"})
	r.Register(stubSource{"alpha"})

	if got := r.Names(); !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Fatalf("Names = %v, ожидали по алфавиту", got)
	}
	if s, ok := r.Get("alpha"); !ok || s.Describe().Name != "alpha" {
		t.Fatalf("Get(alpha) = %v, %v", s, ok)
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("Get неизвестного имени вернул true")
	}
}

func TestRegistryDuplicateNamePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(stubSource{"a"})
	defer func() {
		if recover() == nil {
			t.Fatal("повторное имя должно вызывать панику")
		}
	}()
	r.Register(stubSource{"a"})
}
