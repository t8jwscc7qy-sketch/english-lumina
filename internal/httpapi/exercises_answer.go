package httpapi

import (
	"context"
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
	maxGivenLen   = 400
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
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidAnswer, "Ответ должен быть от 1 до 400 символов")
		return
	}

	ctx := r.Context()
	var result api.AnswerResult
	// Два одновременных неверных ответа на одно предложение могут оба решить открыть ошибку; проигравшего
	// остановит уникальный индекс, и его транзакция откатывается целиком. Один повтор увидит уже открытую ошибку.
	for attempt := 0; attempt < 2; attempt++ {
		err = s.answerTx(ctx, user, req, &result)
		if !isUniqueViolation(err) {
			break
		}
	}
	var matchErr *exercises.MatchError
	switch {
	case errors.As(err, &matchErr):
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidAnswer, matchErr.Message)
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

type answerWord = *struct {
	InCards bool   `json:"inCards"`
	Lemma   string `json:"lemma"`
	Pos     string `json:"pos"`
}

// formWord — слово упражнения «выбор формы» для результата ответа (nil у других видов).
func formWord(ctx context.Context, tx AuthStore, userID int64, kind string, a exerciseAnswer) (answerWord, error) {
	if kind != string(api.Form) {
		return nil, nil
	}
	ids, err := tx.ListCardLexemeIDs(ctx, db.ListCardLexemeIDsParams{UserID: userID, Ids: []int64{a.LexemeID}})
	if err != nil {
		return nil, err
	}
	return &struct {
		InCards bool   `json:"inCards"`
		Lemma   string `json:"lemma"`
		Pos     string `json:"pos"`
	}{InCards: slices.Contains(ids, a.LexemeID), Lemma: a.Lemma, Pos: a.Pos}, nil
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

// answerTx — вся работа ответа в одной транзакции; результат кладёт в result.
func (s *authServer) answerTx(ctx context.Context, user db.User, req api.AnswerRequest, result *api.AnswerResult) error {
	return s.store.InTx(ctx, func(tx AuthStore) error {
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
				Correct bool               `json:"correct"`
				Mistake *api.MistakeResult `json:"mistake"`
				Pairs   []api.PairResult   `json:"pairs"`
				// только у translate: опечатка и совпавший допустимый вариант
				NearMiss string `json:"nearMiss"`
				Matched  string `json:"matched"`
			}
			if err := json.Unmarshal(prev.Payload, &old); err != nil {
				return fmt.Errorf("событие %d: %w", prev.ID, err)
			}
			rows, err := tx.ListTopicRatings(ctx, user.ID)
			if err != nil {
				return err
			}
			*result = api.AnswerResult{
				Correct: old.Correct, Answer: answer.Answers[0], Translation: payload.Translation,
				Ratings: currentRatings(rows, ex.Topics),
				// Состояние ошибки берём из события первого ответа и банк не трогаем: повтор не двигает серию.
				Mistake: old.Mistake,
			}
			if result.Word, err = formWord(ctx, tx, user.ID, ex.Kind, answer); err != nil {
				return err
			}
			if ex.Kind == string(api.Match) {
				result.Pairs = &old.Pairs
			}
			if ex.Kind == string(api.Translate) {
				setTranslateResult(result, answer.Answers, old.Matched, old.NearMiss)
			}
			return nil
		}

		// Время берём уже после блокировки строки.
		now := time.Now()
		var correct bool
		score := 0.0
		var pairs []api.PairResult
		var verdict exercises.TranslateVerdict
		if ex.Kind == string(api.Translate) {
			// Опечатка в одну букву засчитывается верным ответом: серию и рейтинг не ломает, ошибку не открывает.
			verdict = exercises.CheckTranslate(answer.Answers, req.Given)
			correct = verdict.Result != exercises.TranslateWrong
			if correct {
				score = 1
			}
		} else if ex.Kind == string(api.Match) {
			// Дробный счёт: доля верных пар. Неверный формат — ошибка 400, ничего не записано.
			res, right, err := exercises.CheckMatch(answer.Pairs, req.Given)
			if err != nil {
				return err
			}
			correct, score = right == len(res), float64(right)/float64(len(res))
			pairs = make([]api.PairResult, len(res))
			for i, r := range res {
				pairs[i] = api.PairResult{Word: r.Word, Given: r.Given, Answer: r.Answer, Correct: r.Correct}
			}
		} else {
			check := exercises.Check
			if ex.Kind == string(api.Order) {
				check = exercises.CheckOrder
			}
			correct = check(answer.Answers, req.Given)
			if correct {
				score = 1
			}
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
		mres, err := recordMistake(ctx, tx, user.ID, ex, req.Given, correct, now)
		if err != nil {
			return err
		}
		evData := map[string]any{
			"exerciseId": ex.ID, "kind": ex.Kind, "topics": ex.Topics, "correct": correct, "given": req.Given,
			"ratingBefore": before, "ratingAfter": after,
		}
		if ex.Kind == string(api.Form) {
			evData["lexemeId"] = answer.LexemeID
		}
		nearMiss := ""
		if ex.Kind == string(api.Translate) {
			evData["matched"] = verdict.Matched
			if verdict.Result == exercises.TranslateTypo {
				nearMiss = verdict.Matched
				evData["nearMiss"] = nearMiss
			}
		}
		if ex.Kind == string(api.Match) {
			right := 0
			for _, p := range pairs {
				if p.Correct {
					right++
				}
			}
			evData["correctPairs"], evData["pairs"] = right, pairs
		}
		if mres != nil {
			evData["mistake"] = mres // чтобы повтор запроса по ключу отдал то же состояние ошибки
		}
		evPayload, err := json.Marshal(evData)
		if err != nil {
			return err
		}
		if _, err := tx.AppendEvent(ctx, db.AppendEventParams{
			UserID: user.ID, Type: "exercise.answered", SchemaVersion: 1, OccurredAt: ts(now), Payload: evPayload, IdempotencyKey: &key,
		}); err != nil {
			// ErrNoRows здесь — гонка с тем же ключом, невозможная при блокировке строки: считаем ошибкой и откатываем.
			return err
		}
		*result = api.AnswerResult{Correct: correct, Answer: answer.Answers[0], Translation: payload.Translation, Ratings: make([]api.Rating, 0, len(ex.Topics)), Mistake: mres}
		for _, topic := range ex.Topics {
			result.Ratings = append(result.Ratings, updated[topic])
		}
		if result.Word, err = formWord(ctx, tx, user.ID, ex.Kind, answer); err != nil {
			return err
		}
		if ex.Kind == string(api.Match) {
			result.Pairs = &pairs
		}
		if ex.Kind == string(api.Translate) {
			setTranslateResult(result, answer.Answers, verdict.Matched, nearMiss)
		}
		return nil
	})
}

// maxAccepted — сколько других допустимых переводов показываем после ответа.
const maxAccepted = 3

// setTranslateResult дописывает в результат «перевода» опечатку и другие допустимые переводы
// (без основного и без совпавшего; список всегда есть, пусть и пустой).
func setTranslateResult(result *api.AnswerResult, answers []string, matched, nearMiss string) {
	accepted := []string{}
	if len(answers) == 0 {
		result.Accepted = &accepted
		return
	}
	for _, a := range answers[1:] {
		if a != matched && len(accepted) < maxAccepted {
			accepted = append(accepted, a)
		}
	}
	result.Accepted = &accepted
	if nearMiss != "" {
		result.NearMiss = &nearMiss
	}
}
