// Package constructions находит в предложениях типичные грамматические конструкции по словам и знакам. Правила
// простые и осторожные: спорное пропускаем (лучше не показать пояснение, чем показать не к месту). Функции чистые.
package constructions

import (
	"slices"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/grammar"
)

// Идентификаторы конструкций.
const (
	PresentPerfect    = "present_perfect"
	PastSimple        = "past_simple"
	Modals            = "modals"
	ThereIs           = "there_is"
	Comparative       = "comparative"
	FirstConditional  = "first_conditional"
	SecondConditional = "second_conditional"
	QuestionDo        = "question_do"
)

// Order — порядок показа конструкций.
var Order = []string{PresentPerfect, PastSimple, Modals, ThereIs, Comparative, FirstConditional, SecondConditional, QuestionDo}

// Sentence — предложение: исходный текст (знаки нужны для вопроса) и слова в исходном регистре.
type Sentence struct {
	Text   string
	Tokens []string
}

// Match — конструкция и номера предложений, где она найдена.
type Match struct {
	ID        string
	Sentences []int
}

// irregularParticiples — частые неправильные причастия (третья форма). «got» не входит: have got — не Present Perfect.
var irregularParticiples = map[string]bool{
	"been": true, "had": true, "done": true, "said": true, "gone": true, "made": true, "known": true, "thought": true,
	"taken": true, "seen": true, "come": true, "given": true, "found": true, "told": true, "become": true, "left": true,
	"felt": true, "put": true, "brought": true, "begun": true, "kept": true, "held": true, "written": true, "stood": true,
	"heard": true, "let": true, "meant": true, "set": true, "met": true, "run": true, "paid": true, "sat": true, "spoken": true,
	"led": true, "read": true, "grown": true, "lost": true, "fallen": true, "sent": true, "built": true, "understood": true,
	"drawn": true, "broken": true, "spent": true, "cut": true, "driven": true, "bought": true, "worn": true, "chosen": true,
	"eaten": true, "forgotten": true, "won": true, "flown": true, "sung": true, "swum": true, "shown": true,
}

var perfectAux = []string{"have", "has", "i've", "you've", "we've", "they've", "he's", "she's", "it's", "haven't", "hasn't"}

// perfectGap — слова, которые могут стоять между have и причастием.
var perfectGap = []string{"not", "never", "already", "just", "ever", "yet", "still", "also"}

// notComparative — слова на -er, после которых «than» не значит сравнение.
var notComparative = []string{"other", "rather", "another", "whether", "either", "neither", "never", "ever", "after", "over",
	"under", "water", "father", "mother", "brother", "sister", "number", "summer", "winter", "computer", "teacher", "answer",
	"paper", "together", "wonder", "order", "river", "dinner", "letter", "super", "power", "flower", "tower", "member"}

func lowerAll(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		out[i] = strings.ToLower(strings.ReplaceAll(t, "’", "'"))
	}
	return out
}

func isParticiple(w string) bool {
	return irregularParticiples[w] || (len(w) > 4 && strings.HasSuffix(w, "ed"))
}

func presentPerfect(l []string) bool {
	for i, w := range l {
		if !slices.Contains(perfectAux, w) {
			continue
		}
		// «he's» и «it's» значат is/has: берём только если дальше стоит однозначное причастие (been, gone…), не -ed.
		ambiguous := w == "he's" || w == "she's" || w == "it's"
		j := i + 1
		for j < len(l) && slices.Contains(perfectGap, l[j]) {
			j++
		}
		if j >= len(l) {
			continue
		}
		p := l[j]
		if ambiguous && !irregularParticiples[p] {
			continue
		}
		if isParticiple(p) {
			return true
		}
	}
	return false
}

func comparative(l []string) bool {
	for i, w := range l {
		if w != "than" || i == 0 {
			continue
		}
		prev := l[i-1]
		switch {
		case slices.Contains([]string{"more", "less", "better", "worse", "farther", "further"}, prev):
			return true
		case len(prev) > 3 && strings.HasSuffix(prev, "er") && !slices.Contains(notComparative, prev):
			return true
		case i >= 2 && slices.Contains([]string{"more", "less"}, l[i-2]):
			return true
		}
	}
	return false
}

func conditional(l []string, main []string) bool {
	ifAt := slices.Index(l, "if")
	if ifAt < 0 {
		return false
	}
	for _, w := range l[ifAt+1:] {
		if slices.Contains(main, w) {
			return true
		}
	}
	return false
}

func thereIs(l []string) bool {
	if len(l) < 2 {
		return false
	}
	if l[0] == "there's" {
		return true
	}
	return l[0] == "there" && slices.Contains([]string{"is", "are", "was", "were"}, l[1])
}

func questionDo(text string, l []string) bool {
	return len(l) > 1 && slices.Contains([]string{"do", "does", "did"}, l[0]) && strings.HasSuffix(strings.TrimSpace(text), "?")
}

// Has сообщает, есть ли конструкция id в предложении.
func Has(id string, s Sentence) bool {
	l := lowerAll(s.Tokens)
	switch id {
	case PresentPerfect:
		return presentPerfect(l)
	case PastSimple:
		return exercises.PastSimpleTarget(s.Tokens) != "" && !presentPerfect(l)
	case Modals:
		return slices.Contains(grammar.Topics(s.Tokens), grammar.Modals)
	case ThereIs:
		return thereIs(l)
	case Comparative:
		return comparative(l)
	case FirstConditional:
		return conditional(l, []string{"will", "won't", "i'll", "you'll", "we'll", "they'll", "he'll", "she'll"})
	case SecondConditional:
		return conditional(l, []string{"would", "wouldn't"})
	case QuestionDo:
		return questionDo(s.Text, l)
	}
	return false
}

// Detect возвращает найденные конструкции в порядке Order с номерами предложений.
func Detect(sentences []Sentence) []Match {
	var out []Match
	for _, id := range Order {
		var idx []int
		for i, s := range sentences {
			if Has(id, s) {
				idx = append(idx, i)
			}
		}
		if len(idx) > 0 {
			out = append(out, Match{ID: id, Sentences: idx})
		}
	}
	return out
}
