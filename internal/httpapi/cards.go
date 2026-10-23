package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const (
	maxAddCardBody = 4 << 10
	maxCardSources = 5 // сколько примеров рассматриваем для карточки
)

// Ручки очереди и оценки реализуются в следующей задаче; пока отвечают 501, чтобы пакет собирался.
func notImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, api.ErrorErrorCodeInternal, "Пока не реализовано")
}

func (s *authServer) GetCardQueue(w http.ResponseWriter, _ *http.Request) { notImplemented(w) }
func (s *authServer) ReviewCard(w http.ResponseWriter, _ *http.Request)   { notImplemented(w) }

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

// exampleSentences возвращает до maxCardSources примеров, где форма слова найдена (без этого пропуска не сделать).
func exampleSentences(ctx context.Context, store AuthStore, lemma, pos string) ([]exampleSentence, error) {
	forms := words.Forms(lemma, pos)
	if len(forms) == 0 {
		return nil, nil
	}
	rows, err := store.ListExamples(ctx, db.ListExamplesParams{Query: words.TSQuery(forms), MaxRows: maxCardSources})
	if err != nil {
		return nil, err
	}
	var out []exampleSentence
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
