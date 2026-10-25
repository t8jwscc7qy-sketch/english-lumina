package httpapi

import (
	"context"
	"errors"
	"slices"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// ListTranslationsForWords отдаёт переводы слов набором (как запрос: по лемме, части речи и номеру значения).
func (f *fakeStore) ListTranslationsForWords(_ context.Context, arg db.ListTranslationsForWordsParams) ([]db.ListTranslationsForWordsRow, error) {
	f.translationCalls++
	if f.failTranslations {
		return nil, errors.New("словарь недоступен")
	}
	var out []db.ListTranslationsForWordsRow
	for _, r := range f.translations {
		if slices.Contains(arg.Lemmas, r.Lemma) {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b db.ListTranslationsForWordsRow) int {
		if a.Lemma != b.Lemma {
			return cmpStr(a.Lemma, b.Lemma)
		}
		if a.Pos != b.Pos {
			return cmpStr(a.Pos, b.Pos)
		}
		return int(a.SenseNo) - int(b.SenseNo)
	})
	return out, nil
}

func cmpStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
