package library

import (
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// BandWord — слово словаря уровня: лемма и часть речи.
type BandWord struct{ Lemma, Pos string }

// AllowedForms строит множество допустимых словоформ (в нижнем регистре) по словам словаря уровня. «Служебные»
// сокращения (n't, 's, 'm…) проверяются отдельно по основе.
func AllowedForms(ws []BandWord) map[string]bool {
	out := make(map[string]bool, len(ws)*4)
	for _, w := range ws {
		lemma := strings.ToLower(w.Lemma)
		out[lemma] = true
		for _, f := range words.SingleWords(words.Forms(lemma, w.Pos)) {
			out[f] = true
		}
	}
	return out
}
