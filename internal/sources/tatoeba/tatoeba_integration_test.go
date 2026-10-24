package tatoeba

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/importer"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// importContent импортирует rus.txt с заданным содержимым через TLS-сервер, как настоящий запуск.
func importContent(t *testing.T, pool *pgxpool.Pool, content string) (importer.Result, error) {
	t.Helper()
	z := makeZip(t, entry{"rus.txt", content})
	src, _ := serve(t, z, sum(z), 1<<20)
	return importer.Run(context.Background(), pool, src, t.TempDir())
}

func assertEmpty(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"data_sources", "sentences", "sentence_pairs"} {
		if n := count(t, pool, "SELECT count(*) FROM "+table); n != 0 {
			t.Fatalf("после неудачи в %s осталось %d строк", table, n)
		}
	}
}

func pair(en, ru string, enID, ruID int, enAuthor, ruAuthor string) string {
	return en + "\t" + ru + "\tCC-BY 2.0 (France) Attribution: tatoeba.org #" + strconv.Itoa(enID) + " (" + enAuthor + ") & #" + strconv.Itoa(ruID) + " (" + ruAuthor + ")\n"
}

func TestRunImportsSample(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()

	res, err := importContent(t, pool, readSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 32 {
		t.Fatalf("Rows=%d, ожидали 32", res.Rows)
	}
	if en, ru := count(t, pool, `SELECT count(*) FROM sentences WHERE lang = 'en'`), count(t, pool, `SELECT count(*) FROM sentences WHERE lang = 'ru'`); en != 27 || ru != 29 {
		t.Fatalf("английских %d, русских %d; ожидали 27 и 29", en, ru)
	}
	if n := count(t, pool, `SELECT count(*) FROM sentence_pairs`); n != 32 {
		t.Fatalf("пар %d, ожидали 32", n)
	}

	ds, err := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "tatoeba", Version: "2026-02-13"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.RowCount == nil || *ds.RowCount != 32 || !ds.ImportedAt.Valid || !strings.Contains(ds.Attribution, "CC BY 2.0 FR") {
		t.Fatalf("data_sources: %+v", ds)
	}

	var text, author string
	var words int
	err = pool.QueryRow(ctx, `SELECT text, author, word_count FROM sentences WHERE lang = 'en' AND tatoeba_id = 2877272`).Scan(&text, &author, &words)
	if err != nil || text != "Go." || author != "CM" || words != 1 {
		t.Fatalf("Go.: %q %q %d (%v)", text, author, words, err)
	}
	err = pool.QueryRow(ctx, `SELECT text, author, word_count FROM sentences WHERE lang = 'en' AND tatoeba_id = 2951771`).Scan(&text, &author, &words)
	if err != nil || author != "a.b-c" || words != 5 {
		t.Fatalf("golf: %q %q %d (%v)", text, author, words, err)
	}
	err = pool.QueryRow(ctx, `SELECT author, word_count FROM sentences WHERE lang = 'ru' AND tatoeba_id = 3544517`).Scan(&author, &words)
	if err != nil || author != "d.e-f" || words != 6 {
		t.Fatalf("русское golf: %q %d (%v)", author, words, err)
	}

	// Английское «Go.» хранится один раз, а переводов у него три.
	if n := count(t, pool, `SELECT count(*) FROM sentence_pairs p JOIN sentences e ON e.id = p.en_id WHERE e.tatoeba_id = 2877272`); n != 3 {
		t.Fatalf("переводов у Go.: %d, ожидали 3", n)
	}
	// Русское предложение 402127 общее для нескольких английских.
	if n := count(t, pool, `SELECT count(*) FROM sentence_pairs p JOIN sentences r ON r.id = p.ru_id WHERE r.tatoeba_id = 402127`); n < 2 {
		t.Fatalf("английских у 402127: %d", n)
	}
	// В паре английское всегда на английском, русское на русском; источник пар — этот.
	if n := count(t, pool, `SELECT count(*) FROM sentence_pairs p JOIN sentences e ON e.id = p.en_id JOIN sentences r ON r.id = p.ru_id WHERE e.lang <> 'en' OR r.lang <> 'ru' OR p.source_id <> $1`, ds.ID); n != 0 {
		t.Fatalf("пар с перепутанными языками или чужим источником: %d", n)
	}
}

func TestRunTwiceKeepsCounts(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	sample := readSample(t)

	if _, err := importContent(t, pool, sample); err != nil {
		t.Fatal(err)
	}
	first, _ := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "tatoeba", Version: "2026-02-13"})
	if _, err := importContent(t, pool, sample); err != nil {
		t.Fatal(err)
	}
	second, _ := db.New(pool).GetDataSource(ctx, db.GetDataSourceParams{Name: "tatoeba", Version: "2026-02-13"})

	if count(t, pool, `SELECT count(*) FROM data_sources`) != 1 || count(t, pool, `SELECT count(*) FROM sentences`) != 56 || count(t, pool, `SELECT count(*) FROM sentence_pairs`) != 32 {
		t.Fatalf("после второго запуска: sources=%d sentences=%d pairs=%d",
			count(t, pool, `SELECT count(*) FROM data_sources`), count(t, pool, `SELECT count(*) FROM sentences`), count(t, pool, `SELECT count(*) FROM sentence_pairs`))
	}
	if second.ID != first.ID || !second.ImportedAt.Time.After(first.ImportedAt.Time) {
		t.Fatalf("источник %d→%d, imported_at %v→%v", first.ID, second.ID, first.ImportedAt.Time, second.ImportedAt.Time)
	}
}

// После запуска, упавшего на середине, тот же объект источника импортирует корректный файл: состояния между запусками нет.
func TestSameSourceWorksAfterFailedRun(t *testing.T) {
	pool := testdb.Pool(t)
	sample := readSample(t)
	broken := sample + "битая строка без табуляций\n"

	var current atomic.Pointer[[]byte]
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(*current.Load()) }))
	t.Cleanup(srv.Close)
	src := newSource(srv.URL+"/rus-eng.zip", "", 1<<20, 1<<20, srv.Client())
	cache := t.TempDir()

	set := func(content string) {
		z := makeZip(t, entry{"rus.txt", content})
		current.Store(&z)
		src.sha256 = sum(z)
	}
	set(broken)
	if _, err := importer.Run(context.Background(), pool, src, cache); err == nil || !strings.Contains(err.Error(), "строка 35") {
		t.Fatalf("ожидали ошибку про строку 35, получили %v", err)
	}
	assertEmpty(t, pool)

	set(sample)
	if res, err := importer.Run(context.Background(), pool, src, cache); err != nil || res.Rows != 32 {
		t.Fatalf("повторный запуск: %+v, %v", res, err)
	}
	if n := count(t, pool, `SELECT count(*) FROM sentence_pairs`); n != 32 {
		t.Fatalf("пар %d", n)
	}
}

func TestRunWithoutUsablePairsIsAnError(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("word ", 31)) // 31 слово: пара будет пропущена
	tests := map[string]string{
		"пустой файл":                  "",
		"только слишком длинное":       pair(long, "Длинное.", 1, 2, "CM", "CK"),
		"только слишком длинные (две)": pair(long, "Длинное.", 1, 2, "CM", "CK") + pair(long, "Ещё.", 3, 4, "CM", "CK"),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			pool := testdb.Pool(t)
			if _, err := importContent(t, pool, content); err == nil || !strings.Contains(err.Error(), "нет строк") {
				t.Fatalf("ожидали «нет строк», получили %v", err)
			}
			assertEmpty(t, pool)
		})
	}
}

func TestRunConflictsAreErrorsAndRollBack(t *testing.T) {
	tests := map[string]string{
		"то же английское с другим текстом": pair("Go.", "Марш!", 1, 2, "CM", "CK") + pair("Gone.", "Иди.", 1, 3, "CM", "CK"),
		"то же английское с другим автором": pair("Go.", "Марш!", 1, 2, "CM", "CK") + pair("Go.", "Иди.", 1, 3, "XX", "CK"),
		"то же русское с другим текстом":    pair("Go.", "Марш!", 1, 2, "CM", "CK") + pair("Come.", "Иди!", 4, 2, "CM", "CK"),
		"то же русское с другим автором":    pair("Go.", "Марш!", 1, 2, "CM", "CK") + pair("Come.", "Марш!", 4, 2, "CM", "XX"),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			pool := testdb.Pool(t)
			if _, err := importContent(t, pool, content); err == nil || !strings.Contains(err.Error(), "конфликт") {
				t.Fatalf("ожидали ошибку конфликта, получили %v", err)
			}
			assertEmpty(t, pool)
		})
	}
}

func TestRunSameLineTwiceIsNotADuplicate(t *testing.T) {
	pool := testdb.Pool(t)
	line := pair("Go.", "Марш!", 1, 2, "CM", "CK")
	if _, err := importContent(t, pool, line+line); err != nil {
		t.Fatal(err)
	}
	if s, p := count(t, pool, `SELECT count(*) FROM sentences`), count(t, pool, `SELECT count(*) FROM sentence_pairs`); s != 2 || p != 1 {
		t.Fatalf("предложений %d, пар %d; ожидали 2 и 1", s, p)
	}
}

// Предложение, которое уже добавил другой источник, остаётся одной строкой с прежним владельцем.
func TestRunSentenceFromAnotherSourceKeepsOwner(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	other, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "other", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 2877272, Text: "Go.", Author: "CM", WordCount: 1, SourceID: other.ID}); err != nil {
		t.Fatal(err)
	}

	if _, err := importContent(t, pool, readSample(t)); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, `SELECT count(*) FROM sentences`); n != 56 {
		t.Fatalf("предложений %d, ожидали 56 (общее не должно дублироваться)", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM sentences WHERE tatoeba_id = 2877272 AND lang = 'en' AND source_id = $1`, other.ID); n != 1 {
		t.Fatalf("общее предложение потеряло прежнего владельца")
	}
}

func TestRunWrongChecksumTouchesNothing(t *testing.T) {
	pool := testdb.Pool(t)
	z := makeZip(t, entry{"rus.txt", readSample(t)})
	src, _ := serve(t, z, sum([]byte("другое")), 1<<20)
	cache := t.TempDir()
	if _, err := importer.Run(context.Background(), pool, src, cache); err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("ожидали ошибку суммы, получили %v", err)
	}
	assertEmpty(t, pool)
}
