package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/kaikki"
)

// maxLine — потолок длины записи JSONL (как у kb-prepare).
const maxLine = 128 << 20

// Options — параметры сборки.
type Options struct {
	Check   kb.Options     // пороги качества; пустые — без проверки
	Sources map[string]any // сведения об источниках для kb_build.sources (версии, суммы)
	// BeforeCommit вызывается внутри транзакции перед фиксацией (для тестов: читатель в это время видит прежние данные).
	BeforeCommit func()
}

// ReadEntries читает подготовленный файл Kaikki (JSONL); битая строка — ошибка с номером.
func ReadEntries(r io.Reader) ([]kb.Entry, error) {
	var out []kb.Entry
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), maxLine)
	for n := 1; sc.Scan(); n++ {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		e, err := kb.ParseEntry(line)
		if err != nil {
			return nil, fmt.Errorf("строка %d: %w", n, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

func loadInputs(ctx context.Context, q *db.Queries, entries []kb.Entry) (kb.Inputs, error) {
	in := kb.Inputs{Entries: entries, RuRank: map[string]int{}, EnCount: map[string]int64{}, SourceIDs: map[string]int64{}}
	lex, err := q.KbListLexemes(ctx)
	if err != nil {
		return in, fmt.Errorf("слова: %w", err)
	}
	for _, l := range lex {
		in.Lexemes = append(in.Lexemes, kb.Lexeme{ID: l.ID, Lemma: l.Lemma, Pos: l.Pos, Level: l.Level})
	}
	legacy, err := q.KbListLegacyTranslations(ctx, "freedict-eng-rus")
	if err != nil {
		return in, fmt.Errorf("переводы старого словаря: %w", err)
	}
	for _, l := range legacy {
		in.Legacy = append(in.Legacy, kb.LegacyTranslation{Lemma: l.Lemma, Pos: l.Pos, SenseNo: int(l.SenseNo), Ru: l.Ru})
	}
	ru, err := q.KbListFrequencies(ctx, "frequencywords-ru")
	if err != nil {
		return in, fmt.Errorf("частоты русских слов: %w", err)
	}
	for _, f := range ru {
		in.RuRank[f.Word] = int(f.Rank)
	}
	en, err := q.KbListFrequencies(ctx, "frequencywords")
	if err != nil {
		return in, fmt.Errorf("частоты английских слов: %w", err)
	}
	for _, f := range en {
		in.EnCount[f.Word] = f.Count
	}
	for key, name := range map[string]string{"kaikki": kaikki.Name, "freedict": "freedict-eng-rus"} {
		id, err := q.KbSourceID(ctx, name)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return in, fmt.Errorf("источник %s: %w", name, err)
		}
		in.SourceIDs[key] = id // 0, если источник не загружен
	}
	return in, nil
}

// Preview читает входы и считает отчёт, ничего не записывая (подбор констант по реальным данным).
func Preview(ctx context.Context, pool *pgxpool.Pool, kaikkiIn io.Reader) (kb.Report, error) {
	var entries []kb.Entry
	if kaikkiIn != nil {
		var err error
		if entries, err = ReadEntries(kaikkiIn); err != nil {
			return kb.Report{}, fmt.Errorf("файл Kaikki: %w", err)
		}
	}
	in, err := loadInputs(ctx, db.New(pool), entries)
	if err != nil {
		return kb.Report{}, err
	}
	_, rep := kb.Assemble(in)
	return rep, nil
}

// Build собирает базу знаний: читает слова, частоты, старый словарь и (если есть) подготовленный файл Kaikki,
// сливает по правилам, проверяет пороги и пишет kb_* одной транзакцией. Не прошло пороги или ошибка — данные не
// меняются, сборка помечается как неудачная.
func Build(ctx context.Context, pool *pgxpool.Pool, kaikkiIn io.Reader, opts Options) (kb.Report, error) {
	q := db.New(pool)
	buildID, err := q.KbInsertBuild(ctx)
	if err != nil {
		return kb.Report{}, fmt.Errorf("запись о сборке: %w", err)
	}
	rep, err := buildTx(ctx, pool, q, kaikkiIn, opts, buildID)
	if err != nil {
		// Отметка о неудаче делается отдельно и даже после отмены контекста.
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		payload, merr := json.Marshal(map[string]any{"error": err.Error(), "report": rep})
		if merr != nil {
			payload = json.RawMessage(`{}`)
		}
		if ferr := q.KbFinishBuild(fctx, db.KbFinishBuildParams{ID: buildID, Status: "failed", Sources: mustJSON(opts.Sources), Report: payload}); ferr != nil {
			slog.Warn("не удалось отметить сборку как неудачную: запись останется в состоянии running", "buildId", buildID, "err", ferr)
		}
		return rep, err
	}
	return rep, nil
}

func mustJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func buildTx(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, kaikkiIn io.Reader, opts Options, buildID int64) (kb.Report, error) {
	var entries []kb.Entry
	if kaikkiIn != nil {
		var err error
		if entries, err = ReadEntries(kaikkiIn); err != nil {
			return kb.Report{}, fmt.Errorf("файл Kaikki: %w", err)
		}
	}
	in, err := loadInputs(ctx, q, entries)
	if err != nil {
		return kb.Report{}, err
	}
	if len(entries) > 0 && in.SourceIDs["kaikki"] == 0 {
		return kb.Report{}, errors.New("источник " + kaikki.Name + " не зарегистрирован в data_sources (kb-prepare не дошёл до конца): запустите go run ./cmd/kb-prepare kaikki")
	}
	out, rep := kb.Assemble(in)
	if err := rep.Check(opts.Check); err != nil {
		return rep, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return rep, fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // после фиксации ничего не делает
	// Две сборки одновременно не нужны: вторая ждёт первую и затем пишет поверх, а не падает на конфликте ключей.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryKey); err != nil {
		return rep, fmt.Errorf("блокировка сборки: %w", err)
	}
	tq := q.WithTx(tx)
	// DELETE, а не TRUNCATE: читатели видят прежние данные до фиксации. Дочерние строки первыми (внешний ключ значения).
	for _, del := range []func(context.Context) error{
		tq.KbDeleteTeachingCards, tq.KbDeleteLexemeStats, tq.KbDeleteRelations, tq.KbDeleteForms,
		tq.KbDeletePronunciations, tq.KbDeleteTranslations, tq.KbDeleteSenses,
	} {
		if err := del(ctx); err != nil {
			return rep, fmt.Errorf("очистка: %w", err)
		}
	}
	if err := write(ctx, tx, out); err != nil {
		return rep, err
	}
	payload, err := json.Marshal(rep)
	if err != nil {
		return rep, err
	}
	if err := tq.KbFinishBuild(ctx, db.KbFinishBuildParams{ID: buildID, Status: "ok", Sources: mustJSON(opts.Sources), Report: payload}); err != nil {
		return rep, fmt.Errorf("отметка сборки: %w", err)
	}
	if opts.BeforeCommit != nil {
		opts.BeforeCommit()
	}
	if err := tx.Commit(ctx); err != nil {
		return rep, fmt.Errorf("фиксация: %w", err)
	}
	return rep, nil
}

// advisoryKey — ключ консультативной блокировки сборки базы знаний.
const advisoryKey int64 = 7140014

// ThresholdLine — строка для пробного запуска: пройдёт ли такая сборка пороги качества.
func ThresholdLine(rep kb.Report, o kb.Options) string {
	if err := rep.Check(o); err != nil {
		return "пороги качества: НЕ пройдены (" + strings.TrimPrefix(err.Error(), kb.ErrBelowThreshold.Error()+": ") + ")"
	}
	return "пороги качества: пройдены"
}

func copyRows(ctx context.Context, tx pgx.Tx, table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("запись %s: %w", table, err)
	}
	return nil
}

func write(ctx context.Context, tx pgx.Tx, out kb.Output) error {
	var senses, translations, prons, forms, rels, stats, cards [][]any
	for _, r := range out.Senses {
		senses = append(senses, []any{r.LexemeID, int32(r.SenseNo), r.GlossEN, r.Tags, r.Flags, r.SourceID}) //nolint:gosec // номер значения мал
	}
	for _, r := range out.Translations {
		var sense any
		if r.SenseNo != nil {
			sense = int32(*r.SenseNo) //nolint:gosec // номер значения мал
		}
		translations = append(translations, []any{r.LexemeID, sense, r.Lang, r.Text, float32(r.Score), int32(r.Rank), r.Chosen, r.Flags, r.SourceIDs}) //nolint:gosec // ранг мал
	}
	for _, r := range out.Pronunciations {
		prons = append(prons, []any{r.LexemeID, r.Accent, r.IPA, r.Audio, r.Primary, r.SourceID})
	}
	for _, r := range out.Forms {
		forms = append(forms, []any{r.Form, r.LexemeID, r.Tag})
	}
	for _, r := range out.Relations {
		var other any
		if r.OtherLexemeID != nil {
			other = *r.OtherLexemeID
		}
		rels = append(rels, []any{r.LexemeID, r.Kind, r.OtherLemma, other})
	}
	for _, r := range out.Stats {
		var rank any
		if r.Rank != nil {
			rank = int32(*r.Rank) //nolint:gosec // ранг не больше числа слов
		}
		stats = append(stats, []any{r.LexemeID, r.Count, rank})
	}
	for _, r := range out.Cards {
		cards = append(cards, []any{r.LexemeID, r.Payload})
	}
	for _, step := range []struct {
		table string
		cols  []string
		rows  [][]any
	}{
		{"kb_sense", []string{"lexeme_id", "sense_no", "gloss_en", "tags", "flags", "source_id"}, senses},
		{"kb_translation", []string{"lexeme_id", "sense_no", "lang", "text", "score", "rank", "chosen", "flags", "source_ids"}, translations},
		{"kb_pronunciation", []string{"lexeme_id", "accent", "ipa", "audio_ref", "is_primary", "source_id"}, prons},
		{"kb_form", []string{"form", "lexeme_id", "tag"}, forms},
		{"kb_relation", []string{"lexeme_id", "kind", "other_lemma", "other_lexeme_id"}, rels},
		{"kb_lexeme_stat", []string{"lexeme_id", "freq_count", "freq_rank"}, stats},
		{"kb_teaching_card", []string{"lexeme_id", "payload"}, cards},
	} {
		if err := copyRows(ctx, tx, step.table, step.cols, step.rows); err != nil {
			return err
		}
	}
	return nil
}
