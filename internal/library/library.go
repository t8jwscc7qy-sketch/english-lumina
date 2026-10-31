// Package library — заранее написанные тексты («авторские»): схема, проверка и словарь уровня. Тексты пишутся заранее и
// проверяются тестом и командой `go run ./cmd/textcheck`; сервер ничего не генерирует во время работы.
package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// Bands — уровни по размеру словаря из частотного списка: в тексте этого уровня все слова, кроме целевых, входят в
// самые частотные Band слов (лемм).
var Bands = []int{100, 200, 300, 600, 1000, 2000}

// Границы длины текста в словах и числа целевых слов.
const (
	MinTargets = 3
	MaxTargets = 6
	// TargetRepeats — сколько раз целевое слово встречается в тексте минимум.
	TargetRepeats = 2

	microMaxBand  = 100 // микро-истории: короткие, с повторами
	microMinWords = 30
	microMaxWords = 60
	shortBand     = 200 // переходный уровень: рассказ покороче
	shortMinWords = 60
	shortMaxWords = 120
	minWords      = 80
	maxWords      = 150
)

var idPattern = regexp.MustCompile(`^t[0-9]+-[0-9]{3}$`)

// Target — целевое слово текста: лемма и часть речи (по ней строятся формы).
type Target struct {
	Lemma string `json:"lemma"`
	Pos   string `json:"pos"`
	// Mass — неисчисляемое существительное (milk, water, music): артикль перед ним не нужен.
	Mass bool `json:"mass,omitempty"`
}

// Sentence — предложение: английский и русский.
type Sentence struct {
	En string `json:"en"`
	Ru string `json:"ru"`
}

// Text — авторский текст.
type Text struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Band    int      `json:"band"`
	Targets []Target `json:"targets"`
	Names   []string `json:"names,omitempty"` // имена героев: в словарь не входят и не проверяются
	// Allow — осознанные исключения из проверки стиля («артикль:6»: правило и номер предложения), например «Bad puppy!».
	Allow     []string   `json:"allow,omitempty"`
	Sentences []Sentence `json:"sentences"`
	Sources   []string   `json:"sources"`
}

// Parse строго разбирает один текст: лишние поля и хвост в файле — ошибка; структура проверяется.
func Parse(data []byte) (Text, error) {
	var t Text
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return Text{}, fmt.Errorf("разбор: %w", err)
	}
	if dec.More() {
		return Text{}, errors.New("после текста в файле есть лишнее")
	}
	if err := t.Validate(); err != nil {
		return Text{}, fmt.Errorf("текст %q: %w", t.ID, err)
	}
	return t, nil
}

// Tokens — слова предложения в исходном регистре (как у чтения: буквы, цифры, апостроф и дефис внутри слова).
func (s Sentence) Tokens() []string { return exercises.CasedTokens(s.En) }

// WordCount — число слов во всём тексте.
func (t Text) WordCount() int {
	n := 0
	for _, s := range t.Sentences {
		n += len(s.Tokens())
	}
	return n
}

// TargetForms — формы целевого слова в нижнем регистре.
func (tg Target) Forms() map[string]bool {
	out := map[string]bool{strings.ToLower(tg.Lemma): true}
	for _, f := range words.SingleWords(words.Forms(strings.ToLower(tg.Lemma), tg.Pos)) {
		out[f] = true
	}
	return out
}

// Validate проверяет структуру текста (без словаря): поля, число и повторы целевых слов, длину.
func (t Text) Validate() error {
	switch {
	case !idPattern.MatchString(t.ID):
		return fmt.Errorf("id %q: вид t100-001", t.ID)
	case strings.TrimSpace(t.Title) == "" || len(t.Title) > 120:
		return errors.New("нужен заголовок до 120 символов")
	case !slices.Contains(Bands, t.Band):
		return fmt.Errorf("уровень %d: один из %v", t.Band, Bands)
	case !strings.HasPrefix(t.ID, fmt.Sprintf("t%d-", t.Band)):
		return fmt.Errorf("id %q не соответствует уровню %d", t.ID, t.Band)
	case len(t.Targets) < MinTargets || len(t.Targets) > MaxTargets:
		return fmt.Errorf("целевых слов от %d до %d, а их %d", MinTargets, MaxTargets, len(t.Targets))
	case len(t.Sources) == 0:
		return errors.New("нужен источник")
	case len(t.Sentences) < 3:
		return errors.New("предложений не меньше трёх")
	}
	seen := map[string]bool{}
	for _, tg := range t.Targets {
		if tg.Lemma == "" || strings.ToLower(tg.Lemma) != tg.Lemma || !slices.Contains([]string{"noun", "verb", "adjective", "adverb"}, tg.Pos) {
			return fmt.Errorf("целевое слово %+v: строчная лемма и часть речи noun/verb/adjective/adverb", tg)
		}
		if seen[tg.Lemma] {
			return fmt.Errorf("целевое слово %q повторяется в списке", tg.Lemma)
		}
		seen[tg.Lemma] = true
	}
	for i, s := range t.Sentences {
		if strings.TrimSpace(s.En) == "" || strings.TrimSpace(s.Ru) == "" {
			return fmt.Errorf("предложение %d: нужны en и ru", i+1)
		}
	}
	n := t.WordCount()
	lo, hi := minWords, maxWords
	switch {
	case t.Band <= microMaxBand:
		lo, hi = microMinWords, microMaxWords
	case t.Band == shortBand:
		lo, hi = shortMinWords, shortMaxWords
	}
	if n < lo || n > hi {
		return fmt.Errorf("слов %d, для уровня %d нужно от %d до %d", n, t.Band, lo, hi)
	}
	for _, tg := range t.Targets {
		if c := t.countTarget(tg); c < TargetRepeats {
			return fmt.Errorf("целевое слово %q встречается %d раз, нужно не меньше %d", tg.Lemma, c, TargetRepeats)
		}
	}
	return nil
}

func (t Text) countTarget(tg Target) int {
	forms := tg.Forms()
	n := 0
	for _, s := range t.Sentences {
		toks := s.Tokens()
		for i, tok := range toks {
			if forms[lowerToken(tok)] && !isName(tok, i) {
				n++
			}
		}
	}
	return n
}

func lowerToken(t string) string { return strings.ToLower(strings.ReplaceAll(t, "’", "'")) }

// isName — слово с заглавной буквы не в начале предложения (имя).
func isName(tok string, i int) bool {
	r := []rune(tok)
	return i > 0 && len(r) > 0 && unicode.IsUpper(r[0])
}

// Problem — нарушение словаря: слово, которого нет в словаре уровня и среди целевых слов.
type Problem struct {
	Sentence int
	Word     string
}

// contractionBase возвращает основу слова с сокращением (don't → do, I'm → i, it's → it) и true, если оно сокращённое.
func contractionBase(w string) (string, bool) {
	switch w {
	case "can't", "cannot":
		return "can", true
	case "won't":
		return "will", true
	case "ain't":
		return "be", true
	case "shan't":
		return "shall", true
	}
	for _, suf := range []string{"n't", "'m", "'re", "'ve", "'ll", "'d", "'s"} {
		if base, ok := strings.CutSuffix(w, suf); ok && base != "" {
			return base, true
		}
	}
	return "", false
}

// CheckVocabulary проверяет, что все слова текста входят в allowed (формы слов словаря уровня), целевые формы или имена.
// Числа и слова-имена не проверяются; сокращения проверяются по основе.
func (t Text) CheckVocabulary(allowed map[string]bool) []Problem {
	targetForms := map[string]bool{}
	for _, tg := range t.Targets {
		for f := range tg.Forms() {
			targetForms[f] = true
		}
	}
	names := map[string]bool{}
	for _, n := range t.Names {
		names[n] = true
	}
	var out []Problem
	for si, s := range t.Sentences {
		for i, tok := range s.Tokens() {
			w := lowerToken(tok)
			r := []rune(tok)
			switch {
			case len(r) == 0 || unicode.IsDigit(r[0]) || names[tok] || names[strings.TrimSuffix(strings.TrimSuffix(tok, "'s"), "’s")] || isName(tok, i):
				continue
			case allowed[w] || targetForms[w]:
				continue
			}
			if base, ok := contractionBase(w); ok && (allowed[base] || targetForms[base]) {
				continue
			}
			// Любая форма слова из словаря уровня допустима: found → find, was → be, went → go, walked → walk.
			if inflectedFromAllowed(w, allowed, targetForms) {
				continue
			}
			out = append(out, Problem{Sentence: si + 1, Word: w})
		}
	}
	return out
}

// LoadAll читает тексты из каталога texts/*.json, проверяет структуру и уникальность id; порядок — по id.
func LoadAll(fsys fs.FS) ([]Text, error) {
	files, err := fs.Glob(fsys, "texts/*.json")
	if err != nil {
		return nil, err
	}
	var out []Text
	ids := map[string]string{}
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("%s: файл — список текстов: %w", f, err)
		}
		for _, raw := range batch {
			t, err := Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if prev, dup := ids[t.ID]; dup {
				return nil, fmt.Errorf("%s: id %q уже есть в %s", f, t.ID, prev)
			}
			ids[t.ID] = f
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// inflectedFromAllowed — слово является формой слова из словаря уровня (или целевого слова): по таблицам неправильных слов
// и правилам окончаний. Неверные формы («goed», «finded») не проходят.
func inflectedFromAllowed(w string, allowed, targetForms map[string]bool) bool {
	for _, base := range words.InflectionBases(w) {
		if allowed[base] || targetForms[base] {
			return true
		}
	}
	return false
}
