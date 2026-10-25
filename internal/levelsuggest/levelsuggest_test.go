package levelsuggest

import (
	"testing"
	"time"
)

var now = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

// answers строит n ответов, из них right верных (верные первыми).
func answers(n, right int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = i < right
	}
	return out
}

func TestSuggestUpAtExactThreshold(t *testing.T) {
	got := Suggest("B1", answers(30, 27), nil, now)
	if got == nil || got.Direction != Up || got.From != "B1" || got.To != "B2" || got.Correct != 27 || got.Total != 30 {
		t.Fatalf("27 из 30 на B1: ждали повышение до B2, получили %+v", got)
	}
}

func TestSuggestNothingJustBelowUpThreshold(t *testing.T) {
	if got := Suggest("B1", answers(30, 26), nil, now); got != nil {
		t.Fatalf("26 из 30: предложения быть не должно, получили %+v", got)
	}
}

func TestSuggestDownAtExactThreshold(t *testing.T) {
	got := Suggest("B1", answers(30, 12), nil, now)
	if got == nil || got.Direction != Down || got.To != "A2" || got.Correct != 12 {
		t.Fatalf("12 из 30 на B1: ждали понижение до A2, получили %+v", got)
	}
}

func TestSuggestNothingJustAboveDownThreshold(t *testing.T) {
	if got := Suggest("B1", answers(30, 13), nil, now); got != nil {
		t.Fatalf("13 из 30: предложения быть не должно, получили %+v", got)
	}
}

func TestSuggestNeedsFullWindow(t *testing.T) {
	if got := Suggest("B1", answers(29, 29), nil, now); got != nil {
		t.Fatalf("29 ответов — мало, получили %+v", got)
	}
	if got := Suggest("B1", nil, nil, now); got != nil {
		t.Fatalf("нет ответов: получили %+v", got)
	}
}

func TestSuggestUsesOnlyLastWindow(t *testing.T) {
	// 20 неверных, затем 30 верных подряд: старое не должно мешать
	old := append(answers(20, 0), answers(30, 30)...)
	got := Suggest("B1", old, nil, now)
	if got == nil || got.Direction != Up || got.Total != 30 || got.Correct != 30 {
		t.Fatalf("считать нужно последние 30: %+v", got)
	}
	// наоборот: 30 верных, потом 30 неверных
	rev := append(answers(30, 30), answers(30, 0)...)
	got = Suggest("B1", rev, nil, now)
	if got == nil || got.Direction != Down || got.Correct != 0 {
		t.Fatalf("последние 30 неверные: %+v", got)
	}
}

func TestSuggestEdgeLevels(t *testing.T) {
	if got := Suggest("C1", answers(30, 30), nil, now); got != nil {
		t.Fatalf("выше C1 некуда: %+v", got)
	}
	if got := Suggest("A1", answers(30, 0), nil, now); got != nil {
		t.Fatalf("ниже A1 некуда: %+v", got)
	}
	if got := Suggest("A1", answers(30, 30), nil, now); got == nil || got.To != "A2" {
		t.Fatalf("с A1 вверх можно: %+v", got)
	}
	if got := Suggest("C1", answers(30, 0), nil, now); got == nil || got.To != "B2" {
		t.Fatalf("с C1 вниз можно: %+v", got)
	}
}

func TestSuggestUnknownLevel(t *testing.T) {
	if got := Suggest("C2", answers(30, 30), nil, now); got != nil {
		t.Fatalf("C2 не на шкале: %+v", got)
	}
	if got := Suggest("", answers(30, 30), nil, now); got != nil {
		t.Fatalf("пустой уровень: %+v", got)
	}
}

func TestSuggestDismissedForFourteenDays(t *testing.T) {
	for _, tc := range []struct {
		ago  time.Duration
		want bool // предложение должно быть показано
	}{
		{time.Hour, false},
		{13*24*time.Hour + 23*time.Hour, false},
		{14 * 24 * time.Hour, true},
		{30 * 24 * time.Hour, true},
	} {
		at := now.Add(-tc.ago)
		got := Suggest("B1", answers(30, 30), &at, now)
		if (got != nil) != tc.want {
			t.Fatalf("отказ %v назад: показано=%v, ждали %v", tc.ago, got != nil, tc.want)
		}
	}
}

func TestSuggestDismissalInFutureStillHides(t *testing.T) {
	at := now.Add(time.Hour) // часы разошлись: отказ «из будущего» не должен выпускать предложение
	if got := Suggest("B1", answers(30, 30), &at, now); got != nil {
		t.Fatalf("отказ из будущего должен скрывать: %+v", got)
	}
}

func TestNextAndPrevLevel(t *testing.T) {
	if l, ok := Shift("B2", 1); !ok || l != "C1" {
		t.Fatalf("после B2 идёт C1: %q %v", l, ok)
	}
	if _, ok := Shift("C1", 1); ok {
		t.Fatal("после C1 уровня нет")
	}
	if _, ok := Shift("A1", -1); ok {
		t.Fatal("до A1 уровня нет")
	}
}
