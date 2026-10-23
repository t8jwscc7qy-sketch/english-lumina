package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
)

const (
	maxTopicTries    = 5   // сколько тем пробуем, прежде чем сказать «упражнений нет»
	sampleRows       = 200 // сколько подряд идущих предложений просматриваем на тему
	levelLexemeBatch = 40  // сколько случайных слов уровня берём для темы level:*
	maxLexemeTries   = 6   // у скольких из них ищем предложение
	levelExamples    = 20  // примеров на слово
)

// allLevels упорядочены от низшего к высшему (в данных бывает и C2).
var allLevels = []string{"A1", "A2", "B1", "B2", "C1", "C2"}

var grammarTopics = []string{grammar.Articles, grammar.Prepositions, grammar.Future, grammar.Modals, grammar.PastSimple}

// exercisePayload — что показано пользователю; лежит в exercises.payload.
type exercisePayload struct {
	Prompt      string   `json:"prompt"`
	Translation string   `json:"translation"`
	Choices     []string `json:"choices"`
	Topic       string   `json:"topic"`
}

// exerciseAnswer — допустимые ответы; лежит в exercises.answer.
type exerciseAnswer struct {
	Answers []string `json:"answers"`
}

// gapCandidate — собранное упражнение до записи в базу.
type gapCandidate struct {
	sentenceID  int64
	translation string
	gap         exercises.Gap
	level       string // уровень предложения
	pos         string // часть речи слова (только для level:*)
}

// GetNextExercise выбирает тему по рейтингам (слабая раньше), собирает пропуск и записывает упражнение.
// Правильного ответа в ответе нет: он хранится только в базе.
func (s *authServer) GetNextExercise(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	ctx := r.Context()
	available, err := s.availableTopics(ctx, user.ID)
	if err != nil {
		s.failExercises(w, err)
		return
	}
	stored, err := s.store.ListTopicRatings(ctx, user.ID)
	if err != nil {
		s.failExercises(w, err)
		return
	}
	ratings := make([]exercises.TopicRating, len(stored))
	for i, tr := range stored {
		ratings[i] = exercises.TopicRating{Topic: tr.Topic, Mu: tr.Rating, Phi: tr.Deviation, Attempts: int(tr.Attempts)}
	}

	var rng idRange
	for try := 0; try < maxTopicTries; try++ {
		topic, ok := exercises.PickTopic(ratings, available)
		if !ok {
			break
		}
		available = slices.DeleteFunc(available, func(t string) bool { return t == topic })
		used, err := s.usedSentences(ctx, user.ID, topic)
		if err != nil {
			s.failExercises(w, err)
			return
		}
		var cand gapCandidate
		var found bool
		if strings.HasPrefix(topic, "level:") {
			cand, found, err = s.levelCandidate(ctx, topic, used)
		} else {
			cand, found, err = s.grammarCandidate(ctx, &rng, topic, used)
		}
		if err != nil {
			s.failExercises(w, err)
			return
		}
		if !found {
			continue
		}
		ex, err := s.saveExercise(ctx, user.ID, topic, cand)
		if err != nil {
			s.failExercises(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ex)
		return
	}
	writeError(w, http.StatusNotFound, api.ErrorErrorCodeNoExercise, "Подходящих упражнений пока нет")
}

// ListExerciseTopics отдаёт рейтинги тем, по которым пользователь уже занимался.
func (s *authServer) ListExerciseTopics(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	rows, err := s.store.ListTopicRatings(r.Context(), user.ID)
	if err != nil {
		s.failExercises(w, err)
		return
	}
	out := api.TopicList{Topics: make([]api.Rating, 0, len(rows))} // [], а не null
	for _, tr := range rows {
		out.Topics = append(out.Topics, api.Rating{Topic: tr.Topic, Rating: tr.Rating, Deviation: tr.Deviation, Attempts: int(tr.Attempts)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *authServer) failExercises(w http.ResponseWriter, err error) {
	slog.Error("упражнения", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// availableTopics: пять грамматических тем, уровень пользователя и следующий за ним. Без настроек — только грамматика.
func (s *authServer) availableTopics(ctx context.Context, userID int64) ([]string, error) {
	out := slices.Clone(grammarTopics)
	st, err := s.store.GetSettings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	i := slices.Index(allLevels, st.CurrentLevel)
	if i < 0 {
		return out, nil
	}
	out = append(out, "level:"+allLevels[i])
	if i+1 < len(allLevels) {
		out = append(out, "level:"+allLevels[i+1])
	}
	return out, nil
}

func (s *authServer) usedSentences(ctx context.Context, userID int64, topic string) (map[int64]bool, error) {
	ids, err := s.store.UsedSentences(ctx, db.UsedSentencesParams{UserID: userID, Topic: topic})
	if err != nil {
		return nil, err
	}
	used := make(map[int64]bool, len(ids))
	for _, id := range ids {
		used[id] = true
	}
	return used, nil
}

func (s *authServer) saveExercise(ctx context.Context, userID int64, topic string, c gapCandidate) (api.Exercise, error) {
	topics := []string{topic}
	extra := []string{posTopic(c.pos)}
	if c.level != "" {
		extra = append([]string{"level:" + c.level}, extra...)
	}
	for _, t := range extra {
		if t != "" && !slices.Contains(topics, t) {
			topics = append(topics, t)
		}
	}
	payload, err := json.Marshal(exercisePayload{Prompt: c.gap.Prompt, Translation: c.translation, Choices: c.gap.Choices, Topic: topic})
	if err != nil {
		return api.Exercise{}, err
	}
	answer, err := json.Marshal(exerciseAnswer{Answers: c.gap.Answers})
	if err != nil {
		return api.Exercise{}, err
	}
	id, err := s.store.InsertExercise(ctx, db.InsertExerciseParams{
		UserID: userID, Kind: string(api.Gap), SentenceID: c.sentenceID, Topics: topics, Payload: payload, Answer: answer,
	})
	if err != nil {
		return api.Exercise{}, err
	}
	return api.Exercise{Id: id, Kind: api.Gap, Prompt: c.gap.Prompt, Translation: c.translation, Choices: c.gap.Choices, Topic: topic}, nil
}

func posTopic(pos string) string {
	if pos == "" {
		return ""
	}
	return "pos:" + pos
}
