package words

import "strings"

// SingleWords возвращает однословные формы (по порядку, без повторов): именно они ищутся в списке частот.
// Фразы пропускаются — в списке только отдельные слова.
func SingleWords(forms [][]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range forms {
		if len(f) != 1 || seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		out = append(out, f[0])
	}
	return out
}

// BestRank возвращает лучшее (наименьшее) место среди форм слова в списке частот. Слова словаря начинаются
// с лемм, а в списке стоят написания (run, runs, running), поэтому частота леммы — это самая частая из её форм.
// Нет ни одной формы в списке или это фраза — false.
func BestRank(forms [][]string, ranks map[string]int) (int, bool) {
	return BestRankOf(SingleWords(forms), ranks)
}

// WholeVariants возвращает написания леммы целиком, в нижнем регистре, без пробелов внутри: good-looking, a.m.
// Forms режет слово по знакам препинания, поэтому такие слова иначе не находятся в списке частот.
func WholeVariants(lemma string) []string {
	var out []string
	for _, v := range strings.Split(lemma, "/") {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" && !strings.ContainsAny(v, " \t") {
			out = append(out, v)
		}
	}
	return out
}

// FrequencyKeys — всё, что ищется в списке частот для слова: однословные формы и написания леммы целиком.
func FrequencyKeys(lemma string, forms [][]string) []string {
	return append(SingleWords(forms), WholeVariants(lemma)...)
}

// BestRankOf возвращает наименьшее место среди ключей, найденных в списке.
func BestRankOf(keys []string, ranks map[string]int) (int, bool) {
	best, found := 0, false
	for _, w := range keys {
		if r, ok := ranks[w]; ok && (!found || r < best) {
			best, found = r, true
		}
	}
	return best, found
}
