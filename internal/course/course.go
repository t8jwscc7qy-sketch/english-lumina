// Package course — курс уроков «Путь»: формат урока, строгая проверка словаря и сборка заданий закрепления.
//
// Уроки пишутся заранее (content/course/*.json). Главное правило: в тексте урока только слова, введённые в этом и
// прошлых уроках (лемма и явно перечисленные формы), и имена героев. Так понимание текста обеспечено построением,
// а не частотным списком.
package course

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
)

// Ограничения урока.
const (
	MaxNewWords  = 6
	shortLessons = 3 // первые уроки курса короткие: известных слов ещё почти нет
	shortMin     = 15
	shortMax     = 30
	normalMin    = 30
	normalMax    = 150
	minQuestions = 1
	maxQuestions = 3
)

var lessonID = regexp.MustCompile(`^u[0-9]+-l[0-9]+$`)

// Word — новое слово урока. Lemma и Pos совпадают со словарём (lexemes), чтобы урок мог создать карточку.
// Forms — формы, которые тоже считаются этим словом в тексте (I'm, names). Ru — перевод для ученика.
type Word struct {
	Lemma string   `json:"lemma"`
	Pos   string   `json:"pos"`
	Ru    string   `json:"ru"`
	Forms []string `json:"forms,omitempty"`
}

// Sentence — строка текста. Speaker — кто говорит (в диалоге), не входит в словарь.
type Sentence struct {
	Speaker string `json:"speaker,omitempty"`
	En      string `json:"en"`
	Ru      string `json:"ru"`
}

// Question — вопрос «понял ли» на русском, варианты и номер верного.
type Question struct {
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
	Answer  int      `json:"answer"`
}

// Note — короткое пояснение грамматики, встреченной в уроке.
type Note struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Lesson — один урок.
type Lesson struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Words     []Word     `json:"words"`
	Names     []string   `json:"names,omitempty"`
	Sentences []Sentence `json:"sentences"`
	Questions []Question `json:"questions"`
	Note      *Note      `json:"note,omitempty"`
}

// Unit — блок уроков на одну тему.
type Unit struct {
	ID      int      `json:"unit"`
	Title   string   `json:"title"`
	Lessons []Lesson `json:"lessons"`
}

// Course — весь курс по порядку прохождения.
type Course struct {
	Units []Unit
}

// Ordered — уроки курса подряд.
func (c Course) Ordered() []Lesson {
	var out []Lesson
	for _, u := range c.Units {
		out = append(out, u.Lessons...)
	}
	return out
}

// Find возвращает урок, его блок и номер в общем порядке (-1, если урока нет).
func (c Course) Find(id string) (Lesson, Unit, int) {
	i := 0
	for _, u := range c.Units {
		for _, l := range u.Lessons {
			if l.ID == id {
				return l, u, i
			}
			i++
		}
	}
	return Lesson{}, Unit{}, -1
}

// Tokens — слова строки в исходном регистре.
func (s Sentence) Tokens() []string { return exercises.CasedTokens(s.En) }

// WordCount — число слов текста урока.
func (l Lesson) WordCount() int {
	n := 0
	for _, s := range l.Sentences {
		n += len(s.Tokens())
	}
	return n
}

func lower(t string) string { return strings.ToLower(strings.ReplaceAll(t, "’", "'")) }

// forms — все формы слова в нижнем регистре.
func (w Word) forms() []string {
	out := []string{lower(w.Lemma)}
	for _, f := range w.Forms {
		out = append(out, lower(f))
	}
	return out
}

// Load читает course/*.json (один файл — один блок), проверяет каждый урок и весь курс. Порядок — по номеру блока.
func Load(fsys fs.FS) (Course, error) {
	files, err := fs.Glob(fsys, "course/*.json")
	if err != nil {
		return Course{}, err
	}
	var c Course
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return Course{}, err
		}
		var u Unit
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&u); err != nil {
			return Course{}, fmt.Errorf("%s: разбор: %w", f, err)
		}
		c.Units = append(c.Units, u)
	}
	sort.Slice(c.Units, func(i, j int) bool { return c.Units[i].ID < c.Units[j].ID })
	if err := c.Validate(); err != nil {
		return Course{}, err
	}
	return c, nil
}

// Validate проверяет весь курс: уникальность, структуру уроков и главное правило словаря.
func (c Course) Validate() error {
	if len(c.Units) == 0 {
		return errors.New("в курсе нет блоков")
	}
	ids := map[string]bool{}
	seenWord := map[string]string{}
	known := map[string]bool{} // формы слов, введённых в прошлых уроках
	n := 0
	for ui, u := range c.Units {
		if u.ID != ui+1 {
			return fmt.Errorf("блоки должны идти подряд с 1, а встретился %d на месте %d", u.ID, ui+1)
		}
		if strings.TrimSpace(u.Title) == "" || len(u.Lessons) == 0 {
			return fmt.Errorf("блок %d: нужно название и хотя бы один урок", u.ID)
		}
		for _, l := range u.Lessons {
			if err := l.validate(n); err != nil {
				return fmt.Errorf("урок %q: %w", l.ID, err)
			}
			if ids[l.ID] {
				return fmt.Errorf("урок %q повторяется", l.ID)
			}
			ids[l.ID] = true
			for _, w := range l.Words {
				key := lower(w.Lemma) + "/" + w.Pos
				if prev, dup := seenWord[key]; dup {
					return fmt.Errorf("урок %q: слово %s уже введено в уроке %q", l.ID, key, prev)
				}
				seenWord[key] = l.ID
			}
			if probs := l.CheckVocabulary(known); len(probs) > 0 {
				return fmt.Errorf("урок %q: слова, которых ещё не было: %s", l.ID, strings.Join(probs, ", "))
			}
			for _, w := range l.Words {
				for _, f := range w.forms() {
					known[f] = true
				}
			}
			n++
		}
	}
	return nil
}

// validate — структура одного урока; index — номер урока в курсе с нуля (от него зависит длина текста).
func (l Lesson) validate(index int) error {
	switch {
	case !lessonID.MatchString(l.ID):
		return errors.New("номер урока должен быть вида u1-l1")
	case strings.TrimSpace(l.Title) == "":
		return errors.New("нет названия")
	case len(l.Words) < 1 || len(l.Words) > MaxNewWords:
		return fmt.Errorf("новых слов от 1 до %d, а их %d", MaxNewWords, len(l.Words))
	case len(l.Questions) < minQuestions || len(l.Questions) > maxQuestions:
		return fmt.Errorf("вопросов от %d до %d", minQuestions, maxQuestions)
	}
	lo, hi := normalMin, normalMax
	if index < shortLessons {
		lo, hi = shortMin, shortMax
	}
	if wc := l.WordCount(); wc < lo || wc > hi {
		return fmt.Errorf("слов в тексте %d, нужно от %d до %d", wc, lo, hi)
	}
	for _, w := range l.Words {
		if w.Lemma == "" || w.Pos == "" || strings.TrimSpace(w.Ru) == "" {
			return errors.New("у нового слова нужны lemma, pos и ru")
		}
		if !l.uses(w) {
			return fmt.Errorf("новое слово %q не встречается в тексте", w.Lemma)
		}
	}
	for i, s := range l.Sentences {
		if strings.TrimSpace(s.En) == "" || strings.TrimSpace(s.Ru) == "" {
			return fmt.Errorf("строка %d: нужен английский текст и перевод", i+1)
		}
	}
	for i, q := range l.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || len(q.Options) > 4 || q.Answer < 0 || q.Answer >= len(q.Options) {
			return fmt.Errorf("вопрос %d: нужен текст, 2–4 варианта и номер верного", i+1)
		}
	}
	if l.Note != nil && (strings.TrimSpace(l.Note.Title) == "" || strings.TrimSpace(l.Note.Text) == "") {
		return errors.New("у пояснения нужны заголовок и текст")
	}
	return nil
}

// uses — слово встречается в тексте хотя бы одной своей формой.
func (l Lesson) uses(w Word) bool {
	fs := map[string]bool{}
	for _, f := range w.forms() {
		fs[f] = true
	}
	for _, s := range l.Sentences {
		for _, t := range s.Tokens() {
			if fs[lower(t)] {
				return true
			}
		}
	}
	return false
}

// isName — токен совпадает с именем героя (в том числе с притяжательным 's).
func (l Lesson) isName(tok string) bool {
	base := strings.TrimSuffix(strings.TrimSuffix(tok, "'s"), "’s")
	for _, n := range l.Names {
		if tok == n || base == n {
			return true
		}
	}
	return false
}

// CheckVocabulary возвращает слова текста, которых нет среди known (прошлые уроки), новых слов урока и имён. Числа
// пропускаются. Строгая проверка: формы слов не выводятся правилами, каждая форма перечисляется в уроке.
func (l Lesson) CheckVocabulary(known map[string]bool) []string {
	allowed := map[string]bool{}
	for k := range known {
		allowed[k] = true
	}
	for _, w := range l.Words {
		for _, f := range w.forms() {
			allowed[f] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, s := range l.Sentences {
		for _, t := range s.Tokens() {
			r := []rune(t)
			if len(r) == 0 || unicode.IsDigit(r[0]) || l.isName(t) || allowed[lower(t)] {
				continue
			}
			if w := lower(t); !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}
