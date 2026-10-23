package words

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
	best, found := 0, false
	for _, w := range SingleWords(forms) {
		if r, ok := ranks[w]; ok && (!found || r < best) {
			best, found = r, true
		}
	}
	return best, found
}
