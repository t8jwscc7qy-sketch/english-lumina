package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
)

// maxLine — потолок длины одной записи JSONL: самые большие записи Викисловаря (a, the, go) занимают мегабайты.
const maxLine = 128 << 20

// Stats — счётчики прохода: всего строк, английских, отобрано.
type Stats struct {
	Lines    int64
	English  int64
	Selected int64
}

// Prepare читает gzip-поток JSONL Kaikki построчно и пишет в out (как есть, по строке) записи английских слов из
// lemmas. Ошибки gzip и обрыв архива возвращаются, отмена контекста прерывает проход.
func Prepare(ctx context.Context, in io.Reader, lemmas map[string]bool, out io.Writer) (Stats, error) {
	var st Stats
	zr, err := gzip.NewReader(in)
	if err != nil {
		return st, fmt.Errorf("архив: %w", err)
	}
	defer func() { _ = zr.Close() }()
	if err := ctx.Err(); err != nil {
		return st, err
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), maxLine)
	w := bufio.NewWriter(out)
	for sc.Scan() {
		st.Lines++
		if st.Lines%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return st, err
			}
		}
		line := sc.Bytes()
		if !kb.IsEnglishLine(line) {
			continue
		}
		st.English++
		if !kb.Wanted(line, lemmas) {
			continue
		}
		st.Selected++
		if _, err := w.Write(line); err != nil {
			return st, err
		}
		if err := w.WriteByte('\n'); err != nil {
			return st, err
		}
	}
	if err := sc.Err(); err != nil {
		return st, fmt.Errorf("чтение архива: %w", err)
	}
	return st, w.Flush()
}

// LemmaSet строит набор слов для поиска: леммы с вариантами через «/» (colour/color) разбиваются на отдельные слова.
func LemmaSet(lemmas []string) map[string]bool {
	set := map[string]bool{}
	for _, l := range lemmas {
		for _, v := range kb.NormalizeLemmaVariants(l) {
			set[v] = true
		}
	}
	return set
}

// LemmasHash — хеш набора лемм, не зависящий от порядка: по нему сборка проверяет, что подготовленный файл
// соответствует текущему списку слов.
func LemmasHash(lemmas []string) string {
	sorted := slices.Clone(lemmas)
	slices.Sort(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// Meta — сведения о подготовленном файле (лежит рядом, имя .meta).
type Meta struct {
	FileSHA256 string `json:"fileSha256"`
	LemmasHash string `json:"lemmasHash"`
	Selected   int64  `json:"selected"`
}

// WriteMeta пишет сведения о подготовленном файле.
func WriteMeta(path string, m Meta) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644) //nolint:gosec // обычный файл кэша без секретов
}

// ReadMeta читает сведения о подготовленном файле.
func ReadMeta(path string) (Meta, error) {
	var m Meta
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}
