// Package learn — чистая логика «Знакомства»: подбор блока слов для урока.
package learn

import "unicode/utf8"

// Ограничения блока: слов одной части речи и минимальная длина слов, для которых проверяется «близкое написание».
const (
	maxPerPos     = 3
	minNearLength = 3
)

// Candidate — слово-кандидат для блока; Revisit — повторное знакомство (трудное слово из карточек).
type Candidate struct {
	ID      int64
	Lemma   string
	Pos     string
	Revisit bool
}

// PickBlock выбирает до n слов из упорядоченного списка кандидатов (повторное знакомство уже стоит первым).
// Слово берётся, если: не больше maxPerPos слов его части речи, нет связи related (синоним или антоним, в любую
// сторону) ни с одним взятым, написание не отличается от взятого одной правкой (quite/quiet, house/horse) и ok (у слова
// есть пример: дорогая проверка) истинно. После заполнения блока слова не проверяются.
func PickBlock(cands []Candidate, n int, ok func(Candidate) bool, related func(a, b int64) bool) []Candidate {
	if n <= 0 {
		return nil
	}
	var picked []Candidate
	perPos := map[string]int{}
	for _, c := range cands {
		if len(picked) >= n {
			break
		}
		if perPos[c.Pos] >= maxPerPos || clashes(c, picked, related) || !ok(c) {
			continue
		}
		picked = append(picked, c)
		perPos[c.Pos]++
	}
	return picked
}

func clashes(c Candidate, picked []Candidate, related func(a, b int64) bool) bool {
	for _, p := range picked {
		if related(c.ID, p.ID) || related(p.ID, c.ID) || nearSpelling(c.Lemma, p.Lemma) {
			return true
		}
	}
	return false
}

// nearSpelling: слова отличаются одной правкой (замена, вставка, удаление, перестановка соседних букв); слова короче
// minNearLength букв не сравниваются (is/it, to/so отличаются «одной буквой» всегда).
func nearSpelling(a, b string) bool {
	if utf8.RuneCountInString(a) < minNearLength || utf8.RuneCountInString(b) < minNearLength || a == b {
		return false
	}
	x, y := []rune(a), []rune(b)
	if len(x) < len(y) {
		x, y = y, x
	}
	switch len(x) - len(y) {
	case 0:
		var diff []int
		for i := range x {
			if x[i] != y[i] {
				diff = append(diff, i)
			}
		}
		switch len(diff) {
		case 1:
			return true
		case 2:
			i := diff[0]
			return diff[1] == i+1 && x[i] == y[i+1] && x[i+1] == y[i]
		}
		return false
	case 1:
		i := 0
		for i < len(y) && x[i] == y[i] {
			i++
		}
		return string(x[i+1:]) == string(y[i:])
	}
	return false
}
