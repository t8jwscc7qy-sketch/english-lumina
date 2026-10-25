package httpapi

import (
	"context"
	"slices"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// Подбор предложения для упражнения. Запросов к базе немного: границы номеров один раз на вызов, окно предложений
// на тему, одна проверка уровней на окно кандидатов, у level:* ещё по одному поиску примеров на слово.

const (
	maxWindowCandidates = 30 // сколько кандидатов окна проверяем по уровням одним запросом
	maxLevelBatches     = 2  // сколько раз проверяем уровни в level-пути
	maxAlternatives     = 5  // сколько альтернативных английских предложений берём для «порядка слов»
)

// idRange — границы номеров предложений; читаются из базы лениво и один раз на вызов next.
type idRange struct {
	loaded   bool
	min, max int64
}

func (s *authServer) loadRange(ctx context.Context, r *idRange) error {
	if r.loaded {
		return nil
	}
	row, err := s.store.SentenceIDRange(ctx)
	if err != nil {
		return err
	}
	*r = idRange{loaded: true, min: row.MinID, max: row.MaxID}
	return nil
}

// levelInfo одним запросом узнаёт уровни и частоты слов.
func (s *authServer) levelInfo(ctx context.Context, ws []string) (map[string]exercises.WordInfo, error) {
	rows, err := s.store.SentenceLevels(ctx, uniqueWords(ws))
	if err != nil {
		return nil, err
	}
	out := make(map[string]exercises.WordInfo, len(rows))
	for _, r := range rows {
		var info exercises.WordInfo
		if r.Level != nil {
			info.Level = *r.Level
		}
		if r.Rank != nil {
			info.Rank = int(*r.Rank)
		}
		out[r.Word] = info
	}
	return out, nil
}

func uniqueWords(ws []string) []string {
	out := slices.Clone(ws)
	slices.Sort(out)
	return slices.Compact(out)
}

type grammarPending struct {
	row    db.SampleSentencesRow
	tokens []string
	target string // past_simple: слово-цель
	gap    exercises.Gap
	seed   int64
}

// sampleWindow берёт подряд идущие предложения с случайного места; при пустом окне повторяет с нуля.
func (s *authServer) sampleWindow(ctx context.Context, rng *idRange) ([]db.SampleSentencesRow, error) {
	if err := s.loadRange(ctx, rng); err != nil || rng.max == 0 {
		return nil, err
	}
	from := rng.min + s.rnd(rng.max-rng.min+1)
	rows, err := s.store.SampleSentences(ctx, db.SampleSentencesParams{FromID: from, MaxRows: sampleRows})
	if err == nil && len(rows) == 0 && from > 0 {
		rows, err = s.store.SampleSentences(ctx, db.SampleSentencesParams{FromID: 0, MaxRows: sampleRows})
	}
	return rows, err
}

// grammarCandidate просматривает подряд идущие предложения с случайного места, отбирает подходящие по теме
// и берёт первое, у которого проходит уровень и собирается пропуск.
func (s *authServer) grammarCandidate(ctx context.Context, rng *idRange, topic string, used map[int64]bool) (gapCandidate, bool, error) {
	rows, err := s.sampleWindow(ctx, rng)
	if err != nil {
		return gapCandidate{}, false, err
	}
	var pending []grammarPending
	var lookup []string
	for _, row := range rows {
		if len(pending) >= maxWindowCandidates {
			break
		}
		if used[row.ID] {
			continue
		}
		cased := exercises.CasedTokens(row.Text)
		if !slices.Contains(grammar.Topics(cased), topic) {
			continue
		}
		tokens := exercises.Tokens(row.Text)
		p := grammarPending{row: row, tokens: tokens, seed: s.rnd(1 << 62)}
		if topic == grammar.PastSimple {
			p.target = exercises.PastSimpleTarget(cased)
			if p.target == "" {
				continue
			}
			// Основу, по которой строятся варианты, ищем заранее: нет ни таблицы, ни кандидатов — предложение не нужно.
			if _, irregular := words.IrregularLemma(p.target); !irregular {
				bases := exercises.BaseCandidates(p.target)
				if len(bases) == 0 {
					continue
				}
				lookup = append(lookup, bases...)
			}
		} else {
			var ok bool
			if p.gap, ok = exercises.BuildGap(row.Text, topic, cased, p.seed); !ok {
				continue
			}
		}
		lookup = append(lookup, tokens...)
		pending = append(pending, p)
	}
	if len(pending) == 0 {
		return gapCandidate{}, false, nil
	}
	known, err := s.levelInfo(ctx, lookup)
	if err != nil {
		return gapCandidate{}, false, err
	}
	for _, p := range pending {
		level, ok := exercises.SentenceLevel(p.tokens, known)
		if !ok {
			continue
		}
		gap := p.gap
		if topic == grammar.PastSimple {
			if gap, ok = exercises.BuildGapForWord(p.row.Text, p.target, exercises.PastDistractors(p.target, known), p.seed); !ok {
				continue
			}
		}
		return gapCandidate{sentenceID: p.row.ID, translation: p.row.RuText, gap: gap, level: level}, true, nil
	}
	return gapCandidate{}, false, nil
}

type levelPending struct {
	id          int64
	translation string
	tokens      []string
	gap         exercises.Gap
	pos         string
}

// levelCandidate берёт случайные слова уровня и ищет для них предложение (один поиск примеров на слово);
// отвлекающие варианты — другие слова того же уровня и той же части речи. Предложение не должно быть сложнее темы.
func (s *authServer) levelCandidate(ctx context.Context, topic string, used map[int64]bool) (gapCandidate, bool, error) {
	level := strings.TrimPrefix(topic, "level:")
	lexemes, err := s.store.RandomLexemesByLevel(ctx, db.RandomLexemesByLevelParams{Level: level, MaxRows: levelLexemeBatch})
	if err != nil {
		return gapCandidate{}, false, err
	}
	tries, batches := 0, 0
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
		var pending []levelPending
		var lookup []string
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
			tokens := exercises.Tokens(row.EnText)
			lookup = append(lookup, tokens...)
			pending = append(pending, levelPending{id: row.EnID, translation: row.RuText, tokens: tokens, gap: gap, pos: lx.Pos})
		}
		if len(pending) == 0 {
			continue
		}
		if batches++; batches > maxLevelBatches {
			break
		}
		known, err := s.levelInfo(ctx, lookup)
		if err != nil {
			return gapCandidate{}, false, err
		}
		for _, p := range pending {
			if lvl, ok := exercises.SentenceLevel(p.tokens, known); ok && lvl <= level {
				return gapCandidate{sentenceID: p.id, translation: p.translation, gap: p.gap, level: lvl, pos: p.pos}, true, nil
			}
		}
	}
	return gapCandidate{}, false, nil
}

type orderPending struct {
	row    db.SampleSentencesRow
	tokens []string
	seed   int64
}

// orderCandidate берёт окно предложений, оставляет те, из которых собирается «порядок слов», проверяет уровни
// одним запросом (уровень не выше maxLevel) и для первого подходящего ищет альтернативные английские предложения
// с тем же переводом. maxLevel пуст — предела нет.
func (s *authServer) orderCandidate(ctx context.Context, rng *idRange, maxLevel string, used map[int64]bool) (gapCandidate, bool, error) {
	rows, err := s.sampleWindow(ctx, rng)
	if err != nil {
		return gapCandidate{}, false, err
	}
	var pending []orderPending
	var lookup []string
	for _, row := range rows {
		if len(pending) >= maxWindowCandidates {
			break
		}
		if used[row.ID] {
			continue
		}
		p := orderPending{row: row, tokens: exercises.Tokens(row.Text), seed: s.rnd(1 << 62)}
		if _, ok := exercises.BuildOrder(row.Text, nil, p.seed); !ok {
			continue
		}
		lookup = append(lookup, p.tokens...)
		pending = append(pending, p)
	}
	if len(pending) == 0 {
		return gapCandidate{}, false, nil
	}
	known, err := s.levelInfo(ctx, lookup)
	if err != nil {
		return gapCandidate{}, false, err
	}
	for _, p := range pending {
		level, ok := exercises.SentenceLevel(p.tokens, known)
		if !ok || (maxLevel != "" && level > maxLevel) {
			continue
		}
		alts, err := s.store.ListEnglishAlternatives(ctx, db.ListEnglishAlternativesParams{EnID: p.row.ID, RuID: p.row.RuID, MaxRows: maxAlternatives})
		if err != nil {
			return gapCandidate{}, false, err
		}
		texts := make([]string, len(alts))
		for i, a := range alts {
			texts[i] = a.Text
		}
		order, ok := exercises.BuildOrder(p.row.Text, texts, p.seed)
		if !ok {
			continue
		}
		return gapCandidate{sentenceID: p.row.ID, translation: p.row.RuText, order: &order, level: level}, true, nil
	}
	return gapCandidate{}, false, nil
}

const (
	formWeakWords   = 10 // слабых слов карточек, с которых начинаем
	formWordTries   = 6  // у скольких слов ищем предложения
	formMinForms    = 3  // меньше однословных форм — упражнения не будет (у существительных их две)
	formLexemeBatch = 40 // случайных слов на один уровень
)

type formPending struct {
	id          int64
	translation string
	tokens      []string
	gap         exercises.Gap
}

// levelWords — слова упражнений по словам: сначала слабые слова карточек (не больше weak), затем случайные слова
// уровня пользователя и следующего за ним (по batch на уровень; без настроек — A1). Подходят слова, для которых
// posOK верно; повторы убираются.
func (s *authServer) levelWords(ctx context.Context, userID int64, level string, posOK func(string) bool, weak, batch int32) ([]formInfo, error) {
	var out []formInfo
	seen := map[int64]bool{}
	add := func(id int64, lemma, pos string) {
		if !seen[id] && posOK(pos) {
			seen[id] = true
			out = append(out, formInfo{lexemeID: id, lemma: lemma, pos: pos})
		}
	}
	weakRows, err := s.store.ListWeakCardWords(ctx, db.ListWeakCardWordsParams{UserID: userID, MaxRows: weak})
	if err != nil {
		return nil, err
	}
	for _, w := range weakRows {
		add(w.LexemeID, w.Lemma, w.Pos)
	}
	levels := []string{"A1"}
	if i := slices.Index(allLevels, level); i >= 0 {
		levels = allLevels[i:min(i+2, len(allLevels))]
	}
	for _, lv := range levels {
		rows, err := s.store.RandomLexemesByLevel(ctx, db.RandomLexemesByLevelParams{Level: lv, MaxRows: batch})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			add(r.ID, r.Lemma, r.Pos)
		}
	}
	return out, nil
}

// formWords — слова для «выбора формы»: подходят только глаголы и прилагательные.
func (s *authServer) formWords(ctx context.Context, userID int64, level string) ([]formInfo, error) {
	return s.levelWords(ctx, userID, level, func(pos string) bool { return pos == "verb" || pos == "adjective" }, formWeakWords, formLexemeBatch)
}

// formCandidate ищет для слова (слабые слова карточек первыми) предложение, в котором ровно одна его форма.
// level — уровень пользователя; предложение не выше уровня на единицу (без уровня предела нет).
func (s *authServer) formCandidate(ctx context.Context, _ *idRange, userID int64, level string, used map[int64]bool) (gapCandidate, bool, error) {
	cands, err := s.formWords(ctx, userID, level)
	if err != nil {
		return gapCandidate{}, false, err
	}
	maxLevel := orderMaxLevel(level)
	tries, batches := 0, 0
	for _, w := range cands {
		forms := words.SingleWords(words.Forms(w.lemma, w.pos))
		if len(forms) < formMinForms {
			continue
		}
		if tries++; tries > formWordTries {
			break
		}
		// Правила дают и несуществующие формы (stoped, visitted): в варианты идут только формы из списка частот.
		freqs, err := s.store.ListWordFrequencies(ctx, db.ListWordFrequenciesParams{SourceName: frequencySource, Words: forms})
		if err != nil {
			return gapCandidate{}, false, err
		}
		real := make([]string, 0, len(freqs))
		for _, fr := range freqs {
			real = append(real, fr.Word)
		}
		if len(real) < 2 {
			continue
		}
		rows, err := s.store.ListExamples(ctx, db.ListExamplesParams{Query: words.TSQuery(words.Forms(w.lemma, w.pos)), MaxRows: levelExamples})
		if err != nil {
			return gapCandidate{}, false, err
		}
		var pending []formPending
		var lookup []string
		for _, row := range rows {
			if used[row.EnID] {
				continue
			}
			seed := s.rnd(1 << 62)
			// Сначала по всем формам узнаём форму из предложения: она верна, даже если её нет в списке частот.
			_, answer, ok := exercises.BuildForm(row.EnText, w.pos, forms, seed)
			if !ok {
				continue
			}
			allowed := append(slices.Clone(real), strings.ToLower(answer))
			gap, _, ok := exercises.BuildForm(row.EnText, w.pos, allowed, seed)
			if !ok {
				continue
			}
			tokens := exercises.Tokens(row.EnText)
			lookup = append(lookup, tokens...)
			pending = append(pending, formPending{id: row.EnID, translation: row.RuText, tokens: tokens, gap: gap})
		}
		if len(pending) == 0 {
			continue
		}
		if batches++; batches > maxLevelBatches {
			break
		}
		known, err := s.levelInfo(ctx, lookup)
		if err != nil {
			return gapCandidate{}, false, err
		}
		for _, p := range pending {
			lvl, ok := exercises.SentenceLevel(p.tokens, known)
			if !ok || (maxLevel != "" && lvl > maxLevel) {
				continue
			}
			info := w
			return gapCandidate{sentenceID: p.id, translation: p.translation, gap: p.gap, level: lvl, pos: w.pos, form: &info}, true, nil
		}
	}
	return gapCandidate{}, false, nil
}

const (
	matchWeakWords   = 10 // слабых слов карточек в набор кандидатов
	matchLexemeBatch = 30 // случайных слов на уровень
	dictionarySource = "freedict-eng-rus"
)

// dictPos — часть речи словаря (n, v, adj, adv) по части речи лексемы; пусто, если слово не годится.
var dictPos = map[string]string{"noun": "n", "verb": "v", "adjective": "adj", "adverb": "adv"}

// matchCandidate собирает «сопоставление»: слова как у «выбора формы» (слабые карточки, затем слова уровня), переводы
// из словаря. Меньше пяти подходящих слов — не найдено (вид пропускается, остальные работают).
func (s *authServer) matchCandidate(ctx context.Context, userID int64, level string) (gapCandidate, bool, error) {
	ws, err := s.levelWords(ctx, userID, level, func(pos string) bool { return dictPos[pos] != "" }, matchWeakWords, matchLexemeBatch)
	if err != nil {
		return gapCandidate{}, false, err
	}
	if len(ws) < exercises.MatchPairs {
		return gapCandidate{}, false, nil
	}
	lemmas := make([]string, 0, len(ws))
	for _, w := range ws {
		if !slices.Contains(lemmas, w.lemma) {
			lemmas = append(lemmas, w.lemma)
		}
	}
	rows, err := s.store.ListTranslationsForWords(ctx, db.ListTranslationsForWordsParams{SourceName: dictionarySource, Lemmas: lemmas})
	if err != nil {
		return gapCandidate{}, false, err
	}
	type key struct{ lemma, pos string }
	senses := map[key][][]string{}
	lastSense := map[key]int16{}
	for _, r := range rows {
		k := key{r.Lemma, r.Pos}
		if n := len(senses[k]); n == 0 || lastSense[k] != r.SenseNo {
			senses[k] = append(senses[k], nil)
			lastSense[k] = r.SenseNo
		}
		last := len(senses[k]) - 1
		senses[k][last] = append(senses[k][last], r.Ru)
	}
	cands := make([]exercises.MatchCandidate, 0, len(ws))
	for _, w := range ws {
		if sn := senses[key{w.lemma, dictPos[w.pos]}]; len(sn) > 0 {
			cands = append(cands, exercises.MatchCandidate{Word: w.lemma, Senses: sn})
		}
	}
	m, ok := exercises.BuildMatch(cands, s.rnd(1<<62))
	if !ok {
		return gapCandidate{}, false, nil
	}
	c := gapCandidate{match: &m}
	if slices.Contains(allLevels, level) {
		c.level = level
	}
	return c, true, nil
}
