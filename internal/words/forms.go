package words

import (
	"slices"
	"strings"
	"unicode"
)

// MaxForms — предел числа форм в одном запросе: защита от слишком длинного поискового выражения.
const MaxForms = 64

// Forms возвращает формы слова для поиска примеров. Каждая форма — последовательность слов в нижнем регистре
// (однословная форма — одно слово, фраза — несколько). Лемматизатора нет: формы строятся по правилам
// английской морфологии и небольшим таблицам неправильных слов. Омонимы не различаются.
//
// Слова бьются по любым знакам кроме букв и цифр — так же, как токенизатор полнотекстового поиска PostgreSQL
// (`simple`). Поэтому в формах не бывает апострофов, скобок и других знаков синтаксиса tsquery.
func Forms(lemma, pos string) [][]string {
	var out [][]string
	seen := map[string]bool{}
	add := func(words ...string) {
		key := strings.Join(words, " ")
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, words)
	}
	for _, variant := range strings.Split(lemma, "/") {
		tokens := tokenize(variant)
		switch {
		case len(tokens) == 0:
		case len(tokens) > 1:
			add(tokens...) // фразу ищем как есть
		default:
			for _, f := range inflect(tokens[0], pos) {
				add(f)
			}
		}
	}
	slices.SortFunc(out, func(a, b []string) int { return strings.Compare(strings.Join(a, " "), strings.Join(b, " ")) })
	if len(out) > MaxForms {
		out = out[:MaxForms]
	}
	return out
}

// TSQuery собирает из форм выражение для to_tsquery('simple', …): 'run' | 'runs' | 'turn' <-> 'to'.
func TSQuery(forms [][]string) string {
	parts := make([]string, 0, len(forms))
	for _, f := range forms {
		quoted := make([]string, len(f))
		for i, w := range f {
			quoted[i] = "'" + strings.ReplaceAll(w, "'", "''") + "'"
		}
		parts = append(parts, strings.Join(quoted, " <-> "))
	}
	return strings.Join(parts, " | ")
}

// tokenize режет строку на слова: подряд идущие буквы и цифры, нижний регистр.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func inflect(w, pos string) []string {
	switch pos {
	case "be-verb":
		return []string{"be", "am", "is", "are", "was", "were", "been", "being"}
	case "do-verb":
		return []string{"do", "does", "did", "done", "doing"}
	case "have-verb":
		return []string{"have", "has", "had", "having"}
	case "verb":
		if irr, ok := irregularVerbs[w]; ok {
			return append([]string{w}, irr...)
		}
		return verbForms(w)
	case "noun":
		if irr, ok := irregularNouns[w]; ok {
			return append([]string{w}, irr...)
		}
		return nounForms(w)
	case "adjective":
		if irr, ok := irregularAdjectives[w]; ok {
			return append([]string{w}, irr...)
		}
		return adjectiveForms(w)
	}
	return []string{w}
}

func isVowel(r byte) bool { return strings.IndexByte("aeiou", r) >= 0 }

func isASCIILetters(w string) bool {
	for i := 0; i < len(w); i++ {
		if w[i] < 'a' || w[i] > 'z' {
			return false
		}
	}
	return w != ""
}

// endsConsonantY: «city», «study», но не «day».
func endsConsonantY(w string) bool {
	return len(w) >= 2 && w[len(w)-1] == 'y' && !isVowel(w[len(w)-2])
}

// doubled возвращает слово с удвоенной последней согласной для коротких слов вида согласная+гласная+согласная
// (stop → stopp, big → bigg). Для длинных слов с другим ударением форма лишняя, но безвредна: такого слова нет.
func doubled(w string) (string, bool) {
	n := len(w)
	if n < 3 || !isASCIILetters(w) {
		return "", false
	}
	a, b, c := w[n-3], w[n-2], w[n-1]
	if isVowel(a) || !isVowel(b) || isVowel(c) || strings.IndexByte("wxy", c) >= 0 {
		return "", false
	}
	return w + string(c), true
}

func plural(w string) []string {
	switch {
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"),
		strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		return []string{w + "es"}
	case endsConsonantY(w):
		return []string{w[:len(w)-1] + "ies"}
	case strings.HasSuffix(w, "o"):
		return []string{w + "s", w + "es"} // potato → potatoes, photo → photos: берём оба
	}
	return []string{w + "s"}
}

func nounForms(w string) []string { return append([]string{w}, plural(w)...) }

func verbForms(w string) []string {
	out := []string{w}
	out = append(out, plural(w)...) // 3-е лицо: runs, goes, studies
	base, hasDouble := doubled(w)
	switch {
	case strings.HasSuffix(w, "ie"):
		out = append(out, w+"d", w[:len(w)-2]+"ying") // die → died, dying
	case strings.HasSuffix(w, "e"):
		out = append(out, w+"d")
		if !strings.HasSuffix(w, "ee") && !strings.HasSuffix(w, "oe") && !strings.HasSuffix(w, "ye") {
			out = append(out, w[:len(w)-1]+"ing") // like → liking, но see → seeing
		} else {
			out = append(out, w+"ing")
		}
	case endsConsonantY(w):
		out = append(out, w[:len(w)-1]+"ied", w+"ing")
	default:
		out = append(out, w+"ed", w+"ing")
		if hasDouble {
			out = append(out, base+"ed", base+"ing")
		}
	}
	return out
}

func adjectiveForms(w string) []string {
	out := []string{w}
	base, hasDouble := doubled(w)
	switch {
	case strings.HasSuffix(w, "e"):
		out = append(out, w+"r", w+"st")
	case endsConsonantY(w):
		out = append(out, w[:len(w)-1]+"ier", w[:len(w)-1]+"iest")
	default:
		out = append(out, w+"er", w+"est")
		if hasDouble {
			out = append(out, base+"er", base+"est")
		}
	}
	return out
}

// Неправильные слова: формы без леммы. Таблицы короткие — самые частые слова; редкие неправильные формы
// не находятся (ограничение первой версии).
var irregularVerbs = map[string][]string{
	"go": {"goes", "going", "went", "gone"}, "say": {"says", "saying", "said"}, "make": {"makes", "making", "made"},
	"take": {"takes", "taking", "took", "taken"}, "come": {"comes", "coming", "came"}, "see": {"sees", "seeing", "saw", "seen"},
	"know": {"knows", "knowing", "knew", "known"}, "get": {"gets", "getting", "got", "gotten"},
	"give": {"gives", "giving", "gave", "given"}, "find": {"finds", "finding", "found"}, "think": {"thinks", "thinking", "thought"},
	"tell": {"tells", "telling", "told"}, "become": {"becomes", "becoming", "became"}, "show": {"shows", "showing", "showed", "shown"},
	"leave": {"leaves", "leaving", "left"}, "feel": {"feels", "feeling", "felt"}, "put": {"puts", "putting"},
	"bring": {"brings", "bringing", "brought"}, "begin": {"begins", "beginning", "began", "begun"},
	"keep": {"keeps", "keeping", "kept"}, "hold": {"holds", "holding", "held"}, "write": {"writes", "writing", "wrote", "written"},
	"stand": {"stands", "standing", "stood"}, "hear": {"hears", "hearing", "heard"}, "let": {"lets", "letting"},
	"mean": {"means", "meaning", "meant"}, "set": {"sets", "setting"}, "meet": {"meets", "meeting", "met"},
	"run": {"runs", "running", "ran"}, "pay": {"pays", "paying", "paid"}, "sit": {"sits", "sitting", "sat"},
	"speak": {"speaks", "speaking", "spoke", "spoken"}, "lie": {"lies", "lying", "lay", "lain", "lied"},
	"lead": {"leads", "leading", "led"}, "read": {"reads", "reading"}, "grow": {"grows", "growing", "grew", "grown"},
	"lose": {"loses", "losing", "lost"}, "fall": {"falls", "falling", "fell", "fallen"}, "send": {"sends", "sending", "sent"},
	"build": {"builds", "building", "built"}, "understand": {"understands", "understanding", "understood"},
	"draw": {"draws", "drawing", "drew", "drawn"}, "break": {"breaks", "breaking", "broke", "broken"},
	"spend": {"spends", "spending", "spent"}, "cut": {"cuts", "cutting"}, "rise": {"rises", "rising", "rose", "risen"},
	"drive": {"drives", "driving", "drove", "driven"}, "buy": {"buys", "buying", "bought"}, "wear": {"wears", "wearing", "wore", "worn"},
	"choose": {"chooses", "choosing", "chose", "chosen"}, "eat": {"eats", "eating", "ate", "eaten"},
	"drink": {"drinks", "drinking", "drank", "drunk"}, "sing": {"sings", "singing", "sang", "sung"},
	"swim": {"swims", "swimming", "swam", "swum"}, "fly": {"flies", "flying", "flew", "flown"},
	"forget": {"forgets", "forgetting", "forgot", "forgotten"}, "sleep": {"sleeps", "sleeping", "slept"},
	"win": {"wins", "winning", "won"}, "catch": {"catches", "catching", "caught"}, "teach": {"teaches", "teaching", "taught"},
	"sell": {"sells", "selling", "sold"}, "hit": {"hits", "hitting"}, "hurt": {"hurts", "hurting"},
	"fight": {"fights", "fighting", "fought"}, "throw": {"throws", "throwing", "threw", "thrown"},
	"ride": {"rides", "riding", "rode", "ridden"}, "hide": {"hides", "hiding", "hid", "hidden"},
	"shake": {"shakes", "shaking", "shook", "shaken"}, "steal": {"steals", "stealing", "stole", "stolen"},
	"wake": {"wakes", "waking", "woke", "woken"}, "bite": {"bites", "biting", "bit", "bitten"},
	"blow": {"blows", "blowing", "blew", "blown"}, "feed": {"feeds", "feeding", "fed"}, "lend": {"lends", "lending", "lent"},
	"hang": {"hangs", "hanging", "hung"}, "shoot": {"shoots", "shooting", "shot"}, "stick": {"sticks", "sticking", "stuck"},
	"forgive": {"forgives", "forgiving", "forgave", "forgiven"}, "bear": {"bears", "bearing", "bore", "borne"},
}

var irregularNouns = map[string][]string{
	"man": {"men"}, "woman": {"women"}, "child": {"children"}, "person": {"people"}, "foot": {"feet"},
	"tooth": {"teeth"}, "mouse": {"mice"}, "goose": {"geese"}, "ox": {"oxen"}, "sheep": {}, "fish": {"fishes"},
	"life": {"lives"}, "wife": {"wives"}, "knife": {"knives"}, "leaf": {"leaves"}, "half": {"halves"},
	"wolf": {"wolves"}, "shelf": {"shelves"}, "thief": {"thieves"}, "loaf": {"loaves"},
}

var irregularAdjectives = map[string][]string{
	"good": {"better", "best"}, "bad": {"worse", "worst"}, "far": {"farther", "further", "farthest", "furthest"},
}

// irregularPastSet — неправильные формы прошедшего времени и причастия (went, gone, taken):
// всё из таблицы irregularVerbs, кроме основы, форм на -s и -ing.
// Глаголы, у которых прошедшее совпадает с основой (put, read, cut), сюда не попадают.
var irregularPastSet = func() map[string]bool {
	set := map[string]bool{}
	for _, forms := range irregularVerbs {
		for _, f := range forms {
			if !strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ing") {
				set[f] = true
			}
		}
	}
	return set
}()

// IrregularPast сообщает, что слово — неправильная форма прошедшего времени (регистр не важен).
func IrregularPast(word string) bool {
	return irregularPastSet[strings.ToLower(word)]
}
