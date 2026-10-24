package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/rating"
)

const (
	maxAnswerBody = 4 << 10
	maxGivenLen   = 200
	maxAnswerKey  = 64
)

var (
	errExerciseNotFound = errors.New("упражнения нет или оно чужое")
	errAlreadyAnswered  = errors.New("на упражнение уже отвечали с другим ключом")
)

// AnswerExercise проверяет ответ, обновляет рейтинги всех тем упражнения и пишет событие.
// Всё в одной транзакции; блокировка строки упражнения сериализует параллельные ответы.
// Событие с тем же ключом уже есть — это повтор: тот же результат, ничего не пересчитывается.
// Упражнение отвечено с другим ключом — 409.
func (s *authServer) AnswerExercise(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.AnswerRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAnswerBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.Decode(&struct{}{}) != io.EOF || req.ExerciseId <= 0 ||
		req.Key == "" || !validText(req.Key, maxAnswerKey) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны exerciseId, given и key (до 64 символов)")
		return
	}
	// Пробелы в ответе не обрезаем (их срезает сама проверка), но «только пробелы» — пустой ответ.
	if req.Given == "" || strings.TrimSpace(req.Given) == "" || !validText(req.Given, maxGivenLen) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidAnswer, "Ответ должен быть от 1 до 200 символов")
		return
	}

	ctx := r.Context()
	var result api.AnswerResult
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		ex, err := tx.GetExerciseForUpdate(ctx, db.GetExerciseForUpdateParams{ID: req.ExerciseId, UserID: user.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errExerciseNotFound
		}
		if err != nil {
			return err
		}
		var payload exercisePayload
		var answer exerciseAnswer
		if err := json.Unmarshal(ex.Payload, &payload); err != nil {
			return fmt.Errorf("payload упражнения %d: %w", ex.ID, err)
		}
		if err := json.Unmarshal(ex.Answer, &answer); err != nil || len(answer.Answers) == 0 {
			if err == nil {
				return fmt.Errorf("в упражнении %d нет допустимых ответов", ex.ID)
			}
			return fmt.Errorf("ответ упражнения %d: %w", ex.ID, err)
		}
		key := fmt.Sprintf("exercise:%d:%s", ex.ID, req.Key)

		if ex.AnsweredAt.Valid {
			prev, err := tx.GetEventByKey(ctx, db.GetEventByKeyParams{UserID: user.ID, IdempotencyKey: &key})
			if errors.Is(err, pgx.ErrNoRows) {
				return errAlreadyAnswered
			}
			if err != nil {
				return err
			}
			var old struct {
				Correct bool `json:"correct"`
			}
			if err := json.Unmarshal(prev.Payload, &old); err != nil {
				return fmt.Errorf("событие %d: %w", prev.ID, err)
			}
			rows, err := tx.ListTopicRatings(ctx, user.ID)
			if err != nil {
				return err
			}
			result = api.AnswerResult{
				Correct: old.Correct, Answer: answer.Answers[0], Translation: payload.Translation,
				Ratings: currentRatings(rows, ex.Topics),
			}
			return nil
		}

		// Время берём уже после блокировки строки.
		now := time.Now()
		correct := exercises.Check(answer.Answers, req.Given)
		score := 0.0
		if correct {
			score = 1
		}
		opponent := exercises.Opponent(ex.Topics)
		before := make(map[string]float64, len(ex.Topics))
		after := make(map[string]float64, len(ex.Topics))
		updated := make(map[string]api.Rating, len(ex.Topics))
		// Темы блокируем в одном порядке у всех запросов, чтобы два ответа не ждали друг друга по кругу.
		for _, topic := range sortedCopy(ex.Topics) {
			// Сначала гарантируем строку, потом блокируем её: у новой темы иначе блокировать нечего.
			if err := tx.EnsureTopicRating(ctx, db.EnsureTopicRatingParams{UserID: user.ID, Topic: topic}); err != nil {
				return err
			}
			row, err := tx.GetTopicRatingForUpdate(ctx, db.GetTopicRatingForUpdateParams{UserID: user.ID, Topic: topic})
			if err != nil {
				return err
			}
			cur, attempts := rating.Rating{Mu: row.Rating, Phi: row.Deviation, Sigma: row.Volatility}, row.Attempts
			next := rating.Update(cur, opponent, score)
			if err := tx.UpsertTopicRating(ctx, db.UpsertTopicRatingParams{
				UserID: user.ID, Topic: topic, Rating: next.Mu, Deviation: next.Phi, Volatility: next.Sigma, AttemptsDelta: 1,
			}); err != nil {
				return err
			}
			before[topic], after[topic] = cur.Mu, next.Mu
			updated[topic] = api.Rating{Topic: topic, Rating: next.Mu, Deviation: next.Phi, Attempts: int(attempts) + 1}
		}
		if err := tx.MarkExerciseAnswered(ctx, db.MarkExerciseAnsweredParams{ID: ex.ID, AnsweredAt: ts(now)}); err != nil {
			return err
		}
		evPayload, err := json.Marshal(map[string]any{
			"exerciseId": ex.ID, "kind": ex.Kind, "topics": ex.Topics, "correct": correct, "given": req.Given,
			"ratingBefore": before, "ratingAfter": after,
		})
		if err != nil {
			return err
		}
		if _, err := tx.AppendEvent(ctx, db.AppendEventParams{
			UserID: user.ID, Type: "exercise.answered", SchemaVersion: 1, OccurredAt: ts(now), Payload: evPayload, IdempotencyKey: &key,
		}); err != nil {
			// ErrNoRows здесь — гонка с тем же ключом, невозможная при блокировке строки: считаем ошибкой и откатываем.
			return err
		}
		result = api.AnswerResult{Correct: correct, Answer: answer.Answers[0], Translation: payload.Translation, Ratings: make([]api.Rating, 0, len(ex.Topics))}
		for _, topic := range ex.Topics {
			result.Ratings = append(result.Ratings, updated[topic])
		}
		return nil
	})
	switch {
	case errors.Is(err, errExerciseNotFound):
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такого упражнения нет")
	case errors.Is(err, errAlreadyAnswered):
		writeError(w, http.StatusConflict, api.ErrorErrorCodeAlreadyAnswered, "На это упражнение уже отвечали")
	case err != nil:
		s.failExercises(w, err)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

func sortedCopy(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// currentRatings — текущие рейтинги тем упражнения (в порядке тем упражнения).
func currentRatings(rows []db.TopicRating, topics []string) []api.Rating {
	out := make([]api.Rating, 0, len(topics))
	for _, topic := range topics {
		for _, tr := range rows {
			if tr.Topic == topic {
				out = append(out, api.Rating{Topic: tr.Topic, Rating: tr.Rating, Deviation: tr.Deviation, Attempts: int(tr.Attempts)})
				break
			}
		}
	}
	return out
}
