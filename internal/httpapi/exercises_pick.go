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
