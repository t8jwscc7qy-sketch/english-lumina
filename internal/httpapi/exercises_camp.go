package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
)

const (
	campMinWords    = 50 // изучено слов, чтобы «Привал» открылся
	campWindowTries = 8  // сколько раз перебирается окно предложений, пока не найдётся подходящее по покрытию
	campTotalTries  = 12 // всего окон на один запрос по всем темам
	campModeParam   = "camp"
	campClosedText  = "Привал откроется, когда ты изучишь больше слов"
	campNoMatchText = "Подходящих упражнений пока нет, добавь слов"
)

// campFilter — режим «Привал» одного запроса: известные формы слов, порог покрытия и сами слова (для упражнений
// по словам).
type campFilter struct {
	known     map[string]bool
	threshold float64
	words     []formInfo
}

// ok — предложение состоит из известных слов не меньше чем на порог.
func (f *campFilter) ok(text string) bool {
	return exercises.CoverageOK(exercises.CasedTokens(text), f.known, f.threshold)
}

type campKey struct{}

func campFrom(ctx context.Context) *campFilter {
	f, _ := ctx.Value(campKey{}).(*campFilter)
	return f
}

// campOK — предложение подходит режиму: вне «Привала» всегда истина.
func campOK(ctx context.Context, text string) bool {
	f := campFrom(ctx)
	return f == nil || f.ok(text)
}

// skipSentence — предложение уже выдавалось или не проходит покрытие «Привала».
func skipSentence(ctx context.Context, used map[int64]bool, id int64, text string) bool {
	return used[id] || !campOK(ctx, text)
}

var errCampClosed = errors.New("привал закрыт")

// loadCamp собирает фильтр «Привала». errCampClosed — изучено меньше campMinWords слов.
func (s *authServer) loadCamp(ctx context.Context, userID int64) (*campFilter, error) {
	rows, err := s.store.LearnKnownWords(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("известные слова: %w", err)
	}
	if len(rows) < campMinWords {
		return nil, errCampClosed
	}
	prefs, err := s.learnPrefs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("предпочтения: %w", err)
	}
	lemmas := make([]exercises.LemmaPos, len(rows))
	ws := make([]formInfo, len(rows))
	for i, r := range rows {
		lemmas[i] = exercises.LemmaPos{Lemma: r.Lemma, Pos: r.Pos}
		ws[i] = formInfo{lexemeID: r.LexemeID, lemma: r.Lemma, pos: r.Pos}
	}
	return &campFilter{known: exercises.KnownForms(lemmas), threshold: float64(prefs.CampCoverage) - 1e-6, words: ws}, nil
}

// campRequested — в строке запроса mode=camp (любое другое значение — обычный режим).
func campRequested(r *http.Request) bool { return r.URL.Query().Get("mode") == campModeParam }

// campWordsFor — известные слова нужной части речи, в случайном порядке (не больше limit): упражнения по словам
// («выбор формы», «сопоставление») в «Привале» берут только их.
func (s *authServer) campWordsFor(f *campFilter, posOK func(string) bool, limit int) []formInfo {
	var out []formInfo
	for _, w := range f.words {
		if posOK(w.pos) {
			out = append(out, w)
		}
	}
	for i := len(out) - 1; i > 0; i-- {
		j := int(s.rnd(int64(i + 1)))
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func writeCampClosed(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, api.ErrorErrorCodeCampClosed, campClosedText)
}
