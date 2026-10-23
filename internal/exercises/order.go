package exercises

import (
	"math/rand/v2"
	"slices"
	"strings"
	"unicode/utf8"
)

// Границы длины упражнения «порядок слов» в плитках.
const (
	OrderMinTiles = 4
	OrderMaxTiles = 10
)

// shuffleAttempts — сколько раз перемешиваем заново, если плитки случайно сложились в верный ответ.
const shuffleAttempts = 5

// Order — упражнение «порядок слов».
type Order struct {
	Tiles   []string // слова в перемешанном порядке; первое слово предложения — строчное (кроме I)
	Answers []string // допустимые предложения целиком; Answers[0] — исходное
}

// BuildOrder собирает упражнение из английского предложения. Альтернативы — другие английские
// предложения с тем же переводом: они принимаются как верные, если состоят из тех же слов (без учёта
// регистра). Слова — куски между пробелами; запятые остаются на словах, завершающие .!? в плитки не входят. false, если предложение
// не подходит: длина вне 4–10 слов, имя собственное не в начале (регистр выдал бы начало), символы,
// у которых ToLower меняет длину, или плитки не удалось перемешать так, чтобы они не сложились в ответ.
func BuildOrder(sentence string, alternatives []string, seed int64) (Order, bool) {
	sentence = strings.TrimSpace(sentence)
	words := strings.Fields(sentence)
	if len(words) < OrderMinTiles || len(words) > OrderMaxTiles || len(strings.ToLower(sentence)) != len(sentence) {
		return Order{}, false
	}
	for i, w := range words {
		if i > 0 && startsUpper(w) && !isPronounI(w) {
			return Order{}, false
		}
	}
	tiles := withoutFinalPunct(words)
	if tiles[len(tiles)-1] == "" {
		return Order{}, false
	}

	answers := []string{strings.Join(words, " ")}
	seen := map[string]bool{normOrder(answers[0]): true}
	want := sortedLower(tiles)
	for _, alt := range alternatives {
		altWords := strings.Fields(alt)
		if len(altWords) == 0 || seen[normOrder(alt)] || !slices.Equal(sortedLower(withoutFinalPunct(altWords)), want) {
			continue
		}
		seen[normOrder(alt)] = true
		answers = append(answers, strings.Join(altWords, " "))
	}

	if startsUpper(tiles[0]) && !isPronounI(tiles[0]) {
		tiles[0] = strings.ToLower(tiles[0][:1]) + tiles[0][1:]
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
	for range shuffleAttempts {
		rng.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] })
		if !CheckOrder(answers, strings.Join(tiles, " ")) {
			return Order{Tiles: tiles, Answers: answers}, true
		}
	}
	return Order{}, false
}

// CheckOrder сравнивает собранное предложение с допустимыми без учёта регистра и лишних пробелов.
// Завершающие .!? не сравниваются, запятые внутри предложения — да. Пустой ответ и ответ длиннее 200 символов неверны.
func CheckOrder(answers []string, given string) bool {
	given = normOrder(given)
	if given == "" || utf8.RuneCountInString(given) > maxAnswerLen {
		return false
	}
	for _, a := range answers {
		if normOrder(a) == given {
			return true
		}
	}
	return false
}

// OrderTopics — ключи тем упражнения «порядок слов»: навык и уровень предложения (если он известен).
func OrderTopics(level string) []string {
	if level == "" {
		return []string{"skill:order"}
	}
	return []string{"skill:order", "level:" + level}
}

// normOrder приводит предложение к виду для сравнения: нижний регистр, одиночные пробелы, без завершающих .!?.
func normOrder(s string) string {
	return strings.TrimRight(strings.ToLower(strings.Join(strings.Fields(s), " ")), ".!? ")
}

// withoutFinalPunct возвращает копию слов, у последнего убраны завершающие .!?: в плитках их нет.
func withoutFinalPunct(words []string) []string {
	out := slices.Clone(words)
	out[len(out)-1] = strings.TrimRight(out[len(out)-1], ".!?")
	return out
}

func sortedLower(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = strings.ToLower(w)
	}
	slices.Sort(out)
	return out
}

func startsUpper(w string) bool {
	return w != "" && w[0] >= 'A' && w[0] <= 'Z'
}

// isPronounI: «I» и его сокращения (I'm, I'll, I've, I'd) пишутся с заглавной в любом месте предложения.
func isPronounI(w string) bool {
	w = strings.TrimRight(w, ".,;:!?\"")
	return w == "I" || strings.HasPrefix(w, "I'") || strings.HasPrefix(w, "I’")
}
