package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

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
// orderTopic — тема «порядка слов»; выбирается как любая другая слабая тема.
const orderTopic = "skill:order"

// translateTopic — тема «перевода» вводом; выбирается как любая другая слабая тема.
const translateTopic = "skill:translate"

// formTopic — тема «выбора формы»; открывается через exercises.Unlocked.
const formTopic = "skill:form"

// matchTopic — тема «сопоставления»; открывается через exercises.Unlocked, выдаётся, если набралось пять слов со словарём.
const matchTopic = "skill:match"

// matchPrompt — задание «сопоставления» (поле prompt).
const matchPrompt = "Соедини слова с переводами"

var allLevels = []string{"A1", "A2", "B1", "B2", "C1", "C2"}

var grammarTopics = []string{grammar.Articles, grammar.Prepositions, grammar.Future, grammar.Modals, grammar.PastSimple}

// exercisePayload — что показано пользователю; лежит в exercises.payload.
type exercisePayload struct {
	Prompt      string   `json:"prompt"`
	Translation string   `json:"translation"`
	Choices     []string `json:"choices"`
	Tiles       []string `json:"tiles,omitempty"` // только для order
	Topic       string   `json:"topic"`
	Hint        string   `json:"hint,omitempty"`    // только для form: начальная форма слова
	Words       []string `json:"words,omitempty"`   // только для match: английские слова в порядке упражнения
	Options     []string `json:"options,omitempty"` // только для match: переводы вперемешку
}

// exerciseAnswer — допустимые ответы; лежит в exercises.answer.
type exerciseAnswer struct {
	Answers  []string `json:"answers"`
	LexemeID int64    `json:"lexemeId,omitempty"` // только для form: слово упражнения
	Lemma    string   `json:"lemma,omitempty"`
	Pos      string   `json:"pos,omitempty"`
	// Pairs — только для match: верные пары в порядке слов; Answers[0] — они же строкой «слово=перевод;…».
	Pairs []exercises.MatchPair `json:"pairs,omitempty"`
}

// formInfo — слово упражнения «выбор формы».
type formInfo struct {
	lexemeID   int64
	lemma, pos string
}

// gapCandidate — собранное упражнение до записи в базу.
type gapCandidate struct {
	sentenceID  int64
	translation string
	gap         exercises.Gap
	level       string               // уровень предложения
	pos         string               // часть речи слова (только для level:*)
	order       *exercises.Order     // «порядок слов»: вместо gap
	form        *formInfo            // «выбор формы»: слово упражнения
	match       *exercises.Match     // «сопоставление»: вместо gap
	translate   *exercises.Translate // «перевод»: вместо gap
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
	// Сначала возвращаем ошибки, которым пора: так работает банк ошибок.
	if rep, ok, err := s.repeatExercise(ctx, user.ID, time.Now()); err != nil {
		s.failExercises(w, err)
		return
	} else if ok {
		writeJSON(w, http.StatusOK, rep)
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
	available, userLevel, err := s.availableTopics(ctx, user.ID, ratings)
	if err != nil {
		s.failExercises(w, err)
		return
	}

	var rng idRange
	for try := 0; try < maxTopicTries; try++ {
		topic, ok := exercises.PickTopic(ratings, available)
		if !ok {
			break
		}
		available = slices.DeleteFunc(available, func(t string) bool { return t == topic })
		used, err := s.usedSentencesFor(ctx, user.ID, topic)
		if err != nil {
			s.failExercises(w, err)
			return
		}
		var cand gapCandidate
		var found bool
		switch {
		case topic == orderTopic:
			cand, found, err = s.orderCandidate(ctx, &rng, orderMaxLevel(userLevel), used)
		case topic == translateTopic:
			cand, found, err = s.translateCandidate(ctx, &rng, orderMaxLevel(userLevel), used)
		case topic == formTopic:
			cand, found, err = s.formCandidate(ctx, &rng, user.ID, userLevel, used)
		case topic == matchTopic:
			cand, found, err = s.matchCandidate(ctx, user.ID, userLevel)
		case strings.HasPrefix(topic, "level:"):
			cand, found, err = s.levelCandidate(ctx, topic, used)
		default:
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

// availableTopics: пять грамматических тем, «порядок слов», «перевод», «выбор формы» и «сопоставление» (если открыты), уровень пользователя и следующий за ним.
// Без настроек — без level:*. Вторым значением отдаёт текущий уровень пользователя (пусто без настроек).
func (s *authServer) availableTopics(ctx context.Context, userID int64, ratings []exercises.TopicRating) ([]string, string, error) {
	out := append(slices.Clone(grammarTopics), orderTopic, translateTopic)
	if exercises.Unlocked(formTopic, ratings) {
		out = append(out, formTopic)
	}
	if exercises.Unlocked(matchTopic, ratings) {
		out = append(out, matchTopic)
	}
	st, err := s.store.GetSettings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	i := slices.Index(allLevels, st.CurrentLevel)
	if i < 0 {
		return out, "", nil
	}
	out = append(out, "level:"+allLevels[i])
	if i+1 < len(allLevels) {
		out = append(out, "level:"+allLevels[i+1])
	}
	return out, allLevels[i], nil
}

// orderMaxLevel — самый высокий уровень предложения для «порядка слов»: уровень пользователя плюс один.
// Уровень неизвестен — предела нет.
func orderMaxLevel(userLevel string) string {
	i := slices.Index(allLevels, userLevel)
	if i < 0 {
		return ""
	}
	return allLevels[min(i+1, len(allLevels)-1)]
}

// usedSentencesFor: предложения, которые тема уже использовала. «Порядок слов» и «перевод» делят предложения: собрав
// плитки, пользователь видел английский ответ, и печатать его по памяти уже не упражнение.
func (s *authServer) usedSentencesFor(ctx context.Context, userID int64, topic string) (map[int64]bool, error) {
	used, err := s.usedSentences(ctx, userID, topic)
	if err != nil {
		return nil, err
	}
	var twin string
	switch topic {
	case translateTopic:
		twin = orderTopic
	case orderTopic:
		twin = translateTopic
	default:
		return used, nil
	}
	more, err := s.usedSentences(ctx, userID, twin)
	if err != nil {
		return nil, err
	}
	for id := range more {
		used[id] = true
	}
	return used, nil
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
	kind, prompt, choices, answers := api.Gap, c.gap.Prompt, c.gap.Choices, c.gap.Answers
	var tiles []string
	if c.order != nil {
		// «Порядок слов»: пользователь видит перевод и перемешанные плитки.
		kind, prompt, choices, answers, tiles = api.Order, c.translation, []string{}, c.order.Answers, c.order.Tiles
	}
	translation := c.translation
	if c.translate != nil {
		// «Перевод»: пользователь видит русское предложение и печатает английское; перевод-подсказки нет.
		kind, prompt, choices, answers, translation = api.Translate, c.translation, []string{}, c.translate.Answers, ""
	}
	var hint string
	var words, options []string
	var sentenceID *int64
	if c.match == nil {
		sentenceID = &c.sentenceID
	}
	ans := exerciseAnswer{Answers: answers}
	if c.match != nil {
		// «Сопоставление»: предложения нет; пользователь видит слова и переводы вперемешку.
		kind, prompt, choices = api.Match, matchPrompt, []string{}
		words, options = c.match.Words, c.match.Options
		pairs := make([]string, len(c.match.Answers))
		for i, p := range c.match.Answers {
			pairs[i] = p.Word + "=" + p.Ru
		}
		ans = exerciseAnswer{Answers: []string{strings.Join(pairs, ";")}, Pairs: c.match.Answers}
	}
	if c.form != nil {
		kind, hint = api.Form, c.form.lemma
		ans.LexemeID, ans.Lemma, ans.Pos = c.form.lexemeID, c.form.lemma, c.form.pos
	}
	payload, err := json.Marshal(exercisePayload{Prompt: prompt, Translation: translation, Choices: choices, Tiles: tiles, Topic: topic, Hint: hint, Words: words, Options: options})
	if err != nil {
		return api.Exercise{}, err
	}
	answer, err := json.Marshal(ans)
	if err != nil {
		return api.Exercise{}, err
	}
	id, err := s.store.InsertExercise(ctx, db.InsertExerciseParams{
		UserID: userID, Kind: string(kind), SentenceID: sentenceID, Topics: topics, Payload: payload, Answer: answer,
	})
	if err != nil {
		return api.Exercise{}, err
	}
	ex := api.Exercise{Id: id, Kind: kind, Prompt: prompt, Translation: translation, Choices: choices, Topic: topic}
	if tiles != nil {
		ex.Tiles = &tiles
	}
	if hint != "" {
		ex.Hint = &hint
	}
	if c.match != nil {
		ex.Words, ex.Options = &words, &options
	}
	return ex, nil
}

func posTopic(pos string) string {
	if pos == "" {
		return ""
	}
	return "pos:" + pos
}
