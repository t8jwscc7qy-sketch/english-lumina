package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/library"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/texts"
)

// Окно выбора авторского текста: покрытие пользователя в пределах цели ± authoredWindow; если таких нет, берётся ближайший
// к цели в границах [authoredMinCoverage, authoredMaxCoverage].
const (
	authoredWindow      = 0.05
	authoredMinCoverage = 0.70
	authoredMaxCoverage = 0.97
)

// authoredEntry — авторский текст в памяти: номер в базе и слова без имён героев (для подсчёта понимания).
type authoredEntry struct {
	dbID   int64
	text   library.Text
	tokens [][]string
}

// withoutNames убирает из слов предложения имена героев (и их притяжательные формы): они всегда понятны.
func withoutNames(tokens []string, names []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		base := strings.TrimSuffix(strings.TrimSuffix(t, "'s"), "’s")
		if slices.Contains(names, t) || slices.Contains(names, base) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// authoredTexts возвращает встроенные тексты и записывает их в базу (идемпотентно, при первом обращении). Ошибка записи
// не роняет выдачу: авторских текстов просто нет, работает склейка.
func (s *authServer) authoredTexts(ctx context.Context) []authoredEntry {
	s.authoredMu.Lock()
	defer s.authoredMu.Unlock()
	if s.authoredLoaded {
		return s.authored
	}
	all, err := library.LoadAll(content.Texts)
	if err != nil {
		slog.Error("авторские тексты не загружены", "err", err)
		s.authoredLoaded = true
		return nil
	}
	entries := make([]authoredEntry, 0, len(all))
	for _, t := range all {
		body, err := json.Marshal(t)
		if err != nil {
			slog.Error("авторский текст не записан", "id", t.ID, "err", err)
			return nil
		}
		id, err := s.store.TextUpsertAuthored(ctx, db.TextUpsertAuthoredParams{
			Slug: &t.ID, Band: ptrInt16(int16(t.Band)), Title: t.Title, Body: body, WordCount: int16(min(t.WordCount(), 32000)), //nolint:gosec // ограничено
		})
		if err != nil {
			slog.Error("авторский текст не записан", "id", t.ID, "err", err)
			return nil // повторная попытка при следующем запросе
		}
		tokens := make([][]string, len(t.Sentences))
		for i, sn := range t.Sentences {
			tokens[i] = withoutNames(sn.Tokens(), t.Names)
		}
		entries = append(entries, authoredEntry{dbID: id, text: t, tokens: tokens})
	}
	s.authored, s.authoredLoaded = entries, true
	return entries
}

func (e authoredEntry) coverage(known map[string]bool) float64 {
	return texts.Understanding(e.tokens, known)
}

// pickAuthored выбирает непрочитанный авторский текст с понятностью, ближайшей к цели (в окне цели — случайный из них).
func (s *authServer) pickAuthored(entries []authoredEntry, read map[int64]bool, known map[string]bool, goal float64) (authoredEntry, float64, bool) {
	type cand struct {
		e   authoredEntry
		cov float64
	}
	var inWindow, others []cand
	for _, e := range entries {
		if read[e.dbID] {
			continue
		}
		c := cand{e: e, cov: e.coverage(known)}
		switch {
		case math.Abs(c.cov-goal) <= authoredWindow+1e-9:
			inWindow = append(inWindow, c)
		case c.cov >= authoredMinCoverage && c.cov <= authoredMaxCoverage:
			others = append(others, c)
		}
	}
	if len(inWindow) > 0 {
		c := inWindow[s.rnd(int64(len(inWindow)))]
		return c.e, c.cov, true
	}
	if len(others) == 0 {
		return authoredEntry{}, 0, false
	}
	best := others[0]
	for _, c := range others[1:] {
		if math.Abs(c.cov-goal) < math.Abs(best.cov-goal) {
			best = c
		}
	}
	return best.e, best.cov, true
}

// authoredResponse собирает ответ по авторскому тексту: понимание и подсказки считаются для этого пользователя.
func (s *authServer) authoredResponse(ctx context.Context, e authoredEntry, mode api.ReadTextMode, goal float64, known map[string]bool) (api.ReadText, error) {
	hints, err := s.authoredHints(ctx, e, known)
	if err != nil {
		return api.ReadText{}, err
	}
	out := api.ReadText{
		Id: e.dbID, Mode: mode, Title: e.text.Title, Goal: float32(goal), Coverage: float32(e.coverage(known)), Cohesion: 1,
		Targets: make([]string, len(e.text.Targets)), Hints: hints, Sentences: make([]api.ReadSentence, len(e.text.Sentences)),
	}
	sentences := make([]storedSentence, len(e.text.Sentences))
	for i, t := range e.text.Targets {
		out.Targets[i] = t.Lemma
	}
	for i, sn := range e.text.Sentences {
		out.Sentences[i] = api.ReadSentence{En: sn.En, Ru: sn.Ru, Tokens: sn.Tokens()}
		sentences[i] = storedSentence{En: sn.En, Tokens: sn.Tokens()}
	}
	out.Constructions = constructionsOf(sentences)
	return out, nil
}

// authoredHints — подсказки к целевым словам текста (все их формы) и к неизвестным словам, совпадающим с леммой.
func (s *authServer) authoredHints(ctx context.Context, e authoredEntry, known map[string]bool) ([]api.ReadWordHint, error) {
	formOf := map[string]library.Target{}
	var lemmas []string
	for _, t := range e.text.Targets {
		lemmas = append(lemmas, t.Lemma)
		for f := range t.Forms() {
			formOf[f] = t
		}
	}
	unknown := map[string]bool{}
	var other []string
	for _, toks := range e.tokens {
		for _, tok := range exercises.UnknownTokens(toks, known) {
			if unknown[tok] {
				continue
			}
			unknown[tok] = true
			if _, isTarget := formOf[tok]; !isTarget {
				other = append(other, tok)
			}
		}
	}
	rows, err := s.store.TextWordHints(ctx, append(slices.Clone(lemmas), other...))
	if err != nil {
		return nil, fmt.Errorf("подсказки: %w", err)
	}
	byLemma := map[string]db.TextWordHintsRow{}
	for _, r := range rows { // у леммы с несколькими частями речи первым идёт самое частое слово
		if _, ok := byLemma[r.Lemma]; !ok {
			byLemma[r.Lemma] = r
		}
	}
	for _, t := range e.text.Targets { // подходящая часть речи важнее частоты
		for _, r := range rows {
			if r.Lemma == t.Lemma && r.Pos == t.Pos {
				byLemma[t.Lemma] = r
				break
			}
		}
	}
	hints := []api.ReadWordHint{}
	for tok := range unknown {
		var lemma string
		isTarget := false
		if t, ok := formOf[tok]; ok {
			lemma, isTarget = t.Lemma, true
		} else {
			lemma = tok
		}
		r, ok := byLemma[lemma]
		if !ok {
			continue
		}
		card, ok := parseCard(r.Payload)
		if !ok {
			continue
		}
		hints = append(hints, hintOf(tok, r.LexemeID, r.Lemma, r.Pos, card, isTarget))
	}
	slices.SortFunc(hints, func(a, b api.ReadWordHint) int { return strings.Compare(a.Token, b.Token) })
	return hints, nil
}

// entryByID находит авторский текст по номеру в базе.
func entryByID(entries []authoredEntry, id int64) (authoredEntry, bool) {
	for _, e := range entries {
		if e.dbID == id {
			return e, true
		}
	}
	return authoredEntry{}, false
}

func ptrInt16(v int16) *int16 { return &v }
