package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
)

// PrintReport печатает отчёт качества: покрытие по уровням, контрольный список и выборку слов для чтения глазами.
func PrintReport(w io.Writer, rep kb.Report) error {
	var b strings.Builder
	b.WriteString("Покрытие слов по уровням (перевод / произношение):\n")
	levels := make([]string, 0, len(rep.Levels))
	for l := range rep.Levels {
		levels = append(levels, l)
	}
	sort.Strings(levels)
	for _, l := range levels {
		s := rep.Levels[l]
		name := l
		if name == "" {
			name = "без уровня"
		}
		ipa := 0.0
		if s.Total > 0 {
			ipa = float64(s.WithIPA) / float64(s.Total) * 100
		}
		fmt.Fprintf(&b, "  %-10s слов %5d, с переводом %5.1f%%, с произношением %5.1f%%\n", name, s.Total, s.Coverage()*100, ipa)
	}
	fmt.Fprintf(&b, "Контрольный список: проверено %d, верно %d\n", rep.Golden.Checked, rep.Golden.Passed)
	for _, f := range rep.Golden.Failed {
		got := f.Got
		if got == "" {
			got = "— нет перевода"
		}
		fmt.Fprintf(&b, "  не совпало: %s (%s) → %s\n", f.Lemma, f.Pos, got)
	}
	fmt.Fprintf(&b, "Выбранных переводов с пометками (устаревшее, грубое, просторечное…): %d\n", rep.Flagged)
	fmt.Fprintf(&b, "Выборка (%d слов):\n", len(rep.Sample))
	for _, s := range rep.Sample {
		ipa := s.IPA
		if ipa == "" {
			ipa = "—"
		}
		fmt.Fprintf(&b, "  %-18s %-12s %-30s %s\n", s.Lemma, s.Pos, strings.Join(s.Translations, ", "), ipa)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
