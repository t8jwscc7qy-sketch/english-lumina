package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

const (
	maxTopicTries    = 5     // сколько тем пробуем, прежде чем сказать «упражнений нет»
	sampleRows       = 200   // сколько подряд идущих предложений просматриваем на тему
	maxLevelLookups  = 10    // сколько предложений за одну попытку проверяем по уровню
	levelLexemeBatch = 40    // сколько случайных слов уровня берём для темы level:*
	maxLexemeTries   = 6     // у скольких из них ищем предложение
	levelExamples    = 20    // примеров на слово
	maxFreeRank      = 20000 // слово без уровня допустимо, если оно в первых 20 000 частотного списка
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
			cand, found, err = s.grammarCandidate(ctx, topic, used)
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
	for _, t := range []string{"level:" + c.level, posTopic(c.pos)} {
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

// grammarCandidate просматривает подряд идущие предложения с случайного места и берёт первое, где есть тема и
// собирается пропуск.
func (s *authServer) grammarCandidate(ctx context.Context, topic string, used map[int64]bool) (gapCandidate, bool, error) {
	rng, err := s.store.SentenceIDRange(ctx)
	if err != nil || rng.MaxID == 0 {
		return gapCandidate{}, false, err
	}
	from := rng.MinID + s.rnd(rng.MaxID-rng.MinID+1)
	rows, err := s.store.SampleSentences(ctx, db.SampleSentencesParams{FromID: from, MaxRows: sampleRows})
	if err == nil && len(rows) == 0 && from > 0 {
		rows, err = s.store.SampleSentences(ctx, db.SampleSentencesParams{FromID: 0, MaxRows: sampleRows})
	}
	if err != nil {
		return gapCandidate{}, false, err
	}
	lookups := 0
	for _, row := range rows {
		if used[row.ID] {
			continue
		}
		tokens := sentenceTokens(row.Text)
		if !slices.Contains(grammar.Topics(tokens), topic) {
			continue
		}
		seed := s.rnd(1 << 62)
		var gap exercises.Gap
		var extra []string // слова, которые нужно проверить в словаре вместе с предложением
		var target string
		if topic == grammar.PastSimple {
			target = pastSimpleTarget(tokens)
			extra = baseCandidates(target)
		} else {
			var ok bool
			if gap, ok = exercises.BuildGap(row.Text, topic, tokens, seed); !ok {
				continue
			}
		}
		if lookups++; lookups > maxLevelLookups {
			break
		}
		known, err := s.store.SentenceLevels(ctx, uniqueWords(append(slices.Clone(tokens), extra...)))
		if err != nil {
			return gapCandidate{}, false, err
		}
		level, ok := sentenceLevel(tokens, known)
		if !ok {
			continue
		}
		if topic == grammar.PastSimple {
			distractors := pastDistractors(target, known)
			if gap, ok = exercises.BuildGapForWord(row.Text, target, distractors, seed); !ok {
				continue
			}
		}
		return gapCandidate{sentenceID: row.ID, translation: row.RuText, gap: gap, level: level}, true, nil
	}
	return gapCandidate{}, false, nil
}

// levelCandidate берёт случайные слова уровня и ищет для них предложение; отвлекающие варианты — другие слова
// того же уровня и той же части речи.
func (s *authServer) levelCandidate(ctx context.Context, topic string, used map[int64]bool) (gapCandidate, bool, error) {
	level := strings.TrimPrefix(topic, "level:")
	lexemes, err := s.store.RandomLexemesByLevel(ctx, db.RandomLexemesByLevelParams{Level: level, MaxRows: levelLexemeBatch})
	if err != nil {
		return gapCandidate{}, false, err
	}
	tries, lookups := 0, 0
	for _, lx := range lexemes {
		var distractors []string
		for _, o := range lexemes {
			if o.Pos == lx.Pos && o.Lemma != lx.Lemma {
				distractors = append(distractors, o.Lemma)
			}
		}
		if len(distractors) < 3 {
			continue
		}
		if tries++; tries > maxLexemeTries {
			break
		}
		forms := words.Forms(lx.Lemma, lx.Pos)
		if len(forms) == 0 {
			continue
		}
		rows, err := s.store.ListExamples(ctx, db.ListExamplesParams{Query: words.TSQuery(forms), MaxRows: levelExamples})
		if err != nil {
			return gapCandidate{}, false, err
		}
		for _, row := range rows {
			if used[row.EnID] {
				continue
			}
			start, end, ok := words.FindMatch(row.EnText, forms)
			if !ok {
				continue
			}
			// Только само слово без изменений: «dogs» рядом с «cat» выдало бы ответ по форме.
			if strings.ToLower(string([]rune(row.EnText)[start:end])) != lx.Lemma {
				continue
			}
			gap, ok := exercises.BuildGapForWord(row.EnText, lx.Lemma, distractors, s.rnd(1<<62))
			if !ok {
				continue
			}
			if lookups++; lookups > maxLevelLookups {
				return gapCandidate{}, false, nil
			}
			tokens := sentenceTokens(row.EnText)
			known, err := s.store.SentenceLevels(ctx, uniqueWords(tokens))
			if err != nil {
				return gapCandidate{}, false, err
			}
			lvl, ok := sentenceLevel(tokens, known)
			if !ok {
				continue
			}
			return gapCandidate{sentenceID: row.EnID, translation: row.RuText, gap: gap, level: lvl, pos: lx.Pos}, true, nil
		}
	}
	return gapCandidate{}, false, nil
}

// sentenceLevel: высший уровень среди слов предложения. Слово без уровня допустимо, только если оно среди первых
// 20 000 по частоте; если уровня нет ни у одного слова, предложение не берётся.
func sentenceLevel(tokens []string, known []db.SentenceLevelsRow) (string, bool) {
	byWord := make(map[string]db.SentenceLevelsRow, len(known))
	for _, k := range known {
		byWord[k.Word] = k
	}
	best := ""
	for _, t := range tokens {
		k := byWord[t]
		switch {
		case k.Level != nil:
			if *k.Level > best { // «A1» < … < «C2» и по алфавиту
				best = *k.Level
			}
		case k.Rank != nil && *k.Rank <= maxFreeRank:
		default:
			return "", false
		}
	}
	return best, best != ""
}

// sentenceTokens режет предложение на слова в нижнем регистре; апострофы и дефисы внутри слова остаются (don't).
func sentenceTokens(text string) []string {
	text = strings.ReplaceAll(text, "’", "'")
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' && r != '-'
	})
	out := fields[:0]
	for _, f := range fields {
		if f = strings.Trim(f, "'-"); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func uniqueWords(ws []string) []string {
	out := slices.Clone(ws)
	slices.Sort(out)
	return slices.Compact(out)
}

// pastSimpleTarget — первое слово, давшее тему past_simple (с тем же контекстом, что в grammar.Topics).
func pastSimpleTarget(tokens []string) string {
	for i := range tokens {
		if slices.Contains(grammar.Topics(tokens[:i+1]), grammar.PastSimple) {
			return tokens[i]
		}
	}
	return ""
}

// baseCandidates — возможные начальные формы правильного глагола на -ed: walked → walk, walke; liked → lik, like.
// Какая из них настоящая, показывает словарь (SentenceLevels). Для неправильных глаголов — пусто.
func baseCandidates(past string) []string {
	if past == "" {
		return nil
	}
	if _, ok := words.IrregularLemma(past); ok {
		return nil
	}
	switch {
	case strings.HasSuffix(past, "ied") && len(past) > 4:
		return []string{past[:len(past)-3] + "y"}
	case strings.HasSuffix(past, "ed") && len(past) > 4:
		b := past[:len(past)-2]
		out := []string{b, b + "e"}
		if n := len(b); n > 2 && b[n-1] == b[n-2] { // stopped → stop
			out = append(out, b[:n-1])
		}
		return out
	}
	return nil
}

// pastDistractors — другие формы того же глагола. Начальная форма: из таблицы неправильных глаголов или единственная
// из baseCandidates, найденная в словаре; при неоднозначности (hoped: hop/hope) возвращает nil.
func pastDistractors(target string, known []db.SentenceLevelsRow) []string {
	base, ok := words.IrregularLemma(target)
	if !ok {
		var found []string
		for _, c := range baseCandidates(target) {
			for _, k := range known {
				if k.Word == c && (k.Level != nil || k.Rank != nil) {
					found = append(found, c)
				}
			}
		}
		if len(found) != 1 {
			return nil
		}
		base = found[0]
	}
	var out []string
	for _, f := range words.Forms(base, "verb") {
		if len(f) == 1 && f[0] != target {
			out = append(out, f[0])
		}
	}
	return out
}

// AnswerExercise пока заглушка (следующая задача); вход проверяется уже сейчас.
func (s *authServer) AnswerExercise(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	writeError(w, http.StatusNotImplemented, api.ErrorErrorCodeNotImplemented, "Упражнения ещё не готовы")
}
