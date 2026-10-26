package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
)

// Explain печатает для слов (в виде лемма/часть_речи) всех кандидатов-переводов с оценками, пометками и источниками:
// инструмент подбора констант оценки. Ничего не записывает.
func Explain(ctx context.Context, pool *pgxpool.Pool, kaikkiIn io.Reader, words []string, w io.Writer) error {
	type target struct{ lemma, pos string }
	var targets []target
	for _, s := range words {
		lemma, pos, ok := strings.Cut(strings.TrimSpace(s), "/")
		if !ok || lemma == "" || pos == "" {
			return fmt.Errorf("слово %q: нужен вид лемма/часть_речи (например dad/noun)", s)
		}
		targets = append(targets, target{lemma, pos})
	}
	var entries []kb.Entry
	if kaikkiIn != nil {
		var err error
		if entries, err = ReadEntries(kaikkiIn); err != nil {
			return fmt.Errorf("файл Kaikki: %w", err)
		}
	}
	in, err := loadInputs(ctx, db.New(pool), entries)
	if err != nil {
		return err
	}
	all := in.Lexemes
	for _, t := range targets {
		var lex *kb.Lexeme
		for i := range all {
			if all[i].Pos == t.pos && (all[i].Lemma == t.lemma || strings.Contains("/"+all[i].Lemma+"/", "/"+t.lemma+"/")) {
				lex = &all[i]
				break
			}
		}
		if lex == nil {
			if _, err := fmt.Fprintf(w, "%s (%s): в таблице lexemes нет такого слова\n", t.lemma, t.pos); err != nil {
				return err
			}
			continue
		}
		one := in
		one.Lexemes = []kb.Lexeme{*lex}
		out, _ := kb.Assemble(one)
		if _, err := fmt.Fprintf(w, "%s (%s), уровень %s:\n", lex.Lemma, lex.Pos, lex.Level); err != nil {
			return err
		}
		for _, r := range out.Translations {
			mark := " "
			if r.Chosen {
				mark = "*"
			}
			sense := "—"
			if r.SenseNo != nil {
				sense = fmt.Sprint(*r.SenseNo)
			}
			if _, err := fmt.Fprintf(w, "  %s %-20s оценка %5.1f  значение %-3s источники %v  %s\n", mark, r.Text, r.Score, sense, r.SourceIDs, strings.Join(r.Flags, ",")); err != nil {
				return err
			}
		}
		for _, s := range out.Senses {
			if _, err := fmt.Fprintf(w, "    значение %d: %s %s\n", s.SenseNo, s.GlossEN, strings.Join(s.Flags, ",")); err != nil {
				return err
			}
		}
	}
	return nil
}
