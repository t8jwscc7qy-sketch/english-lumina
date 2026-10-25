package kb

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// ErrBelowThreshold — сборка хуже порогов качества: данные не записываются.
var ErrBelowThreshold = errors.New("качество базы знаний ниже порога")

const (
	maxStoredTranslations = 8  // сколько переводов слова хранится (выбранные и запасные)
	maxChosen             = 2  // сколько переводов слова выбирается
	maxRelations          = 10 // синонимов (и антонимов) на слово
	maxCardForms          = 8  // форм слова в карточке
	sampleSize            = 50 // слов в выборке отчёта
)

// Lexeme — слово из таблицы lexemes; Level — самый низкий уровень CEFR среди источников (может быть пуст).
type Lexeme struct {
	ID    int64
	Lemma string
	Pos   string
	Level string
}

// LegacyTranslation — перевод из прежнего словаря (FreeDict): Pos — n, v, adj или adv.
type LegacyTranslation struct {
	Lemma   string
	Pos     string
	SenseNo int
	Ru      string
}

// Inputs — всё, что нужно сборке; чтение из базы и файлов делает команда, слияние — Assemble.
type Inputs struct {
	Lexemes   []Lexeme
	Entries   []Entry
	Legacy    []LegacyTranslation
	RuRank    map[string]int   // слово → место в списке частот русского языка
	EnCount   map[string]int64 // английская форма (строчная) → счёт употреблений
	SourceIDs map[string]int64 // "kaikki" и "freedict" → номера в data_sources
}

type (
	SenseRow struct {
		LexemeID int64
		SenseNo  int
		GlossEN  string
		Tags     []string
		Flags    []string
		SourceID int64
	}
	TranslationRow struct {
		LexemeID  int64
		SenseNo   *int
		Lang      string
		Text      string
		Score     float64
		Rank      int
		Chosen    bool
		Flags     []string
		SourceIDs []int64
	}
	PronRow struct {
		LexemeID int64
		Accent   string
		IPA      string
		Audio    string
		Primary  bool
		SourceID int64
	}
	FormRow struct {
		Form     string
		LexemeID int64
		Tag      string
	}
	RelationRow struct {
		LexemeID      int64
		Kind          string
		OtherLemma    string
		OtherLexemeID *int64
	}
	StatRow struct {
		LexemeID int64
		Count    int64
		Rank     *int
	}
	CardRow struct {
		LexemeID int64
		Payload  []byte
	}
)

// Output — строки таблиц kb_* в порядке вставки (значения раньше переводов: внешний ключ).
type Output struct {
	Senses         []SenseRow
	Translations   []TranslationRow
	Pronunciations []PronRow
	Forms          []FormRow
	Relations      []RelationRow
	Stats          []StatRow
	Cards          []CardRow
}

// LevelStats — покрытие слов одного уровня.
type LevelStats struct {
	Total           int
	WithTranslation int
	WithIPA         int
}

// Coverage — доля слов уровня с выбранным переводом (0, если слов нет).
func (l LevelStats) Coverage() float64 {
	if l.Total == 0 {
		return 0
	}
	return float64(l.WithTranslation) / float64(l.Total)
}

// GoldenFail — контрольное слово, у которого основной перевод не из допустимых.
type GoldenFail struct {
	Lemma string
	Pos   string
	Got   string
}

// GoldenResult — итог по контрольному списку (слова, которых нет среди lexemes, не проверяются).
type GoldenResult struct {
	Checked int
	Passed  int
	Failed  []GoldenFail
}

// SampleRow — слово из выборки отчёта: для чтения глазами.
type SampleRow struct {
	Lemma        string
	Pos          string
	Translations []string
	IPA          string
}

// Report — отчёт качества сборки.
type Report struct {
	Levels  map[string]LevelStats
	Golden  GoldenResult
	Sample  []SampleRow
	Flagged int // выбранных переводов с пометками (устаревшее, грубое, просторечное…)
}

// Options — пороги качества. Пустые значения порог не включают.
type Options struct {
	MinCoverage map[string]float64
	MinGolden   float64
}

// DefaultOptions — пороги по умолчанию: покрытие переводом не ниже уровня на момент подключения Kaikki
// (A1 93 %, A2 91 %, B1 89 %, B2 85 %, C1 71 %, C2 67 %) и 90 % контрольного списка.
func DefaultOptions() Options {
	return Options{
		MinCoverage: map[string]float64{"A1": 0.93, "A2": 0.91, "B1": 0.89, "B2": 0.85, "C1": 0.71, "C2": 0.67},
		MinGolden:   0.90,
	}
}

// Check сверяет отчёт с порогами; ErrBelowThreshold с перечнем нарушений, если качество хуже.
func (r Report) Check(o Options) error {
	var problems []string
	for level, min := range o.MinCoverage {
		ls := r.Levels[level]
		if ls.Total > 0 && ls.Coverage() < min {
			problems = append(problems, fmt.Sprintf("покрытие %s %.1f%% ниже порога %.1f%%", level, ls.Coverage()*100, min*100))
		}
	}
	if o.MinGolden > 0 && r.Golden.Checked > 0 {
		if share := float64(r.Golden.Passed) / float64(r.Golden.Checked); share < o.MinGolden {
			problems = append(problems, fmt.Sprintf("контрольный список %.1f%% ниже порога %.1f%%", share*100, o.MinGolden*100))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w: %s", ErrBelowThreshold, strings.Join(problems, "; "))
	}
	return nil
}

// KaikkiPos — части речи Kaikki, подходящие слову с данной частью речи из lexemes (источник CEFR-J).
func KaikkiPos(pos string) []string {
	switch pos {
	case "noun":
		return []string{"noun"}
	case "verb", "modal auxiliary", "be-verb", "have-verb", "do-verb":
		return []string{"verb"}
	case "adjective":
		return []string{"adj"}
	case "adverb":
		return []string{"adv"}
	case "pronoun":
		return []string{"pron"}
	case "preposition":
		return []string{"prep"}
	case "conjunction":
		return []string{"conj"}
	case "determiner":
		return []string{"det"}
	case "number":
		return []string{"num"}
	case "interjection":
		return []string{"intj"}
	case "infinitive-to":
		return []string{"particle"}
	}
	return nil
}

// LegacyPos — часть речи старого словаря (n, v, adj, adv); у остальных пусто.
func LegacyPos(pos string) string {
	switch pos {
	case "noun":
		return "n"
	case "verb":
		return "v"
	case "adjective":
		return "adj"
	case "adverb":
		return "adv"
	}
	return ""
}

var flagTags = []string{"archaic", "obsolete", "dated", "historical", "vulgar", "offensive", "derogatory", "slang", "rare", "dialectal", "regional", "informal"}

func flagsOf(tags []string) []string {
	var out []string
	for _, t := range tags {
		t = strings.ToLower(t)
		if slices.Contains(flagTags, t) && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

type posKey struct{ word, pos string }

type keptSense struct {
	no    int
	gloss []string
	tags  []string
}

// matchSense находит значение, к которому относится пояснение перевода (sense у Kaikki — слова, а не номер).
func matchSense(senses []keptSense, text string) (int, []string) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return 0, nil
	}
	for _, s := range senses {
		for _, g := range s.gloss {
			if strings.Contains(strings.ToLower(g), text) {
				return s.no, s.tags
			}
		}
	}
	return 0, nil
}

func unionTags(a, b []string) []string {
	out := slices.Clone(a)
	for _, t := range b {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func sourceIDsOf(names []string, ids map[string]int64) []int64 {
	var out []int64
	for _, n := range names {
		if id := ids[n]; id != 0 {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

type cardPayload struct {
	Lemma        string   `json:"lemma"`
	Pos          string   `json:"pos"`
	Level        string   `json:"level,omitempty"`
	Rank         *int     `json:"rank,omitempty"`
	Translations []string `json:"translations"`
	IPA          string   `json:"ipa,omitempty"`
	Audio        string   `json:"audio,omitempty"`
	Forms        []string `json:"forms,omitempty"`
	Synonyms     []string `json:"synonyms,omitempty"`
}

// Assemble сливает источники по правилам и возвращает строки таблиц kb_* и отчёт качества. Чистая функция:
// результат не зависит от порядка входных срезов.
func Assemble(in Inputs) (Output, Report) {
	lexemes := slices.Clone(in.Lexemes)
	slices.SortFunc(lexemes, func(a, b Lexeme) int { return int(a.ID - b.ID) })

	entries := slices.Clone(in.Entries)
	slices.SortStableFunc(entries, func(a, b Entry) int {
		if c := strings.Compare(a.Word, b.Word); c != 0 {
			return c
		}
		if c := strings.Compare(a.Pos, b.Pos); c != 0 {
			return c
		}
		return strings.Compare(firstGloss(a), firstGloss(b))
	})
	byKey := map[posKey][]Entry{}
	for _, e := range entries {
		byKey[posKey{e.Word, e.Pos}] = append(byKey[posKey{e.Word, e.Pos}], e)
	}
	legacy := map[posKey][]LegacyTranslation{}
	for _, l := range in.Legacy {
		legacy[posKey{l.Lemma, l.Pos}] = append(legacy[posKey{l.Lemma, l.Pos}], l)
	}
	for _, v := range legacy {
		slices.SortFunc(v, func(a, b LegacyTranslation) int {
			if a.SenseNo != b.SenseNo {
				return a.SenseNo - b.SenseNo
			}
			return strings.Compare(a.Ru, b.Ru)
		})
	}
	byLemma := map[string][]Lexeme{}
	for _, l := range lexemes {
		for _, v := range NormalizeLemmaVariants(l.Lemma) {
			byLemma[strings.ToLower(v)] = append(byLemma[strings.ToLower(v)], l)
		}
	}
	kaikkiID := in.SourceIDs["kaikki"]

	var out Output
	rep := Report{Levels: map[string]LevelStats{}}
	type built struct {
		lex    Lexeme
		chosen []string
		ipa    string
	}
	var results []built
	counts := map[int64]int64{}

	for _, lex := range lexemes {
		variants := NormalizeLemmaVariants(lex.Lemma)
		var matched []Entry
		for _, v := range variants {
			for _, p := range KaikkiPos(lex.Pos) {
				matched = append(matched, byKey[posKey{v, p}]...)
			}
		}

		// Значения и кандидаты-переводы.
		var cands []Candidate
		var sounds []Sound
		synSeen := map[string]bool{}
		var rels []RelationRow
		senseNo := 0
		for _, e := range matched {
			var kept []keptSense
			local := make([]int, len(e.Senses)) // номер значения записи → общий номер (0 — значение без пояснения)
			for i, s := range e.Senses {
				if len(s.Glosses) == 0 {
					continue
				}
				senseNo++
				local[i] = senseNo
				kept = append(kept, keptSense{no: senseNo, gloss: s.Glosses, tags: s.Tags})
				out.Senses = append(out.Senses, SenseRow{
					LexemeID: lex.ID, SenseNo: senseNo, GlossEN: strings.Join(s.Glosses, "; "), Tags: nonNil(s.Tags), Flags: nonNil(flagsOf(s.Tags)), SourceID: kaikkiID,
				})
			}
			for i, s := range e.Senses {
				for _, t := range s.Translations {
					if t.Code == "ru" {
						cands = append(cands, Candidate{Text: t.Word, Source: "kaikki", SenseNo: local[i], Tags: unionTags(t.Tags, s.Tags)})
					}
				}
			}
			for _, t := range e.Translations {
				if t.Code != "ru" {
					continue
				}
				no, senseTags := matchSense(kept, t.Sense)
				cands = append(cands, Candidate{Text: t.Word, Source: "kaikki", SenseNo: no, Tags: unionTags(t.Tags, senseTags)})
			}
			sounds = append(sounds, e.Sounds...)
			for _, kind := range []struct {
				name  string
				words []string
			}{{"synonym", e.Synonyms}, {"antonym", e.Antonyms}} {
				for _, w := range kind.words {
					k := kind.name + "/" + strings.ToLower(w)
					if synSeen[k] || slices.Contains(variants, w) {
						continue
					}
					synSeen[k] = true
					rels = append(rels, RelationRow{LexemeID: lex.ID, Kind: kind.name, OtherLemma: w, OtherLexemeID: resolve(byLemma, w, lex)})
				}
			}
		}
		if lp := LegacyPos(lex.Pos); lp != "" {
			for _, v := range variants {
				for _, l := range legacy[posKey{v, lp}] {
					cands = append(cands, Candidate{Text: l.Ru, Source: "freedict", SenseNo: l.SenseNo})
				}
			}
		}

		scored := ScoreTranslations(cands, in.RuRank)
		picked := PickTranslations(scored, maxChosen)
		pickedText := map[string]bool{}
		var chosenTexts []string
		for i, p := range picked {
			pickedText[p.Text] = true
			chosenTexts = append(chosenTexts, p.Text)
			out.Translations = append(out.Translations, translationRow(lex.ID, p, i+1, true, in.SourceIDs))
			if len(p.Flags) > 0 {
				rep.Flagged++
			}
		}
		rank := len(picked)
		for _, s := range scored {
			if pickedText[s.Text] || rank >= maxStoredTranslations {
				continue
			}
			rank++
			out.Translations = append(out.Translations, translationRow(lex.ID, s, rank, false, in.SourceIDs))
		}

		prons := PickPronunciations(sounds)
		ipa, audio := "", ""
		for _, p := range prons {
			out.Pronunciations = append(out.Pronunciations, PronRow{LexemeID: lex.ID, Accent: p.Accent, IPA: p.IPA, Audio: p.Audio, Primary: p.Primary, SourceID: kaikkiID})
			if p.Primary {
				ipa, audio = p.IPA, p.Audio
			}
		}

		// Формы: форма слова → слово; сами варианты леммы тоже формы.
		formSeen := map[string]bool{}
		var cardForms []string
		addForm := func(form, tag string) {
			form = strings.ToLower(strings.TrimSpace(form))
			if form == "" || form == "-" || strings.Contains(form, " ") || formSeen[form+"/"+tag] {
				return
			}
			formSeen[form+"/"+tag] = true
			out.Forms = append(out.Forms, FormRow{Form: form, LexemeID: lex.ID, Tag: tag})
			if tag != "lemma" && !slices.Contains(cardForms, form) && !slices.Contains(variants, form) && len(cardForms) < maxCardForms {
				cardForms = append(cardForms, form)
			}
		}
		for _, v := range variants {
			addForm(v, "lemma")
		}
		for _, e := range matched {
			for _, f := range e.Forms {
				if slices.Contains(f.Tags, "table-tags") || slices.Contains(f.Tags, "inflection-template") || slices.Contains(f.Tags, "class") {
					continue
				}
				addForm(f.Form, strings.Join(f.Tags, ","))
			}
		}
		counted := map[string]bool{}
		for _, f := range out.Forms {
			if f.LexemeID == lex.ID && !counted[f.Form] {
				counted[f.Form] = true
				counts[lex.ID] += in.EnCount[f.Form]
			}
		}

		// Связи слов (до maxRelations каждого вида).
		perKind := map[string]int{}
		for _, r := range rels {
			if perKind[r.Kind] < maxRelations {
				perKind[r.Kind]++
				out.Relations = append(out.Relations, r)
			}
		}

		results = append(results, built{lex: lex, chosen: chosenTexts, ipa: ipa})
		if len(picked) > 0 || len(prons) > 0 {
			var syn []string
			for _, r := range rels {
				if r.Kind == "synonym" && len(syn) < 5 {
					syn = append(syn, r.OtherLemma)
				}
			}
			out.Cards = append(out.Cards, CardRow{LexemeID: lex.ID, Payload: mustJSON(cardPayload{
				Lemma: lex.Lemma, Pos: lex.Pos, Level: lex.Level, Translations: nonNil(chosenTexts), IPA: ipa, Audio: audio, Forms: cardForms, Synonyms: syn,
			})})
		}

		ls := rep.Levels[lex.Level]
		ls.Total++
		if len(picked) > 0 {
			ls.WithTranslation++
		}
		if len(prons) > 0 {
			ls.WithIPA++
		}
		rep.Levels[lex.Level] = ls
	}

	// Ранги частоты по сумме форм; слова без употреблений ранга не имеют.
	order := slices.Clone(lexemes)
	slices.SortFunc(order, func(a, b Lexeme) int {
		if counts[a.ID] != counts[b.ID] {
			if counts[a.ID] > counts[b.ID] {
				return -1
			}
			return 1
		}
		return int(a.ID - b.ID)
	})
	ranks := map[int64]int{}
	for _, l := range order {
		if counts[l.ID] > 0 {
			ranks[l.ID] = len(ranks) + 1
		}
	}
	for _, l := range lexemes {
		row := StatRow{LexemeID: l.ID, Count: counts[l.ID]}
		if r, ok := ranks[l.ID]; ok {
			row.Rank = &r
		}
		out.Stats = append(out.Stats, row)
	}
	for i, c := range out.Cards {
		var p cardPayload
		if err := json.Unmarshal(c.Payload, &p); err == nil {
			if r, ok := ranks[c.LexemeID]; ok {
				p.Rank = &r
				out.Cards[i].Payload = mustJSON(p)
			}
		}
	}

	// Отчёт: контрольный список и выборка.
	var withTr []built
	for _, r := range results {
		if len(r.chosen) > 0 {
			withTr = append(withTr, r)
		}
	}
	for _, g := range Golden {
		for _, r := range results {
			if r.lex.Pos != g.Pos || !slices.Contains(NormalizeLemmaVariants(r.lex.Lemma), g.Lemma) {
				continue
			}
			rep.Golden.Checked++
			got := ""
			if len(r.chosen) > 0 {
				got = r.chosen[0]
			}
			if slices.Contains(g.Accept, got) {
				rep.Golden.Passed++
			} else {
				rep.Golden.Failed = append(rep.Golden.Failed, GoldenFail{Lemma: g.Lemma, Pos: g.Pos, Got: got})
			}
			break
		}
	}
	step := max(1, (len(withTr)+sampleSize-1)/sampleSize)
	for i := 0; i < len(withTr); i += step {
		r := withTr[i]
		rep.Sample = append(rep.Sample, SampleRow{Lemma: r.lex.Lemma, Pos: r.lex.Pos, Translations: r.chosen, IPA: r.ipa})
	}
	return out, rep
}

func translationRow(lexID int64, s Scored, rank int, chosen bool, ids map[string]int64) TranslationRow {
	var sense *int
	if s.KaikkiSense > 0 {
		n := s.KaikkiSense
		sense = &n
	}
	return TranslationRow{
		LexemeID: lexID, SenseNo: sense, Lang: "ru", Text: s.Text, Score: s.Score, Rank: rank, Chosen: chosen,
		Flags: nonNil(s.Flags), SourceIDs: nonNilIDs(sourceIDsOf(s.Sources, ids)),
	}
}

// resolve находит слово из lexemes по лемме синонима: та же часть речи предпочтительнее, иначе наименьший номер.
func resolve(byLemma map[string][]Lexeme, word string, from Lexeme) *int64 {
	cands := byLemma[strings.ToLower(word)]
	if len(cands) == 0 {
		return nil
	}
	best := cands[0]
	for _, c := range cands {
		if c.Pos == from.Pos {
			best = c
			break
		}
	}
	if best.ID == from.ID {
		return nil
	}
	id := best.ID
	return &id
}

func firstGloss(e Entry) string {
	if len(e.Senses) > 0 && len(e.Senses[0].Glosses) > 0 {
		return e.Senses[0].Glosses[0]
	}
	return ""
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nonNilIDs(in []int64) []int64 {
	if in == nil {
		return []int64{}
	}
	return in
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // структуры из строк и чисел не маршалятся с ошибкой
	}
	return b
}
