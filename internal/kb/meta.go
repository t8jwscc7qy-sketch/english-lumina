package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

// LemmaSet строит набор слов для поиска: леммы с вариантами через «/» (colour/color) разбиваются на отдельные слова.
func LemmaSet(lemmas []string) map[string]bool {
	set := map[string]bool{}
	for _, l := range lemmas {
		for _, v := range NormalizeLemmaVariants(l) {
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
