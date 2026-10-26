package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/constructions"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/texts"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const (
	textMinKnown       = 20 // известных слов, с которых имеет смысл собирать текст
	textTargetPool     = 40 // сколько следующих по частоте слов смотрим
	textCandidateWords = 18 // сколько слов с подходящими предложениями собираем для склейки
	textExampleRows    = 30 // предложений на слово
	textRecentTexts    = 5  // из скольких последних текстов не повторяем предложения
	maxTextBody        = 1 << 10
	defaultTextGoal    = 0.85
)

const noTextMessage = "Пока не из чего собрать текст. Сначала познакомься со словами или пройди быстрый тест"

func (s *authServer) failTexts(w http.ResponseWriter, err error) {
	slog.Error("тексты", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

func writeNoText(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, api.ErrorErrorCodeNoText, noTextMessage)
}

// roundGoal — цель понимания с точностью до сотых: в базе она лежит как real.
func roundGoal(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

func (s *authServer) textGoal(ctx context.Context, userID int64) (float64, error) {
	p, err := s.learnPrefs(ctx, userID)
	if err != nil {
		return 0, err
	}
	if p.TextCoverage == 0 { // строка настроек по умолчанию без поля
		return defaultTextGoal, nil
	}
	return roundGoal(float64(p.TextCoverage)), nil
}

type storedSentence struct {
	ID          int64    `json:"id"`
	TatoebaID   int64    `json:"tatoebaId"`
	Author      string   `json:"author"`
	RuTatoebaID int64    `json:"ruTatoebaId"`
	RuAuthor    string   `json:"ruAuthor"`
	En          string   `json:"en"`
	Ru          string   `json:"ru"`
	Tokens      []string `json:"tokens"`
}

// storedText — тело сохранённого текста: по нему повторно отдаётся ещё не оценённый текст.
type storedText struct {
	Sentences []storedSentence   `json:"sentences"`
	Targets   []string           `json:"targets"`
	Hints     []api.ReadWordHint `json:"hints"`
	Cohesion  float64            `json:"cohesion"`
}

// constructionInfos — пояснения к конструкциям (встроены в сервер, проверяются тестом `go test ./content`).
var constructionInfos = sync.OnceValue(func() map[string]constructions.Info {
	infos, err := constructions.LoadAll(content.Constructions)
	if err != nil {
		slog.Error("пояснения к конструкциям не загружены", "err", err)
		return nil
	}
	return infos
})

// constructionsOf находит конструкции в предложениях текста и подкладывает пояснения; без пояснения конструкция не показывается.
func constructionsOf(sentences []storedSentence) []api.ReadConstruction {
	in := make([]constructions.Sentence, len(sentences))
	for i, s := range sentences {
		in[i] = constructions.Sentence{Text: s.En, Tokens: s.Tokens}
	}
	out := []api.ReadConstruction{}
	infos := constructionInfos()
	for _, m := range constructions.Detect(in) {
		info, ok := infos[m.ID]
		if !ok {
			continue
		}
		c := api.ReadConstruction{Id: info.ID, Title: info.Title, Form: info.Form, Explanation: info.Explanation, ExampleEn: info.ExampleEn, ExampleRu: info.ExampleRu, Sentences: m.Sentences}
		if info.SheetID != "" {
			sheet := info.SheetID
			c.SheetId = &sheet
		}
		out = append(out, c)
	}
	return out
}

func readTextOf(id int64, mode api.ReadTextMode, title string, goal, coverage float64, body storedText) api.ReadText {
	out := api.ReadText{
		Id: id, Mode: mode, Title: title, Goal: float32(goal), Coverage: float32(coverage), Cohesion: float32(body.Cohesion),
		Targets: body.Targets, Hints: body.Hints, Sentences: make([]api.ReadSentence, len(body.Sentences)),
	}
	out.Constructions = constructionsOf(body.Sentences)
	if out.Hints == nil {
		out.Hints = []api.ReadWordHint{}
	}
	if out.Targets == nil {
		out.Targets = []string{}
	}
	for i, st := range body.Sentences {
		out.Sentences[i] = api.ReadSentence{TatoebaId: st.TatoebaID, Author: st.Author, RuTatoebaId: st.RuTatoebaID, RuAuthor: st.RuAuthor, En: st.En, Ru: st.Ru, Tokens: st.Tokens}
	}
	return out
}

// GetNextText собирает и отдаёт текст на целевом уровне понимания.
func (s *authServer) GetNextText(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	mode := api.ReadTextModeRead
	if r.URL.Query().Get("mode") == "listen" {
		mode = api.ReadTextModeListen
	}
	ctx := r.Context()
	// Пока предыдущий текст не оценён, отдаём его же: возврат на страницу и смена режима не теряют текст.
	if r.URL.Query().Get("new") != "1" {
		last, err := s.store.TextLastUnrated(ctx, &user.ID)
		switch {
		case err == nil:
			var body storedText
			if json.Unmarshal(last.Body, &body) == nil && len(body.Sentences) > 0 {
				writeJSON(w, http.StatusOK, readTextOf(last.ID, mode, last.Title, float64(last.Goal), float64(last.CoverageAtBuild), body))
				return
			}
		case !errors.Is(err, pgx.ErrNoRows):
			s.failTexts(w, fmt.Errorf("последний текст: %w", err))
			return
		}
	}
	knownRows, err := s.store.LearnKnownWords(ctx, user.ID)
	if err != nil {
		s.failTexts(w, fmt.Errorf("известные слова: %w", err))
		return
	}
	if len(knownRows) < textMinKnown {
		writeNoText(w)
		return
	}
	goal, err := s.textGoal(ctx, user.ID)
	if err != nil {
		s.failTexts(w, fmt.Errorf("цель понимания: %w", err))
		return
	}
	lemmas := make([]exercises.LemmaPos, len(knownRows))
	for i, k := range knownRows {
		lemmas[i] = exercises.LemmaPos{Lemma: k.Lemma, Pos: k.Pos}
	}
	known := exercises.KnownForms(lemmas)

	recent, err := s.store.TextRecentSentenceIDs(ctx, db.TextRecentSentenceIDsParams{UserID: &user.ID, MaxTexts: textRecentTexts})
	if err != nil {
		s.failTexts(w, fmt.Errorf("недавние предложения: %w", err))
		return
	}
	skip := make(map[int64]bool, len(recent))
	for _, id := range recent {
		skip[id] = true
	}
	targets, err := s.store.TextTargetWords(ctx, db.TextTargetWordsParams{UserID: user.ID, MaxRows: textTargetPool})
	if err != nil {
		s.failTexts(w, fmt.Errorf("целевые слова: %w", err))
		return
	}

	var cands []texts.Candidate
	targetRow := map[string]db.TextTargetWordsRow{} // лемма → строка
	byEnID := map[int64]db.ListExamplesRow{}
	for _, t := range targets {
		if len(cands) >= textCandidateWords {
			break
		}
		// Формы, которые уже известны (то же слово другой частью речи, совпавшая форма другого слова), целевыми не бывают.
		var forms []string
		for _, f := range words.SingleWords(words.Forms(t.Lemma, t.Pos)) {
			if !known[f] && !exercises.IsFunctionWord(f) {
				forms = append(forms, f)
			}
		}
		_, dup := targetRow[t.Lemma]
		if dup || len(forms) == 0 || known[strings.ToLower(t.Lemma)] || exercises.IsFunctionWord(t.Lemma) { // одна лемма — одно целевое слово
			continue
		}
		rows, err := s.store.ListExamples(ctx, db.ListExamplesParams{Query: words.TSQuery(words.Forms(t.Lemma, t.Pos)), MaxRows: textExampleRows})
		if err != nil {
			s.failTexts(w, fmt.Errorf("примеры: %w", err))
			return
		}
		c := texts.Candidate{Target: t.Lemma, Forms: map[string]bool{}}
		for _, f := range forms {
			c.Forms[f] = true
		}
		for _, row := range rows {
			if skip[row.EnID] {
				continue
			}
			byEnID[row.EnID] = row
			c.Sentences = append(c.Sentences, texts.Sentence{ID: row.EnID, En: row.EnText, Ru: row.RuText, Tokens: exercises.CasedTokens(row.EnText)})
		}
		if len(c.Sentences) < 2 {
			continue
		}
		cands = append(cands, c)
		targetRow[t.Lemma] = t
	}
	text, ok := texts.Stitch(known, cands, texts.Options{Goal: goal, Seed: s.rnd(1 << 62)})
	if !ok {
		writeNoText(w)
		return
	}

	sentences := make([]storedSentence, len(text.Sentences))
	tokens := make([][]string, len(text.Sentences))
	wordCount := 0
	for i, sn := range text.Sentences {
		row := byEnID[sn.ID]
		sentences[i] = storedSentence{ID: sn.ID, TatoebaID: row.EnTatoebaID, Author: row.EnAuthor, RuTatoebaID: row.RuTatoebaID, RuAuthor: row.RuAuthor, En: sn.En, Ru: sn.Ru, Tokens: sn.Tokens}
		tokens[i] = sn.Tokens
		wordCount += len(sn.Tokens)
	}
	hints, err := s.textHints(ctx, text, cands, targetRow, known)
	if err != nil {
		s.failTexts(w, err)
		return
	}
	title := "Новые слова: " + strings.Join(text.Targets[:min(3, len(text.Targets))], ", ")
	stored := storedText{Sentences: sentences, Targets: text.Targets, Hints: hints, Cohesion: text.Cohesion}
	body, err := json.Marshal(stored)
	if err != nil {
		s.failTexts(w, err)
		return
	}
	id, err := s.store.TextInsert(ctx, db.TextInsertParams{
		UserID: &user.ID, Kind: "stitched", Title: title, Body: body, WordCount: int16(min(wordCount, 32000)), //nolint:gosec // ограничено
		CoverageAtBuild: float32(text.Coverage), Goal: float32(goal),
	})
	if err != nil {
		s.failTexts(w, fmt.Errorf("сохранение текста: %w", err))
		return
	}
	out := readTextOf(id, mode, title, goal, text.Coverage, stored)
	writeJSON(w, http.StatusOK, out)
}

// textHints собирает подсказки: целевые слова (все их формы в тексте) и неизвестные слова, совпадающие с леммой из базы.
func (s *authServer) textHints(ctx context.Context, text texts.Text, cands []texts.Candidate, targetRow map[string]db.TextTargetWordsRow, known map[string]bool) ([]api.ReadWordHint, error) {
	var hints []api.ReadWordHint
	seen := map[string]bool{}
	formOf := map[string]string{} // форма → лемма целевого слова
	for _, c := range cands {
		if slices.Contains(text.Targets, c.Target) {
			for f := range c.Forms {
				formOf[f] = c.Target
			}
		}
	}
	var other []string
	for _, sn := range text.Sentences {
		for _, tok := range exercises.UnknownTokens(sn.Tokens, known) {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			base := strings.TrimSuffix(strings.TrimSuffix(tok, "'s"), "'")
			lemma, isTarget := formOf[tok]
			if !isTarget {
				lemma, isTarget = formOf[base]
			}
			if isTarget {
				row := targetRow[lemma]
				card, _ := parseCard(row.Payload)
				hints = append(hints, hintOf(tok, row.LexemeID, row.Lemma, row.Pos, card, true))
			} else {
				other = append(other, tok)
			}
		}
	}
	if len(other) > 0 {
		rows, err := s.store.TextWordHints(ctx, other)
		if err != nil {
			return nil, fmt.Errorf("подсказки: %w", err)
		}
		done := map[string]bool{}
		for _, r := range rows { // самое частое слово леммы идёт первым
			if done[r.Lemma] {
				continue
			}
			done[r.Lemma] = true
			card, ok := parseCard(r.Payload)
			if !ok {
				continue
			}
			hints = append(hints, hintOf(r.Lemma, r.LexemeID, r.Lemma, r.Pos, card, false))
		}
	}
	if hints == nil {
		hints = []api.ReadWordHint{}
	}
	return hints, nil
}

func hintOf(token string, lexemeID int64, lemma, pos string, card teachingCard, target bool) api.ReadWordHint {
	h := api.ReadWordHint{Token: token, LexemeId: lexemeID, Lemma: lemma, Pos: pos, Target: target, Translations: card.Translations[:min(len(card.Translations), 2)]}
	if h.Translations == nil {
		h.Translations = []string{}
	}
	if card.IPA != "" {
		ipa := card.IPA
		h.Ipa = &ipa
	}
	return h
}

// RateText принимает ответ «как понял»: один раз на текст двигает цель понимания.
func (s *authServer) RateText(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.RateTextRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxTextBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() || req.TextId <= 0 ||
		!slices.Contains([]string{texts.UnderstoodEasy, texts.UnderstoodOK, texts.UnderstoodHard}, string(req.Understood)) ||
		(req.Mode != api.RateTextRequestModeRead && req.Mode != api.RateTextRequestModeListen) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны textId, understood (easy, ok, hard) и mode (read, listen)")
		return
	}
	ctx := r.Context()
	text, err := s.store.TextGet(ctx, db.TextGetParams{ID: req.TextId, UserID: &user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такого текста нет")
		return
	}
	if err != nil {
		s.failTexts(w, fmt.Errorf("текст: %w", err))
		return
	}
	goal, err := s.textGoal(ctx, user.ID)
	if err != nil {
		s.failTexts(w, fmt.Errorf("цель понимания: %w", err))
		return
	}
	result := api.RateTextResult{Goal: float32(goal)}
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		n, err := tx.TextRateInsert(ctx, db.TextRateInsertParams{
			UserID: user.ID, TextID: text.ID, Mode: string(req.Mode), Understood: string(req.Understood), CoverageAtRead: text.CoverageAtBuild,
		})
		if err != nil || n == 0 {
			return err
		}
		next := texts.NextGoal(goal, string(req.Understood))
		if err := tx.TextSetCoverage(ctx, db.TextSetCoverageParams{UserID: user.ID, TextCoverage: float32(next)}); err != nil {
			return err
		}
		result = api.RateTextResult{Goal: float32(next), Changed: true}
		payload, err := json.Marshal(map[string]any{"textId": text.ID, "understood": req.Understood, "mode": req.Mode, "goalBefore": goal, "goalAfter": next})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: user.ID, Type: "text.read", SchemaVersion: 1, OccurredAt: ts(s.now()), Payload: payload})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		s.failTexts(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// KnowTextWord отмечает слово из текста известным (known_at_start), если у него ещё нет карточки и состояния.
func (s *authServer) KnowTextWord(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.KnowWordRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxTextBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() || req.LexemeId <= 0 {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужен lexemeId")
		return
	}
	ctx := r.Context()
	cards, err := s.store.KbGetTeachingCards(ctx, []int64{req.LexemeId})
	if err != nil {
		s.failTexts(w, fmt.Errorf("слово: %w", err))
		return
	}
	if len(cards) == 0 {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такого слова нет в базе знаний")
		return
	}
	var marked bool
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		n, err := tx.LearnMarkKnown(ctx, db.LearnMarkKnownParams{UserID: user.ID, LexemeID: req.LexemeId})
		if err != nil || n == 0 {
			return err
		}
		marked = true
		payload, err := json.Marshal(map[string]any{"lexemeId": req.LexemeId, "kind": "known"})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: user.ID, Type: "text.word_added", SchemaVersion: 1, OccurredAt: ts(s.now()), Payload: payload})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		s.failTexts(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.KnowWordResult{Marked: marked})
}
