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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/mistakes"
)

const (
	maxListedMistakes = 200                // сколько ошибок отдаёт GET /mistakes; counts.open считает все
	closedWindow      = 7 * 24 * time.Hour // окно для «закрыто за неделю»
	statusOpen        = "open"
	statusClosed      = "closed"
)

// ListMistakes отдаёт открытые ошибки по сроку повтора (не больше 200) и счётчики.
func (s *authServer) ListMistakes(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	now := time.Now()
	rows, err := s.store.ListOpenMistakes(ctx, db.ListOpenMistakesParams{UserID: user.ID, MaxRows: maxListedMistakes})
	if err != nil {
		s.failMistakes(w, fmt.Errorf("список ошибок: %w", err))
		return
	}
	counts, err := s.store.CountMistakes(ctx, db.CountMistakesParams{UserID: user.ID, Now: ts(now), WeekAgo: ts(now.Add(-closedWindow))})
	if err != nil {
		s.failMistakes(w, fmt.Errorf("счётчики ошибок: %w", err))
		return
	}
	out := api.MistakeList{
		Open:   make([]api.Mistake, 0, len(rows)), // [], а не null
		Counts: api.MistakeCounts{Open: counts.Open, DueNow: counts.DueNow, ClosedThisWeek: counts.ClosedThisWeek},
	}
	for _, m := range rows {
		out.Open = append(out.Open, mistakeToAPI(m))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *authServer) failMistakes(w http.ResponseWriter, err error) {
	slog.Error("банк ошибок", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// mistakeToAPI: данные копии, которые не читаются, дают пустые поля, а не ошибку: список остаётся рабочим.
func mistakeToAPI(m db.Mistake) api.Mistake {
	var p exercisePayload
	var a exerciseAnswer
	_ = json.Unmarshal(m.Payload, &p)
	_ = json.Unmarshal(m.Answer, &a)
	topic := p.Topic
	if topic == "" && len(m.Topics) > 0 {
		topic = m.Topics[0]
	}
	answer := ""
	if len(a.Answers) > 0 {
		answer = a.Answers[0]
	}
	return api.Mistake{
		Id: m.ID, Topic: topic, Kind: m.Kind, Prompt: p.Prompt, Given: m.Given, Answer: answer,
		Streak: int(m.Streak), NextDue: m.NextDue.Time, FirstWrongAt: m.FirstWrongAt.Time,
	}
}

// dayStart — начало суток пользователя в UTC: пояс из настроек, без настроек UTC.
func (s *authServer) dayStart(ctx context.Context, userID int64, now time.Time) (time.Time, error) {
	tz := ""
	st, err := s.store.GetSettings(ctx, userID)
	switch {
	case err == nil:
		tz = st.Timezone
	case !errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, fmt.Errorf("настройки для начала суток: %w", err)
	}
	return mistakes.DayStart(now, tz), nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// recordMistake обновляет банк ошибок по результату ответа; вызывается внутри транзакции ответа.
//   - Ответ на повтор (у упражнения есть mistake_id): серия двигается через mistakes.Advance. Закрытую ошибку
//     не трогаем (иначе неверный ответ открыл бы её снова), отдаём только её состояние.
//   - Неверный ответ на обычное упражнение: открывает ошибку, а если открытая на то же предложение и вид уже есть,
//     сбрасывает её (серия 0, новый срок), не создавая вторую.
//
// nil в результате — банк не затронут (верный обычный ответ или ошибка из упражнения уже удалена).
func recordMistake(ctx context.Context, tx AuthStore, userID int64, ex db.Exercise, given string, correct bool, now time.Time) (*api.MistakeResult, error) {
	if ex.SentenceID == nil {
		return nil, nil // у «сопоставления» предложения нет, банк ошибок ему не нужен
	}
	if ex.MistakeID != nil {
		m, err := tx.GetMistakeForUpdate(ctx, db.GetMistakeForUpdateParams{ID: *ex.MistakeID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка %d для повтора: %w", *ex.MistakeID, err)
		}
		if m.Status != statusOpen {
			return mistakeResult(m.Status, int(m.Streak), m.NextDue.Time), nil
		}
		st := mistakes.Advance(mistakes.State{Streak: int(m.Streak), NextDue: m.NextDue.Time}, correct, now)
		status, closedAt, newGiven := statusOpen, pgtype.Timestamptz{}, m.Given
		if st.Closed {
			status, closedAt = statusClosed, ts(now)
		}
		if !correct {
			newGiven = given
		}
		if err := tx.UpdateMistakeProgress(ctx, db.UpdateMistakeProgressParams{
			ID: m.ID, Streak: int16(st.Streak), Status: status, NextDue: ts(st.NextDue), ClosedAt: closedAt, Given: newGiven,
		}); err != nil {
			return nil, fmt.Errorf("ход ошибки %d: %w", m.ID, err)
		}
		if st.Closed {
			if err := appendMistakeEvent(ctx, tx, userID, "mistake.closed", now, map[string]any{"mistakeId": m.ID}); err != nil {
				return nil, err
			}
		}
		return mistakeResult(status, st.Streak, st.NextDue), nil
	}
	if correct {
		return nil, nil
	}
	opened := mistakes.Opened(now)
	m, err := tx.GetOpenMistakeForUpdate(ctx, db.GetOpenMistakeForUpdateParams{UserID: userID, SentenceID: *ex.SentenceID, Kind: ex.Kind})
	switch {
	case err == nil:
		if err := tx.ResetMistake(ctx, db.ResetMistakeParams{
			ID: m.ID, Given: given, NextDue: ts(opened.NextDue), Payload: ex.Payload, Answer: ex.Answer, Topics: ex.Topics,
		}); err != nil {
			return nil, fmt.Errorf("сброс ошибки %d: %w", m.ID, err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		id, err := tx.InsertMistake(ctx, db.InsertMistakeParams{
			UserID: userID, Kind: ex.Kind, SentenceID: *ex.SentenceID, Topics: ex.Topics, Payload: ex.Payload, Answer: ex.Answer,
			Given: given, NextDue: ts(opened.NextDue), FirstWrongAt: ts(now), SourceExerciseID: &ex.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("новая ошибка по упражнению %d: %w", ex.ID, err)
		}
		if err := appendMistakeEvent(ctx, tx, userID, "mistake.opened", now,
			map[string]any{"mistakeId": id, "exerciseId": ex.ID, "kind": ex.Kind, "topics": ex.Topics}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("поиск открытой ошибки: %w", err)
	}
	return mistakeResult(statusOpen, 0, opened.NextDue), nil
}

func mistakeResult(status string, streak int, nextDue time.Time) *api.MistakeResult {
	return &api.MistakeResult{Status: api.MistakeResultStatus(status), Streak: streak, NextDue: nextDue}
}

func appendMistakeEvent(ctx context.Context, tx AuthStore, userID int64, typ string, now time.Time, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("событие %s: %w", typ, err)
	}
	if _, err := tx.AppendEvent(ctx, db.AppendEventParams{UserID: userID, Type: typ, SchemaVersion: 1, OccurredAt: ts(now), Payload: payload}); err != nil {
		return fmt.Errorf("событие %s: %w", typ, err)
	}
	return nil
}

// repeatExercise выдаёт повтор ошибки, если пора: срок наступил, за сутки пользователя меньше 5 повторов,
// два последних упражнения не были повторами. Самая старая по сроку; ошибка с испорченными данными пропускается.
// Для ошибки с неотвеченной копией возвращается та же копия. false — повторов нет, нужен обычный выбор.
func (s *authServer) repeatExercise(ctx context.Context, userID int64, now time.Time) (api.Exercise, bool, error) {
	if _, err := s.store.NextDueMistake(ctx, db.NextDueMistakeParams{UserID: userID, Now: ts(now)}); errors.Is(err, pgx.ErrNoRows) {
		return api.Exercise{}, false, nil
	} else if err != nil {
		return api.Exercise{}, false, fmt.Errorf("ближайшая ошибка: %w", err)
	}
	since, err := s.dayStart(ctx, userID, now)
	if err != nil {
		return api.Exercise{}, false, err
	}
	today, err := s.store.CountRepeatsToday(ctx, db.CountRepeatsTodayParams{UserID: userID, Since: ts(since)})
	if err != nil {
		return api.Exercise{}, false, fmt.Errorf("повторов за сегодня: %w", err)
	}
	flags, err := s.store.LastTwoExerciseRepeatFlags(ctx, userID)
	if err != nil {
		return api.Exercise{}, false, fmt.Errorf("последние упражнения: %w", err)
	}
	anyRepeat := slices.Contains(flags, true) // повтор среди двух последних отвеченных
	if !mistakes.ShouldServeRepeat(int(today), anyRepeat, true) {
		return api.Exercise{}, false, nil
	}
	open, err := s.store.ListOpenMistakes(ctx, db.ListOpenMistakesParams{UserID: userID, MaxRows: maxListedMistakes})
	if err != nil {
		return api.Exercise{}, false, fmt.Errorf("открытые ошибки: %w", err)
	}
	for _, m := range open {
		if m.NextDue.Time.After(now) {
			break // список по сроку: дальше только будущие
		}
		ex, ok, err := s.repeatFor(ctx, userID, m)
		if err != nil {
			return api.Exercise{}, false, err
		}
		if ok {
			return ex, true, nil
		}
	}
	return api.Exercise{}, false, nil
}

// repeatFor отдаёт неотвеченную копию ошибки или создаёт новую. false — данные ошибки не годятся, её надо пропустить.
func (s *authServer) repeatFor(ctx context.Context, userID int64, m db.Mistake) (api.Exercise, bool, error) {
	prev, err := s.store.GetUnansweredRepeat(ctx, db.GetUnansweredRepeatParams{UserID: userID, MistakeID: &m.ID})
	switch {
	case err == nil:
		var p exercisePayload
		if json.Unmarshal(prev.Payload, &p) == nil && p.Prompt != "" {
			return exerciseView(prev.ID, prev.Kind, p, true), true, nil
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return api.Exercise{}, false, fmt.Errorf("неотвеченная копия ошибки %d: %w", m.ID, err)
	}

	var p exercisePayload
	var a exerciseAnswer
	if json.Unmarshal(m.Payload, &p) != nil || json.Unmarshal(m.Answer, &a) != nil || p.Prompt == "" || len(a.Answers) == 0 {
		slog.Warn("ошибка пропущена: данные копии не читаются", "mistakeId", m.ID)
		return api.Exercise{}, false, nil
	}
	seed := s.rnd(1 << 62)
	switch m.Kind {
	case string(api.Gap), string(api.Form):
		if len(p.Choices) == 0 {
			slog.Warn("ошибка пропущена: нет вариантов", "mistakeId", m.ID)
			return api.Exercise{}, false, nil
		}
		shuffled, ok := exercises.Reshuffle(p.Choices, nil, seed)
		if !ok {
			return api.Exercise{}, false, nil
		}
		p.Choices = shuffled
	case string(api.Order):
		if len(p.Tiles) < 2 {
			slog.Warn("ошибка пропущена: нет плиток", "mistakeId", m.ID)
			return api.Exercise{}, false, nil
		}
		shuffled, ok := exercises.Reshuffle(p.Tiles, func(t []string) bool {
			return exercises.CheckOrder(a.Answers, strings.Join(t, " "))
		}, seed)
		if !ok {
			slog.Warn("ошибка пропущена: плитки не перемешать", "mistakeId", m.ID)
			return api.Exercise{}, false, nil
		}
		p.Tiles = shuffled
	default:
		slog.Warn("ошибка пропущена: неизвестный вид", "mistakeId", m.ID, "kind", m.Kind)
		return api.Exercise{}, false, nil
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return api.Exercise{}, false, fmt.Errorf("копия ошибки %d: %w", m.ID, err)
	}
	id, err := s.store.InsertExercise(ctx, db.InsertExerciseParams{
		UserID: userID, Kind: m.Kind, SentenceID: &m.SentenceID, Topics: slices.Clone(m.Topics),
		Payload: payload, Answer: m.Answer, MistakeID: &m.ID,
	})
	if err != nil {
		return api.Exercise{}, false, fmt.Errorf("запись повтора ошибки %d: %w", m.ID, err)
	}
	return exerciseView(id, m.Kind, p, true), true, nil
}

// exerciseView — то, что видит клиент: без допустимых ответов.
func exerciseView(id int64, kind string, p exercisePayload, repeat bool) api.Exercise {
	choices := p.Choices
	if choices == nil {
		choices = []string{}
	}
	ex := api.Exercise{Id: id, Kind: api.ExerciseKind(kind), Prompt: p.Prompt, Translation: p.Translation, Choices: choices, Topic: p.Topic}
	if len(p.Tiles) > 0 {
		tiles := slices.Clone(p.Tiles)
		ex.Tiles = &tiles
	}
	if p.Hint != "" {
		hint := p.Hint
		ex.Hint = &hint
	}
	if repeat {
		r := true
		ex.Repeat = &r
	}
	return ex
}
