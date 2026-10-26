package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/library"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const (
	maxAddCardBody = 4 << 10
	maxCardSources = 5 // сколько примеров рассматриваем для карточки
	maxReviewBody  = 4 << 10
	maxReviewKey   = 64
)

var errCardNotFound = errors.New("карточки нет или она чужая")

// AddCard добавляет слово в карточки. Слово ищется по точной паре (лемма, часть речи); карточку нельзя
// собрать без примера, где форма слова найдена, поэтому без примеров ответ 409. Повтор не создаёт вторую карточку.
func (s *authServer) AddCard(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.AddCardRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAddCardBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный запрос")
		return
	}
	lemma, pos := strings.TrimSpace(req.Lemma), strings.TrimSpace(req.Pos)
	if lemma == "" || !validText(lemma, words.MaxQueryLen) || !validText(pos, maxPOSLen) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны lemma (до 64 символов) и pos (до 32 символов)")
		return
	}

	ctx := r.Context()
	lexeme, err := s.store.GetLexemeByPair(ctx, db.GetLexemeByPairParams{Lemma: lemma, Pos: pos})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeWordNotFound, "Такого слова нет в словаре")
		return
	}
	if err != nil {
		s.failCards(w, err)
		return
	}
	// Карточка показывает уровень слова: без строк в lexeme_cefr очередь падала бы на NULL, поэтому такое слово не берём.
	levels, err := s.store.ListLexemeLevels(ctx, []int64{lexeme.ID})
	if err != nil {
		s.failCards(w, err)
		return
	}
	if len(levels) == 0 {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeWordNotFound, "В списках нет уровня для этого слова")
		return
	}
	sentences, err := exampleSentences(ctx, s.store, lexeme.Lemma, lexeme.Pos)
	if err != nil {
		s.failCards(w, err)
		return
	}
	if len(sentences) == 0 {
		writeError(w, http.StatusConflict, api.ErrorErrorCodeNoExamples, "Для этого слова нет подходящих примеров, карточку не собрать")
		return
	}

	var cardID int64
	var created bool
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		now := time.Now()
		id, err := tx.InsertCard(ctx, insertParams(user.ID, lexeme.ID, srs.New(now)))
		if errors.Is(err, pgx.ErrNoRows) { // карточка уже есть
			existing, err := tx.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: user.ID, LexemeID: lexeme.ID})
			cardID = existing.ID
			return err
		}
		if err != nil {
			return err
		}
		cardID, created = id, true
		payload, err := json.Marshal(map[string]any{"cardId": id, "lexemeId": lexeme.ID, "lemma": lexeme.Lemma, "pos": lexeme.Pos})
		if err != nil {
			return err
		}
		key := fmt.Sprintf("card.added:%d", id)
		if _, err := tx.AppendEvent(ctx, db.AppendEventParams{
			UserID: user.ID, Type: "card.added", SchemaVersion: 1, OccurredAt: ts(now), Payload: payload, IdempotencyKey: &key,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		s.failCards(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, api.AddCardResponse{CardId: cardID, Created: created})
}

// exampleSentence — пример предложения для слова и место найденной формы в нём (в символах).
type exampleSentence struct {
	Row        db.ListExamplesRow
	Start, End int
}

// authoredSentenceLimit — сколько фраз из авторских текстов добавляется к примерам слова.
const authoredSentenceLimit = 2

var authoredLibrary = sync.OnceValue(func() []library.Text {
	all, err := library.LoadAll(content.Texts)
	if err != nil {
		slog.Error("авторские тексты не загружены для карточек", "err", err)
		return nil
	}
	return all
})

// authoredSentences — предложения авторских текстов, где есть форма слова (первые authoredSentenceLimit). У них нет
// номера Tatoeba: автор «Verst», номера 0.
func authoredSentences(forms [][]string) []exampleSentence {
	var out []exampleSentence
	for _, t := range authoredLibrary() {
		for _, sn := range t.Sentences {
			if len(out) >= authoredSentenceLimit {
				return out
			}
			if start, end, ok := words.FindMatch(sn.En, forms); ok {
				out = append(out, exampleSentence{Row: db.ListExamplesRow{EnText: sn.En, RuText: sn.Ru, EnAuthor: authoredAuthor, RuAuthor: authoredAuthor}, Start: start, End: end})
			}
		}
	}
	return out
}

// cardAuthoredSentences — источник фраз из авторских текстов; в тестах подменяется, чтобы обычные тесты не зависели от
// содержимого библиотеки.
var cardAuthoredSentences = authoredSentences

// authoredAuthor — «автор» фраз из авторских текстов проекта.
const authoredAuthor = "Verst"

// exampleSentences возвращает до maxCardSources примеров, где форма слова найдена (без этого пропуска не сделать), и
// первыми фразы из авторских текстов (со связным контекстом), если они есть.
func exampleSentences(ctx context.Context, store AuthStore, lemma, pos string) ([]exampleSentence, error) {
	forms := words.Forms(lemma, pos)
	if len(forms) == 0 {
		return nil, nil
	}
	rows, err := store.ListExamples(ctx, db.ListExamplesParams{Query: words.TSQuery(forms), MaxRows: maxCardSources})
	if err != nil {
		return nil, err
	}
	out := cardAuthoredSentences(forms)
	for _, row := range rows {
		if start, end, ok := words.FindMatch(row.EnText, forms); ok {
			out = append(out, exampleSentence{Row: row, Start: start, End: end})
		}
	}
	return out, nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()} }

func insertParams(userID, lexemeID int64, c srs.Card) db.InsertCardParams {
	return db.InsertCardParams{
		UserID: userID, LexemeID: lexemeID, Due: ts(c.Due), Stability: c.Stability, Difficulty: c.Difficulty,
		ScheduledDays: int32(c.ScheduledDays), Reps: int32(c.Reps), Lapses: int32(c.Lapses), //nolint:gosec // малые значения
		State: int16(c.State), LastReview: ts(c.LastReview), RemainingSteps: int32(c.RemainingSteps), //nolint:gosec // малые значения
	}
}

func (s *authServer) failCards(w http.ResponseWriter, err error) {
	slog.Error("карточки", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// GetCardQueue отдаёт, сколько карточек к повторению сейчас, и следующую из них.
func (s *authServer) GetCardQueue(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	q, err := queueResponse(r.Context(), s.store, user.ID, time.Now())
	if err != nil {
		s.failCards(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// ReviewCard применяет оценку: в одной транзакции блокирует карточку, пересчитывает FSRS, пишет событие
// и обновляет состояние. Событие с таким же ключом уже есть — оценка была применена раньше, повтор ничего не меняет.
// Ответ — очередь после оценки (следующая карточка).
func (s *authServer) ReviewCard(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.ReviewCardRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxReviewBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.CardId <= 0 || req.Rating < int(srs.Again) || req.Rating > int(srs.Easy) ||
		req.Key == "" || !validText(req.Key, maxReviewKey) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны cardId, rating от 1 до 4 и key (до 64 символов)")
		return
	}

	ctx := r.Context()
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		row, err := tx.GetCardForUpdate(ctx, db.GetCardForUpdateParams{ID: req.CardId, UserID: user.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errCardNotFound
		}
		if err != nil {
			return err
		}
		// Время берём уже после блокировки строки и не раньше последнего повторения: при гонке двух запросов или
		// сбое часов last_review может оказаться «в будущем», а библиотека такую карточку отклоняет (500 вместо повтора).
		now := time.Now()
		if row.LastReview.Valid && now.Before(row.LastReview.Time) {
			now = row.LastReview.Time
		}
		before := toSRS(row.Due, row.Stability, row.Difficulty, row.ScheduledDays, row.Reps, row.Lapses, row.State, row.LastReview, row.RemainingSteps)
		after, err := srs.Review(before, srs.Rating(req.Rating), now)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{
			"cardId": row.ID, "lexemeId": row.LexemeID, "rating": req.Rating,
			"stateBefore": int(before.State), "stateAfter": int(after.State),
			"dueAfter": after.Due.UTC().Format(time.RFC3339Nano), "scheduledDays": after.ScheduledDays,
			"stability": after.Stability, "difficulty": after.Difficulty,
		})
		if err != nil {
			return err
		}
		key := fmt.Sprintf("review:%d:%s", row.ID, req.Key)
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{
			UserID: user.ID, Type: "card.reviewed", SchemaVersion: 1, OccurredAt: ts(now), Payload: payload, IdempotencyKey: &key,
		})
		if errors.Is(err, pgx.ErrNoRows) { // тот же ключ уже применён: повтор запроса, ничего не меняем
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.UpdateCardState(ctx, updateParams(row.ID, after)); err != nil {
			return err
		}
		if req.Rating >= int(srs.Good) {
			return markLearned(ctx, tx, user.ID, row.LexemeID, now)
		}
		return nil
	})
	if errors.Is(err, errCardNotFound) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такой карточки нет")
		return
	}
	if err != nil {
		s.failCards(w, err)
		return
	}
	q, err := queueResponse(ctx, s.store, user.ID, time.Now())
	if err != nil {
		s.failCards(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// queueResponse собирает очередь: счётчики, следующая просроченная карточка, её сроки после каждой оценки
// и предложение с пропуском. Предложение меняется от повторения к повторению: номер reps mod число примеров.
func queueResponse(ctx context.Context, store AuthStore, userID int64, now time.Time) (api.CardQueue, error) {
	counts, err := store.CountCards(ctx, db.CountCardsParams{UserID: userID, Now: ts(now)})
	if err != nil {
		return api.CardQueue{}, err
	}
	out := api.CardQueue{DueCount: counts.Due, TotalCount: counts.Total}
	next, err := store.NextDueCard(ctx, db.NextDueCardParams{UserID: userID, Now: ts(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return api.CardQueue{}, err
	}

	card := toSRS(next.Due, next.Stability, next.Difficulty, next.ScheduledDays, next.Reps, next.Lapses, next.State, next.LastReview, next.RemainingSteps)
	preview, err := srs.Preview(card, now)
	if err != nil {
		return api.CardQueue{}, err
	}
	sec := func(r srs.Rating) int64 { return int64(preview[r] / time.Second) }
	qc := &api.QueueCard{
		CardId: next.ID, Lemma: next.Lemma, Pos: next.Pos, Level: api.CefrLevel(next.Level), State: int(next.State),
		Intervals: api.CardIntervals{Again: sec(srs.Again), Hard: sec(srs.Hard), Good: sec(srs.Good), Easy: sec(srs.Easy)},
	}
	sentences, err := exampleSentences(ctx, store, next.Lemma, next.Pos)
	if err != nil {
		return api.CardQueue{}, err
	}
	if len(sentences) > 0 {
		ex := sentences[int(next.Reps)%len(sentences)]
		qc.Sentence = &api.CardSentence{
			En:  api.ExampleSentence{Text: ex.Row.EnText, Author: ex.Row.EnAuthor, TatoebaId: ex.Row.EnTatoebaID},
			Ru:  api.ExampleSentence{Text: ex.Row.RuText, Author: ex.Row.RuAuthor, TatoebaId: ex.Row.RuTatoebaID},
			Gap: api.TextRange{Start: ex.Start, End: ex.End},
		}
		if a, ok := audioFor(ctx, store, []int64{ex.Row.EnTatoebaID})[ex.Row.EnTatoebaID]; ok && ex.Row.EnTatoebaID != 0 {
			qc.Sentence.En.Audio = &a
		}
	}
	out.Card = qc
	return out, nil
}

// toSRS переводит поля строки таблицы в состояние карточки для алгоритма.
func toSRS(due pgtype.Timestamptz, stability, difficulty float64, scheduledDays, reps, lapses int32, state int16, lastReview pgtype.Timestamptz, remainingSteps int32) srs.Card {
	c := srs.Card{
		Due: due.Time, Stability: stability, Difficulty: difficulty, ScheduledDays: int(scheduledDays),
		Reps: int(reps), Lapses: int(lapses), State: srs.State(state), RemainingSteps: int(remainingSteps),
	}
	if lastReview.Valid {
		c.LastReview = lastReview.Time
	}
	return c
}

func updateParams(id int64, c srs.Card) db.UpdateCardStateParams {
	return db.UpdateCardStateParams{
		ID: id, Due: ts(c.Due), Stability: c.Stability, Difficulty: c.Difficulty,
		ScheduledDays: int32(c.ScheduledDays), Reps: int32(c.Reps), Lapses: int32(c.Lapses), //nolint:gosec // малые значения
		State: int16(c.State), LastReview: ts(c.LastReview), RemainingSteps: int32(c.RemainingSteps), //nolint:gosec // малые значения
	}
}
