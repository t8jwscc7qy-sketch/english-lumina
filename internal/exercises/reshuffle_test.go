package exercises

import (
	"slices"
	"testing"
)

func TestReshuffleSameElements(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e"}
	orig := slices.Clone(in)
	out, ok := Reshuffle(in, nil, 1)
	if !ok {
		t.Fatal("ok = false")
	}
	if !slices.Equal(in, orig) {
		t.Fatal("входной срез изменён")
	}
	a, b := slices.Clone(out), slices.Clone(in)
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		t.Fatalf("состав изменился: %v", out)
	}
}

func TestReshuffleDeterministic(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f"}
	x, _ := Reshuffle(in, nil, 42)
	y, _ := Reshuffle(in, nil, 42)
	if !slices.Equal(x, y) {
		t.Fatalf("%v != %v", x, y)
	}
}

func TestReshuffleAvoidsForbidden(t *testing.T) {
	in := []string{"a", "b", "c", "d"}
	for seed := range int64(50) {
		out, ok := Reshuffle(in, func(o []string) bool { return slices.Equal(o, in) }, seed)
		if ok && slices.Equal(out, in) {
			t.Fatalf("seed %d: вернулся запрещённый порядок", seed)
		}
	}
	if _, ok := Reshuffle(in, func([]string) bool { return true }, 1); ok {
		t.Fatal("все порядки запрещены, но ok = true")
	}
}

func TestReshuffleNilForbidden(t *testing.T) {
	if _, ok := Reshuffle([]string{"a", "b", "c"}, nil, 7); !ok {
		t.Fatal("ok = false при forbidden = nil")
	}
}
