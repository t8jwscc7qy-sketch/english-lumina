package words

import (
	"slices"
	"testing"
)

func TestInflectionBases(t *testing.T) {
	cases := map[string]string{
		"found": "find", "went": "go", "was": "be", "were": "be", "been": "be", "had": "have", "did": "do", "done": "do",
		"men": "man", "better": "good", "children": "child", "wrote": "write", "thought": "think",
		"walked": "walk", "liked": "like", "stopped": "stop", "studied": "study", "tries": "try", "boxes": "box", "dogs": "dog",
		"running": "run", "making": "make", "going": "go", "wanted": "want", "teachers": "teacher", "goes": "go",
	}
	for form, lemma := range cases {
		if got := InflectionBases(form); !slices.Contains(got, lemma) {
			t.Errorf("%s: %v, ждали %s", form, got, lemma)
		}
	}
}

func TestInflectionBasesRejectWrongIrregularForms(t *testing.T) {
	for form, bad := range map[string]string{"goed": "go", "finded": "find", "runned": "run", "taked": "take", "buyed": "buy", "eated": "eat"} {
		if got := InflectionBases(form); slices.Contains(got, bad) {
			t.Errorf("%s не должно давать основу %s: %v", form, bad, got)
		}
	}
	if got := InflectionBases("bus"); slices.Contains(got, "bu") {
		t.Logf("bus даёт основу bu (не слово словаря, значит безвредно): %v", got)
	}
}
