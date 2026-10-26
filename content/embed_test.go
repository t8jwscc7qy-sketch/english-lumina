package content

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sheets"
)

// Встроенные шпаргалки проходят проверку схемы: битый файл ловится тестом, а не в рантайме.
func TestEmbeddedSheetsAreValid(t *testing.T) {
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("нет ни одной шпаргалки")
	}
}

// Каждая лемма привязки существует в загруженных словах (проверка на настоящих данных; без базы пропускается).
func TestSheetWordKeysExistInLexemes(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("нет DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skip("база недоступна")
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lexemes`).Scan(&n); err != nil || n == 0 {
		t.Skip("слова не загружены")
	}
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		keys := s.WordKeys()
		if len(keys) == 0 {
			continue
		}
		poss := []string{}
		for _, b := range s.Blocks {
			if b.Type == sheets.BlockTable && b.Bind == sheets.BindWord {
				poss = append(poss, b.Pos...)
			}
		}
		rows, err := pool.Query(ctx, `SELECT DISTINCT lemma FROM lexemes WHERE lemma = ANY($1) AND (cardinality($2::text[]) = 0 OR pos = ANY($2::text[]))`, keys, poss)
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for rows.Next() {
			var l string
			if err := rows.Scan(&l); err != nil {
				t.Fatal(err)
			}
			have[l] = true
		}
		rows.Close()
		var missing []string
		for _, k := range keys {
			if !have[k] {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("шпаргалка %s: нет в словах: %v", s.ID, missing)
		}
	}
}
