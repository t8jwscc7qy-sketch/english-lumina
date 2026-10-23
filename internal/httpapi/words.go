package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const maxWordResults = 50

// SearchWords ищет слово по началу (и по началу варианта через слэш). Запрос q читаем сами:
// объявленный в контракте параметр заставил бы генератор добавить новую зависимость.
func (s *authServer) SearchWords(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	query, err := words.ParseQuery(r.URL.Query().Get("q"))
	switch {
	case errors.Is(err, words.ErrEmpty):
		writeJSON(w, http.StatusOK, api.WordList{Words: []api.Word{}})
		return
	case errors.Is(err, words.ErrTooLong):
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Запрос длиннее 64 символов")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "В запросе недопустимые символы")
		return
	}

	ctx := r.Context()
	found, err := s.store.SearchLexemes(ctx, db.SearchLexemesParams{
		Prefix: query.Prefix, Variant: query.Variant, Exact: query.Exact, MaxRows: maxWordResults,
	})
	if err != nil {
		s.failWords(w, err)
		return
	}
	out := api.WordList{Words: []api.Word{}} // пустой список должен быть [], а не null
	if len(found) == 0 {
		writeJSON(w, http.StatusOK, out)
		return
	}

	ids := make([]int64, len(found))
	for i, f := range found {
		ids[i] = f.ID
	}
	levels, err := s.store.ListLexemeLevels(ctx, ids)
	if err != nil {
		s.failWords(w, err)
		return
	}
	byWord := map[int64][]api.WordLevel{}
	for _, l := range levels {
		byWord[l.LexemeID] = append(byWord[l.LexemeID], api.WordLevel{
			Source: l.Source, Level: api.CefrLevel(l.Level), Notes: notesOf(l.Extra),
		})
	}
	for _, f := range found {
		out.Words = append(out.Words, api.Word{
			Lemma: f.Lemma, Pos: f.Pos, Level: api.CefrLevel(f.Level), Levels: append([]api.WordLevel{}, byWord[f.ID]...),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// notesOf достаёт заметки источника из extra; битый или чужой JSON даёт пустой список, а не ошибку.
func notesOf(extra json.RawMessage) []string {
	var x struct {
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(extra, &x); err != nil || x.Notes == nil {
		return []string{}
	}
	return x.Notes
}

func (s *authServer) failWords(w http.ResponseWriter, err error) {
	slog.Error("поиск слов", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}
