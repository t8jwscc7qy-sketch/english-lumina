// Package words готовит поисковые запросы по словарю.
package words

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxQueryLen — длина запроса в символах; как предел длины слова в таблице lexemes.
const MaxQueryLen = 64

var (
	ErrEmpty   = errors.New("пустой запрос")
	ErrTooLong = errors.New("запрос длиннее 64 символов")
	ErrInvalid = errors.New("в запросе недопустимые символы")
)

// Query — запрос в виде, который принимает SearchLexemes.
type Query struct {
	Exact   string // слово целиком в нижнем регистре; сравнивается через =, не экранируется
	Prefix  string // шаблон LIKE: слово начинается с запроса
	Variant string // шаблон LIKE: с запроса начинается вариант после слэша (cafe/café)
}

// ParseQuery обрезает пробелы, приводит к нижнему регистру и экранирует спецсимволы LIKE (\ % _),
// чтобы запрос «%» искал знак процента, а не все слова.
func ParseQuery(raw string) (Query, error) {
	if !utf8.ValidString(raw) || strings.ContainsRune(raw, 0) {
		return Query{}, ErrInvalid // PostgreSQL такие строки не принимает
	}
	q := strings.ToLower(strings.TrimSpace(raw))
	if q == "" {
		return Query{}, ErrEmpty
	}
	if utf8.RuneCountInString(q) > MaxQueryLen {
		return Query{}, ErrTooLong
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	return Query{Exact: q, Prefix: escaped + "%", Variant: "%/" + escaped + "%"}, nil
}
