package testdb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
		t.Fatalf("to_regclass(%s): %v", name, err)
	}
	return exists
}

var allTables = []string{"users", "settings", "data_sources", "lexemes", "lexeme_cefr", "sentences", "sentence_pairs", "cards", "word_frequencies", "exercises", "topic_ratings"}

func TestMigrationsUpDownUp(t *testing.T) {
	pool := Pool(t)
	for _, name := range allTables {
		if !tableExists(t, pool, name) {
			t.Fatalf("после Up нет таблицы %s", name)
		}
	}

	Down(t, pool)
	for _, name := range allTables {
		if tableExists(t, pool, name) {
			t.Fatalf("после Down осталась таблица %s", name)
		}
	}

	Up(t, pool)
	for _, name := range allTables {
		if !tableExists(t, pool, name) {
			t.Fatalf("после повторного Up нет таблицы %s", name)
		}
	}
}

func TestKnowledgeSchemaConstraints(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()

	var srcID int64
	err := pool.QueryRow(ctx,
		`INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a') RETURNING id`).Scan(&srcID)
	if err != nil {
		t.Fatalf("data_sources: %v", err)
	}
	var lexID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO lexemes (lemma, pos, source_id) VALUES ('run', 'verb', $1) RETURNING id`, srcID).Scan(&lexID); err != nil {
		t.Fatalf("lexemes: %v", err)
	}

	tests := []struct {
		name string
		sql  string
		args []any
		code string
	}{
		{"уровень D1", `INSERT INTO lexeme_cefr (lexeme_id, source_id, level) VALUES ($1, $2, 'D1')`, []any{lexID, srcID}, "23514"},
		{"extra не объект", `INSERT INTO lexeme_cefr (lexeme_id, source_id, level, extra) VALUES ($1, $2, 'A1', '[]')`, []any{lexID, srcID}, "23514"},
		{"повтор слова и части речи", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('run', 'verb', $1)`, []any{srcID}, "23505"},
		{"повтор имени и версии", `INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a')`, nil, "23505"},
		{"пустое слово", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('', 'verb', $1)`, []any{srcID}, "23514"},
		{"слово длиннее 64", `INSERT INTO lexemes (lemma, pos, source_id) VALUES (repeat('x', 65), 'verb', $1)`, []any{srcID}, "23514"},
		{"источник не существует", `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('x', 'verb', 999999)`, nil, "23503"},
		{"отрицательное row_count", `UPDATE data_sources SET row_count = -1`, nil, "23514"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, tc.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("ожидали код %s, получили %v", tc.code, err)
			}
		})
	}

	// Источник нельзя удалить, пока на него ссылаются слова (RESTRICT).
	if _, err := pool.Exec(ctx, `DELETE FROM data_sources WHERE id = $1`, srcID); err == nil {
		t.Fatal("источник со словами удалился")
	}
}

func TestSentenceSchemaConstraints(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()

	var srcID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a') RETURNING id`).Scan(&srcID); err != nil {
		t.Fatal(err)
	}
	insert := func(lang string, tid int64) int64 {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ($1, $2, 'Go.', 'CK', 1, $3) RETURNING id`,
			lang, tid, srcID).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	en, ru := insert("en", 1), insert("ru", 2)
	if _, err := pool.Exec(ctx, `INSERT INTO sentence_pairs (en_id, ru_id, source_id) VALUES ($1, $2, $3)`, en, ru, srcID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		sql  string
		args []any
		code string
	}{
		{"язык fr", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('fr', 9, 'x', 'a', 1, $1)`, []any{srcID}, "23514"},
		{"tatoeba_id равен нулю", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 0, 'x', 'a', 1, $1)`, []any{srcID}, "23514"},
		{"пустой текст", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, '', 'a', 1, $1)`, []any{srcID}, "23514"},
		{"текст из 1001 символа", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, repeat('x', 1001), 'a', 1, $1)`, []any{srcID}, "23514"},
		{"автор из 65 символов", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, 'x', repeat('a', 65), 1, $1)`, []any{srcID}, "23514"},
		{"пустой автор", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, 'x', '', 1, $1)`, []any{srcID}, "23514"},
		{"ноль слов", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, 'x', 'a', 0, $1)`, []any{srcID}, "23514"},
		{"повтор языка и id", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 1, 'x', 'a', 1, $1)`, []any{srcID}, "23505"},
		{"пара с несуществующим en_id", `INSERT INTO sentence_pairs (en_id, ru_id, source_id) VALUES (999999, $1, $2)`, []any{ru, srcID}, "23503"},
		{"повтор пары", `INSERT INTO sentence_pairs (en_id, ru_id, source_id) VALUES ($1, $2, $3)`, []any{en, ru, srcID}, "23505"},
		{"несуществующий источник", `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 9, 'x', 'a', 1, 999999)`, nil, "23503"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, tc.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("ожидали код %s, получили %v", tc.code, err)
			}
		})
	}

	// Источник нельзя удалить, пока на него ссылаются предложения.
	if _, err := pool.Exec(ctx, `DELETE FROM data_sources WHERE id = $1`, srcID); err == nil {
		t.Fatal("источник с предложениями удалился")
	}
	// Удаление предложения каскадно убирает его пары.
	if _, err := pool.Exec(ctx, `DELETE FROM sentences WHERE id = $1`, en); err != nil {
		t.Fatal(err)
	}
	var pairs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sentence_pairs`).Scan(&pairs); err != nil || pairs != 0 {
		t.Fatalf("пар после удаления предложения: %d (%v)", pairs, err)
	}
}

// UpsertSentence отдаёт строку, которая лежит в базе: вызывающий по ней находит конфликт текста или автора.
func TestUpsertSentenceReturnsStoredRow(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "s", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	params := db.UpsertSentenceParams{Lang: "en", TatoebaID: 7, Text: "Go.", Author: "CK", WordCount: 1, SourceID: src.ID}

	first, err := q.UpsertSentence(ctx, params)
	if err != nil || first.ID == 0 || first.Text != "Go." || first.Author != "CK" {
		t.Fatalf("новая строка: %+v (%v)", first, err)
	}
	again, err := q.UpsertSentence(ctx, params)
	if err != nil || again.ID != first.ID {
		t.Fatalf("повтор того же: %+v (%v), ожидали id %d", again, err, first.ID)
	}
	other := params
	other.Text, other.Author = "Come.", "CM"
	got, err := q.UpsertSentence(ctx, other)
	if err != nil || got.ID != first.ID || got.Text != "Go." || got.Author != "CK" {
		t.Fatalf("при конфликте должна вернуться сохранённая строка: %+v (%v)", got, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sentences`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("строк в sentences: %d (%v)", n, err)
	}
	// Тот же номер на другом языке — другая строка.
	ru := params
	ru.Lang = "ru"
	if r, err := q.UpsertSentence(ctx, ru); err != nil || r.ID == first.ID {
		t.Fatalf("русская строка: %+v (%v)", r, err)
	}
}

// Индекс под поиск примеров: без него поиск по формам слова идёт полным перебором английских предложений.
func TestExamplesFullTextIndex(t *testing.T) {
	pool := Pool(t)
	var def string
	err := pool.QueryRow(context.Background(), `SELECT indexdef FROM pg_indexes WHERE indexname = 'sentences_en_fts'`).Scan(&def)
	if err != nil {
		t.Fatalf("индекса sentences_en_fts нет: %v", err)
	}
	for _, want := range []string{"USING gin", "to_tsvector('simple'::regconfig, text)", "lang = 'en'"} {
		if !strings.Contains(def, want) {
			t.Fatalf("в определении индекса %q нет %q", def, want)
		}
	}
}

func TestCardSchemaConstraints(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()

	var userID, srcID, lexID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('vlad', '$argon2id$x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('s', '1', 'l', 'u', 'a') RETURNING id`).Scan(&srcID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('run', 'verb', $1) RETURNING id`, srcID).Scan(&lexID); err != nil {
		t.Fatal(err)
	}
	const cols = `user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, remaining_steps`
	insert := func(user, lex int64, over string) error {
		_, err := pool.Exec(ctx, `INSERT INTO cards (`+cols+`) VALUES ($1, $2, now(), 0, 0, 0, 0, 0, 0, 0)`+over, user, lex)
		return err
	}
	if err := insert(userID, lexID, ""); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		sql  string
		code string
	}{
		{"состояние 4", `UPDATE cards SET state = 4`, "23514"},
		{"состояние -1", `UPDATE cards SET state = -1`, "23514"},
		{"отрицательная стабильность", `UPDATE cards SET stability = -0.1`, "23514"},
		{"отрицательная сложность", `UPDATE cards SET difficulty = -1`, "23514"},
		{"отрицательные дни", `UPDATE cards SET scheduled_days = -1`, "23514"},
		{"отрицательные повторения", `UPDATE cards SET reps = -1`, "23514"},
		{"отрицательные ошибки", `UPDATE cards SET lapses = -1`, "23514"},
		{"отрицательные шаги", `UPDATE cards SET remaining_steps = -1`, "23514"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("ожидали код %s, получили %v", tc.code, err)
			}
		})
	}
	for name, run := range map[string]func() error{
		"повтор пары пользователь–слово": func() error { return insert(userID, lexID, "") },
		"несуществующий пользователь":    func() error { return insert(999999, lexID, "") },
		"несуществующее слово":           func() error { return insert(userID, 999999, "") },
	} {
		t.Run(name, func(t *testing.T) {
			err := run()
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || (pgErr.Code != "23505" && pgErr.Code != "23503") {
				t.Fatalf("ожидали 23505 или 23503, получили %v", err)
			}
		})
	}

	// Слово с карточкой удалить нельзя, повторов не было ещё: last_review пуст.
	if _, err := pool.Exec(ctx, `DELETE FROM lexemes WHERE id = $1`, lexID); err == nil {
		t.Fatal("слово с карточкой удалилось")
	}
	var lastReviewNull bool
	if err := pool.QueryRow(ctx, `SELECT last_review IS NULL FROM cards`).Scan(&lastReviewNull); err != nil || !lastReviewNull {
		t.Fatalf("last_review должен быть пустым у нового: %v (%v)", lastReviewNull, err)
	}
	// Удаление пользователя каскадно убирает его карточки.
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cards`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("карточек после удаления пользователя: %d (%v)", n, err)
	}
}

func TestWordFrequencySchemaConstraints(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()
	var srcID int64
	if err := pool.QueryRow(ctx, `INSERT INTO data_sources (name, version, license, url, attribution) VALUES ('f', '1', 'l', 'u', 'a') RETURNING id`).Scan(&srcID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, 'you', 10, 1)`, srcID); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		sql  string
		code string
	}{
		{"счёт ноль", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, 'a', 0, 2)`, "23514"},
		{"ранг ноль", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, 'a', 5, 0)`, "23514"},
		{"пустое слово", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, '', 5, 2)`, "23514"},
		{"слово из 65 символов", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, repeat('x', 65), 5, 2)`, "23514"},
		{"повтор слова", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, 'you', 5, 2)`, "23505"},
		{"повтор ранга", `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES ($1, 'the', 5, 1)`, "23505"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, srcID)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("ожидали код %s, получили %v", tc.code, err)
			}
		})
	}
	if _, err := pool.Exec(ctx, `INSERT INTO word_frequencies (source_id, word, count, rank) VALUES (999999, 'x', 5, 9)`); err == nil {
		t.Fatal("несуществующий источник должен отклоняться")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM data_sources WHERE id = $1`, srcID); err == nil {
		t.Fatal("источник с частотами удалился")
	}
}
