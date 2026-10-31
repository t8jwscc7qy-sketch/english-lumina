package library

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/words"
)

// StyleIssue — типичная ошибка неносителя в тексте: правило, предложение (с единицы) и пояснение.
type StyleIssue struct {
	Rule     string
	Sentence int
	Detail   string
}

func (s StyleIssue) String() string {
	return fmt.Sprintf("[%s, предл. %d: %s]", s.Rule, s.Sentence, s.Detail)
}

var (
	canNotRe       = regexp.MustCompile(`(?i)\bcan not\b`)
	noApostropheRe = regexp.MustCompile(`(?i)\b(dont|cant|wont|isnt|arent|wasnt|werent|doesnt|didnt|im|ive|youre|theyre|thats|whats)\b`)
	endRe          = regexp.MustCompile(`[.!?…]["”']?$`)
	quotedRe       = regexp.MustCompile(`"[^"]*"|“[^”]*”`)
)

// Слова, перед которыми a, а не an (звук [j] или [w]), и слова с немой h, перед которыми an.
var (
	aBeforeVowelLetter = []string{"university", "uniform", "union", "unit", "user", "usual", "useful", "european", "one", "unique", "utensil"}
	anBeforeConsonant  = []string{"hour", "honest", "honor", "honour", "heir"}
)

var determiners = []string{"a", "an", "the", "my", "your", "his", "her", "its", "our", "their", "this", "that", "these", "those",
	"some", "any", "no", "one", "two", "three", "every", "each", "another", "more", "much", "many", "all", "both", "next", "last", "happy"}

var prepositions = []string{"to", "at", "in", "on", "by", "for", "from", "with", "of", "into", "after", "before", "about", "near", "like"}

// Причастия-прилагательные на -ed и другие слова на -ed, которые не форма прошедшего времени.
var notPastED = []string{"tired", "worried", "scared", "excited", "interested", "bored", "surprised", "married", "crowded", "wicked", "naked",
	"hundred", "used", "red", "bed", "need", "feed", "seed", "speed", "weed", "proceed", "indeed", "breed", "agreed", "pleased", "pretty",
	"hungry", "tired", "ready", "closed", "broken", "lost", "scared", "puzzled", "embarrassed", "confused", "relaxed", "shocked", "annoyed"}

// Формы настоящего времени 3-го лица и вспомогательные: признак повествования в настоящем.
var presentVerbs = []string{"is", "are", "am", "has", "does", "goes", "says", "makes", "takes", "comes", "sees", "knows", "gets", "gives",
	"finds", "thinks", "tells", "feels", "puts", "brings", "keeps", "holds", "writes", "stands", "hears", "lets", "means", "meets", "runs",
	"pays", "sits", "speaks", "leaves", "wants", "likes", "looks", "needs", "loves", "walks", "plays", "helps", "lives", "works", "calls",
	"asks", "tries", "starts", "opens", "watches", "waits", "eats", "drinks", "laughs", "smiles", "buys", "wears", "reads", "turns", "sings",
	"flies", "swims", "sleeps", "wins", "catches", "hurries", "answers", "kicks", "scores", "studies", "passes", "practices", "whispers"}

var pastIrregular = []string{"was", "were", "had", "did", "went", "saw", "took", "came", "got", "gave", "found", "knew", "thought", "told",
	"felt", "brought", "kept", "held", "wrote", "stood", "heard", "met", "ran", "paid", "sat", "spoke", "said", "made", "ate", "drank",
	"bought", "wore", "sang", "flew", "swam", "slept", "won", "caught", "began", "fell", "forgot"}

func lowerWord(t string) string { return strings.ToLower(strings.ReplaceAll(t, "’", "'")) }

func isPastWord(w string) bool {
	if slices.Contains(pastIrregular, w) || words.IrregularPast(w) {
		return true
	}
	return len(w) > 4 && strings.HasSuffix(w, "ed") && !slices.Contains(notPastED, w)
}

// narration — предложение без прямой речи (то, что в кавычках, вырезано).
func narration(en string) string { return strings.TrimSpace(quotedRe.ReplaceAllString(en, " ")) }

// CheckStyle ищет типичные ошибки неносителя: «can not» вместо can't/cannot, неверные a/an, пропущенные апострофы и
// точки, повествование со смешением прошедшего и настоящего, пропущенный артикль перед исчисляемым существительным
// (целевые слова-существительные без пометки mass). Это эвристики: они ловят частые ошибки, но не заменяют вычитку.
func (t Text) CheckStyle() []StyleIssue {
	var out []StyleIssue
	add := func(rule string, i int, detail string) { out = append(out, StyleIssue{rule, i + 1, detail}) }
	var past, present []int
	for i, s := range t.Sentences {
		en := strings.TrimSpace(s.En)
		if canNotRe.MatchString(en) {
			add("can not", i, "пиши can't или cannot")
		}
		if m := noApostropheRe.FindString(en); m != "" {
			add("апостроф", i, fmt.Sprintf("в %q нужен апостроф", m))
		}
		if !endRe.MatchString(en) {
			add("знак в конце", i, "предложение должно кончаться . ! или ?")
		}
		if r := []rune(strings.TrimLeft(en, `"“'`)); len(r) > 0 && !unicode.IsUpper(r[0]) && !unicode.IsDigit(r[0]) {
			add("заглавная", i, "предложение начинается со строчной буквы")
		}
		toks := s.Tokens()
		for j := 0; j+1 < len(toks); j++ {
			a, next := lowerWord(toks[j]), lowerWord(toks[j+1])
			vowel := strings.ContainsRune("aeiou", rune(next[0]))
			switch {
			case a == "a" && vowel && !slices.Contains(aBeforeVowelLetter, next):
				add("a/an", i, fmt.Sprintf("«a %s»: перед гласной нужно an", next))
			case a == "an" && !vowel && !slices.Contains(anBeforeConsonant, next):
				add("a/an", i, fmt.Sprintf("«an %s»: перед согласной нужно a", next))
			case a == "an" && vowel && slices.Contains(aBeforeVowelLetter, next):
				add("a/an", i, fmt.Sprintf("«an %s»: слово начинается со звука [j] или [w], нужно a", next))
			}
		}
		// Время повествования (вне прямой речи).
		hasPast, hasPresent := false, false
		for _, tok := range strings.Fields(strings.NewReplacer(".", " ", ",", " ", "!", " ", "?", " ", ";", " ").Replace(narration(en))) {
			w := lowerWord(strings.Trim(tok, `"“”'`))
			hasPast = hasPast || isPastWord(w)
			hasPresent = hasPresent || slices.Contains(presentVerbs, w)
		}
		switch {
		case hasPast && !hasPresent:
			past = append(past, i)
		case hasPresent && !hasPast:
			present = append(present, i)
		}
	}
	if len(past) > 0 && len(present) > 0 {
		add("время", present[0], fmt.Sprintf("повествование смешивает прошедшее (предл. %d) и настоящее (предл. %d)", past[0]+1, present[0]+1))
	}
	// Артикль перед исчисляемым существительным-целью.
	for _, tg := range t.Targets {
		if tg.Pos != "noun" || tg.Mass {
			continue
		}
		for i, s := range t.Sentences {
			toks := s.Tokens()
			for j, tok := range toks {
				if lowerWord(tok) != tg.Lemma || isName(tok, j) {
					continue
				}
				if !hasDeterminer(toks, j) && !vocativeOrExclamation(s.En, tok) {
					add("артикль", i, fmt.Sprintf("перед исчисляемым %q нет артикля или притяжательного", tg.Lemma))
				}
			}
		}
	}
	return filterAllowed(out, t.Allow)
}

// filterAllowed убирает замечания, которые автор явно разрешил полями вида «артикль:6» (правило и номер предложения).
func filterAllowed(issues []StyleIssue, allow []string) []StyleIssue {
	if len(allow) == 0 {
		return issues
	}
	var out []StyleIssue
	for _, is := range issues {
		if !slices.Contains(allow, fmt.Sprintf("%s:%d", is.Rule, is.Sentence)) {
			out = append(out, is)
		}
	}
	return out
}

// vocativeOrExclamation — слово стоит в обращении или восклицании («Mother, ...», «..., boss.», «Goal!»): артикль там не нужен.
func vocativeOrExclamation(en, tok string) bool {
	re := regexp.MustCompile(`(?i)(^|[,"“]\s*)` + regexp.QuoteMeta(tok) + `\s*[,!.?"”]`)
	return re.MatchString(en)
}

// hasDeterminer — перед существительным toks[j] (не дальше трёх слов назад, через прилагательные) стоит определитель, или
// это устойчивое выражение с предлогом без артикля (go to school, in bed, at work).
func hasDeterminer(toks []string, j int) bool {
	for k := j - 1; k >= 0 && k >= j-3; k-- {
		w := lowerWord(toks[k])
		if slices.Contains(determiners, w) || strings.HasSuffix(w, "'s") {
			return true
		}
		if slices.Contains(prepositions, w) {
			return k == j-1 // «to school», «in bed»: предлог сразу перед словом
		}
	}
	return false
}
