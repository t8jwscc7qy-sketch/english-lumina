package words

import "strings"

// Формы be, have и do: в таблице неправильных глаголов их нет.
var auxiliaryForms = map[string][]string{
	"be":   {"am", "is", "are", "was", "were", "been", "being"},
	"have": {"has", "had", "having"},
	"do":   {"does", "did", "done", "doing"},
}

// formToLemmas — любая неправильная форма (went, found, men, better) → начальные формы.
var formToLemmas = func() map[string][]string {
	m := map[string][]string{}
	add := func(lemma string, forms []string) {
		for _, f := range forms {
			if f != "" {
				m[f] = append(m[f], lemma)
			}
		}
	}
	for l, f := range irregularVerbs {
		add(l, f)
	}
	for l, f := range irregularNouns {
		add(l, f)
	}
	for l, f := range irregularAdjectives {
		add(l, f)
	}
	for l, f := range auxiliaryForms {
		add(l, f)
	}
	return m
}()

func isConsonant(b byte) bool { return b >= 'a' && b <= 'z' && !strings.ContainsRune("aeiou", rune(b)) }

// InflectionBases возвращает возможные начальные формы слова: для неправильных слов по таблицам (found → find,
// went → go, was → be), для правильных — снятием окончаний -s, -es, -ies, -ed, -ied, -ing (с возвратом -e и удвоенной
// согласной). Правильное окончание -ed неправильному глаголу не приписывается (goed → go не возвращается): неверные
// формы вроде «goed» не должны проходить как слова словаря. Начальные формы — только кандидаты: какая из них настоящая,
// решает словарь вызывающего.
func InflectionBases(word string) []string {
	w := strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	out := append([]string(nil), formToLemmas[w]...)
	add := func(b string) {
		if len(b) >= 2 {
			out = append(out, b)
		}
	}
	// -ed допустимо только у правильных глаголов.
	regularBase := func(b string) {
		if _, irregular := irregularVerbs[b]; !irregular {
			add(b)
		}
	}
	switch {
	case strings.HasSuffix(w, "ies"):
		add(w[:len(w)-3] + "y")
	case strings.HasSuffix(w, "es"):
		add(w[:len(w)-2])
		add(w[:len(w)-1])
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		add(w[:len(w)-1])
	}
	switch {
	case strings.HasSuffix(w, "ied"):
		regularBase(w[:len(w)-3] + "y")
	case strings.HasSuffix(w, "ed"):
		regularBase(w[:len(w)-2])
		regularBase(w[:len(w)-1])
		if n := len(w); n > 4 && w[n-3] == w[n-4] && isConsonant(w[n-3]) {
			regularBase(w[:n-3])
		}
	}
	if strings.HasSuffix(w, "ing") {
		b := w[:len(w)-3]
		add(b)
		add(b + "e")
		if n := len(b); n > 2 && b[n-1] == b[n-2] && isConsonant(b[n-1]) {
			add(b[:n-1])
		}
	}
	return out
}
