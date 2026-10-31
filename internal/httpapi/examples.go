package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const (
	maxExamples = 5
	maxPOSLen   = 32 // как CHECK в lexemes.pos
)

// ListWordExamples отдаёт примеры предложений для слова. Параметры lemma и pos читаем из URL сами:
// объявленные в контракте они заставили бы генератор добавить новую зависимость.
func (s *authServer) ListWordExamples(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	lemma := strings.TrimSpace(r.URL.Query().Get("lemma"))
	pos := strings.TrimSpace(r.URL.Query().Get("pos"))
	if lemma == "" || !validText(lemma, words.MaxQueryLen) || !validText(pos, maxPOSLen) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны параметры lemma (до 64 символов) и pos (до 32 символов)")
		return
	}

	out := api.ExampleList{Examples: []api.Example{}} // пустой список должен быть [], а не null
	forms := words.Forms(lemma, pos)
	if len(forms) == 0 { // в лемме нет слов (одни знаки): искать нечего
		writeJSON(w, http.StatusOK, out)
		return
	}
	rows, err := s.store.ListExamples(r.Context(), db.ListExamplesParams{Query: words.TSQuery(forms), MaxRows: maxExamples})
	if err != nil {
		slog.Error("примеры слова", "err", err)
		writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
		return
	}
	for _, row := range rows {
		ex := api.Example{
			En: api.ExampleSentence{Text: row.EnText, Author: row.EnAuthor, TatoebaId: row.EnTatoebaID},
			Ru: api.ExampleSentence{Text: row.RuText, Author: row.RuAuthor, TatoebaId: row.RuTatoebaID},
		}
		if start, end, ok := words.FindMatch(row.EnText, forms); ok {
			ex.Highlight = &api.TextRange{Start: start, End: end}
		}
		out.Examples = append(out.Examples, ex)
	}
	attachExampleAudio(r.Context(), s.store, out.Examples)
	writeJSON(w, http.StatusOK, out)
}

// validText: корректный UTF-8, без нулевого байта (PostgreSQL его не принимает), не длиннее maxRunes символов.
func validText(s string, maxRunes int) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) <= maxRunes
}
