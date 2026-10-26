// Package kb собирает базу знаний из внешних источников: разбор записей Kaikki (Wiktextract), правила слияния
// переводов, произношения и вариантов лемм. Чистые функции без обращения к базе и сети.
package kb

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// Translation — перевод слова на другой язык: Code — код языка Викисловаря (ru), Sense — значение словами.
type Translation struct {
	Code  string
	Word  string
	Sense string
	Tags  []string
}

// Sense — значение слова.
type Sense struct {
	Glosses      []string
	Tags         []string
	Translations []Translation
}

// Sound — произношение: IPA и/или имя звукового файла Wikimedia Commons; Tags отличают акценты (US, UK, …).
type Sound struct {
	IPA   string
	Audio string
	Tags  []string
}

// Form — форма слова (plural, past, …).
type Form struct {
	Form string
	Tags []string
}

// Entry — запись Kaikki об одном слове и части речи.
type Entry struct {
	Word         string
	Pos          string
	LangCode     string
	EtymNo       int // номер статьи по этимологии (у слов с несколькими статьями); 0 — одна статья
	Senses       []Sense
	Sounds       []Sound
	Forms        []Form
	Translations []Translation
	Synonyms     []string
	Antonyms     []string
}

// flexInt читает целое из числа или из строки («2»); всё остальное даёт 0 (поле необязательное и в данных непостоянное).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if n, err := strconv.Atoi(s); err == nil {
			*f = flexInt(n)
			return nil
		}
	}
	*f = 0
	return nil
}

type rawLink struct {
	Word string   `json:"word"`
	Tags []string `json:"tags"`
}

// skipLinkTags — пометки связей (синонимов и антонимов), с которыми слово не берётся: грубое, просторечное, устаревшее.
var skipLinkTags = []string{"vulgar", "offensive", "derogatory", "slang", "archaic", "obsolete", "dated", "dialectal", "nonstandard", "rare"}

type rawTranslation struct {
	Code  string   `json:"code"`
	Word  string   `json:"word"`
	Sense string   `json:"sense"`
	Tags  []string `json:"tags"`
}

type rawSense struct {
	Glosses      []string         `json:"glosses"`
	RawGlosses   []string         `json:"raw_glosses"`
	Tags         []string         `json:"tags"`
	Translations []rawTranslation `json:"translations"`
	Synonyms     []rawLink        `json:"synonyms"`
	Antonyms     []rawLink        `json:"antonyms"`
}

type rawSound struct {
	IPA   string   `json:"ipa"`
	Audio string   `json:"audio"`
	Tags  []string `json:"tags"`
}

type rawForm struct {
	Form string   `json:"form"`
	Tags []string `json:"tags"`
}

type rawEntry struct {
	Word         *string          `json:"word"`
	Pos          string           `json:"pos"`
	LangCode     string           `json:"lang_code"`
	EtymNo       flexInt          `json:"etymology_number"`
	Senses       []rawSense       `json:"senses"`
	Sounds       []rawSound       `json:"sounds"`
	Forms        []rawForm        `json:"forms"`
	Translations []rawTranslation `json:"translations"`
	Synonyms     []rawLink        `json:"synonyms"`
	Antonyms     []rawLink        `json:"antonyms"`
}

func toTranslations(in []rawTranslation) []Translation {
	var out []Translation
	for _, t := range in {
		if t.Word == "" {
			continue // у записи может быть только пояснение без слова
		}
		out = append(out, Translation(t))
	}
	return out
}

func appendLinks(dst []string, in []rawLink) []string {
	for _, l := range in {
		if l.Word == "" || slices.ContainsFunc(l.Tags, func(t string) bool { return slices.Contains(skipLinkTags, strings.ToLower(t)) }) {
			continue
		}
		dst = append(dst, l.Word)
	}
	return dst
}

// ParseEntry разбирает одну строку JSONL Kaikki. Неизвестные поля игнорируются; нет слова или строка не JSON — ошибка.
// Синонимы и антонимы собираются и с уровня записи, и из значений, без повторов.
func ParseEntry(line []byte) (Entry, error) {
	var r rawEntry
	if err := json.Unmarshal(line, &r); err != nil {
		return Entry{}, err
	}
	if r.Word == nil {
		return Entry{}, errors.New("в записи нет поля word")
	}
	e := Entry{Word: *r.Word, Pos: r.Pos, LangCode: r.LangCode, EtymNo: int(r.EtymNo)}
	for _, s := range r.Senses {
		glosses := s.Glosses
		if len(glosses) == 0 {
			glosses = s.RawGlosses // у части записей чистых значений нет
		}
		e.Senses = append(e.Senses, Sense{Glosses: glosses, Tags: s.Tags, Translations: toTranslations(s.Translations)})
		if !slices.ContainsFunc(s.Tags, func(t string) bool { return slices.Contains(skipLinkTags, strings.ToLower(t)) }) {
			// связи значений с пометкой «грубое», «просторечное», «устаревшее» не берём: «suck» у «blow» не синоним
			e.Synonyms = appendLinks(e.Synonyms, s.Synonyms)
			e.Antonyms = appendLinks(e.Antonyms, s.Antonyms)
		}
	}
	for _, s := range r.Sounds {
		e.Sounds = append(e.Sounds, Sound(s))
	}
	for _, f := range r.Forms {
		e.Forms = append(e.Forms, Form(f))
	}
	e.Translations = toTranslations(r.Translations)
	e.Synonyms = dedupe(appendLinks(e.Synonyms, r.Synonyms))
	e.Antonyms = dedupe(appendLinks(e.Antonyms, r.Antonyms))
	return e, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

var (
	langSpaced  = []byte(`"lang_code": "en"`)
	langCompact = []byte(`"lang_code":"en"`)
)

// IsEnglishLine — быстрая проверка без разбора: в строке есть пометка английского языка (при любом стиле пробелов).
func IsEnglishLine(line []byte) bool {
	return bytes.Contains(line, langSpaced) || bytes.Contains(line, langCompact)
}

// Wanted — быстрая проверка строки до полного разбора: язык английский (при любом стиле пробелов в JSON) и слово
// записи есть среди лемм. Битая строка не отбирается.
func Wanted(line []byte, lemmas map[string]bool) bool {
	if !IsEnglishLine(line) {
		return false
	}
	var probe struct {
		Word     string `json:"word"`
		LangCode string `json:"lang_code"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return false
	}
	return probe.LangCode == "en" && lemmas[probe.Word]
}
