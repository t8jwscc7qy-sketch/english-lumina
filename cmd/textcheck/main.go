// Команда textcheck проверяет авторские тексты (content/texts/*.json) по словарю уровня из базы знаний:
// все слова, кроме целевых и имён, должны входить в самые частотные N слов уровня.
//
//	go run ./cmd/textcheck            # все встроенные тексты
//	go run ./cmd/textcheck файл.json  # файл со списком текстов (до встраивания)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/library"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(files []string) error {
	var texts []library.Text
	var err error
	if len(files) == 0 {
		texts, err = library.LoadAll(content.Texts)
	} else {
		for _, f := range files {
			data, rerr := os.ReadFile(f) //nolint:gosec // путь передаёт сам автор текстов
			if rerr != nil {
				return rerr
			}
			var batch []json.RawMessage
			if jerr := json.Unmarshal(data, &batch); jerr != nil {
				return fmt.Errorf("%s: %w", f, jerr)
			}
			for _, raw := range batch {
				t, perr := library.Parse(raw)
				if perr != nil {
					fmt.Printf("СТРУКТУРА %s: %v\n", f, perr)
					err = fmt.Errorf("есть нарушения")
					continue
				}
				texts = append(texts, t)
			}
		}
	}
	if err != nil && len(files) == 0 {
		return err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("нужен DATABASE_URL (словарь уровня берётся из базы знаний)")
	}
	ctx := context.Background()
	pool, perr := pgxpool.New(ctx, url)
	if perr != nil {
		return perr
	}
	defer pool.Close()
	q := db.New(pool)
	allowed := map[int]map[string]bool{}
	bad := err != nil
	for _, t := range texts {
		if allowed[t.Band] == nil {
			rows, qerr := q.LibraryBandLexemes(ctx, int32(t.Band)) //nolint:gosec // уровни — малые числа
			if qerr != nil {
				return qerr
			}
			ws := make([]library.BandWord, len(rows))
			for i, r := range rows {
				ws[i] = library.BandWord{Lemma: r.Lemma, Pos: r.Pos}
			}
			allowed[t.Band] = library.AllowedForms(ws)
		}
		if issues := t.CheckStyle(); len(issues) > 0 {
			bad = true
			fmt.Printf("СТИЛЬ %s:", t.ID)
			for _, is := range issues {
				fmt.Print(" ", is.String())
			}
			fmt.Println()
		}
		if probs := t.CheckVocabulary(allowed[t.Band]); len(probs) > 0 {
			bad = true
			fmt.Printf("СЛОВАРЬ %s (уровень %d, слов %d):", t.ID, t.Band, t.WordCount())
			for _, p := range probs {
				fmt.Printf(" [предл. %d: %s]", p.Sentence, p.Word)
			}
			fmt.Println()
		}
	}
	fmt.Printf("текстов проверено: %d\n", len(texts))
	if bad {
		return fmt.Errorf("есть нарушения")
	}
	return nil
}
