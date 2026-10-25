package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/kaikki"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

// Путь считается при старте пакета: testdb.Pool меняет рабочий каталог.
var samplePath, _ = filepath.Abs("../../internal/kb/testdata/kaikki-sample.jsonl")

func sampleReader(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// seed кладёт в временную схему слова, частоты и старый словарь (как после импортов) и возвращает номера слов.
func seed(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	q := db.New(pool)
	src := func(name, version string) int64 {
		ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: name, Version: version, License: "l", Url: "u", Attribution: "a"})
		if err != nil {
			t.Fatal(err)
		}
		n := int32(1)
		if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: ds.ID, RowCount: &n}); err != nil {
			t.Fatal(err)
		}
		return ds.ID
	}
	cefr, fdict, fen, fru := src("cefrj", "1.5"), src("freedict-eng-rus", "1"), src("frequencywords", "2018"), src("frequencywords-ru", "2018")
	_ = src(kaikki.Name, kaikki.DumpDate)

	ids := map[string]int64{}
	for _, l := range []struct{ lemma, pos, level string }{
		{"dad", "noun", "A1"}, {"clothes", "noun", "A1"}, {"blow", "verb", "B1"}, {"thin", "adjective", "A2"},
		{"colour/color", "noun", "A2"}, {"nothing", "pronoun", "A1"}, {"ghost", "noun", "C1"}, {"father", "noun", "A1"},
	} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: l.lemma, Pos: l.pos, SourceID: cefr})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: cefr, Level: l.level, Extra: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}
		ids[l.lemma] = id
	}
	for _, w := range []struct {
		lemma, pos string
		sense      int16
		ru         string
	}{{"dad", "n", 1, "папа"}, {"dad", "n", 1, "батюшка"}, {"clothes", "n", 1, "одёжа"}, {"thin", "adj", 1, "редкий"}, {"ghost", "n", 1, "призрак"}} {
		if err := q.UpsertWordTranslation(ctx, db.UpsertWordTranslationParams{SourceID: fdict, Lemma: w.lemma, Pos: w.pos, SenseNo: w.sense, Ru: w.ru}); err != nil {
			t.Fatal(err)
		}
	}
	rank := 0
	for word, count := range map[string]int64{"dad": 1000, "dads": 100, "blow": 500, "thin": 300, "clothes": 400, "nothing": 5000, "father": 2000} {
		rank++
		if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: fen, Word: word, Count: count, Rank: int32(rank)}); err != nil { //nolint:gosec // малое число
			t.Fatal(err)
		}
	}
	for i, w := range []string{"папа", "дуть", "тонкий", "редкий", "одежда", "призрак"} {
		if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: fru, Word: w, Count: 100, Rank: int32(100 + i*1000)}); err != nil { //nolint:gosec // малое число
			t.Fatal(err)
		}
	}
	return ids
}

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var kbTables = []string{"kb_sense", "kb_translation", "kb_pronunciation", "kb_form", "kb_relation", "kb_lexeme_stat", "kb_teaching_card"}

func counts(t *testing.T, pool *pgxpool.Pool) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, tb := range kbTables {
		out[tb] = count(t, pool, tb)
	}
	return out
}

func chosen(t *testing.T, pool *pgxpool.Pool, lexID int64) []string {
	t.Helper()
	rows, err := db.New(pool).KbListChosenTranslations(context.Background(), db.KbListChosenTranslationsParams{Lang: "ru", LexemeIds: []int64{lexID}})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, r.Text)
	}
	return out
}

func equal(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") }

func TestBuildWritesKb(t *testing.T) {
	pool := testdb.Pool(t)
	ids := seed(t, pool)
	rep, err := Build(context.Background(), pool, sampleReader(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := chosen(t, pool, ids["dad"]); !equal(got, []string{"папа", "бать"}) {
		t.Fatalf("переводы dad: %v", got)
	}
	if got := chosen(t, pool, ids["blow"]); !equal(got, []string{"дуть"}) {
		t.Fatalf("переводы blow: %v", got)
	}
	if got := chosen(t, pool, ids["ghost"]); !equal(got, []string{"призрак"}) {
		t.Fatalf("слово только из старого словаря: %v", got)
	}
	cards, err := db.New(pool).KbGetTeachingCards(context.Background(), []int64{ids["dad"], ids["nothing"]})
	if err != nil || len(cards) != 1 {
		t.Fatalf("карточки: %v %v", cards, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(cards[0].Payload, &payload); err != nil || payload["lemma"] != "dad" || payload["ipa"] != "dæd" {
		t.Fatalf("карточка dad: %v (%v)", payload, err)
	}
	var pron int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM kb_pronunciation WHERE lexeme_id = $1 AND accent = 'us' AND is_primary`, ids["dad"]).Scan(&pron); err != nil || pron != 1 {
		t.Fatalf("американское произношение dad: %d (%v)", pron, err)
	}
	b, err := db.New(pool).KbLatestBuild(context.Background())
	if err != nil || b.Status != "ok" || !b.FinishedAt.Valid {
		t.Fatalf("запись о сборке: %+v (%v)", b, err)
	}
	if rep.Golden.Checked < 4 {
		t.Fatalf("отчёт: %+v", rep.Golden)
	}
}

func TestBuildIsIdempotent(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	if _, err := Build(context.Background(), pool, sampleReader(t), Options{}); err != nil {
		t.Fatal(err)
	}
	first := counts(t, pool)
	if _, err := Build(context.Background(), pool, sampleReader(t), Options{}); err != nil {
		t.Fatal(err)
	}
	second := counts(t, pool)
	for tb, n := range first {
		if n == 0 {
			t.Errorf("таблица %s пуста после сборки", tb)
		}
		if second[tb] != n {
			t.Errorf("%s: после второй сборки %d строк, было %d", tb, second[tb], n)
		}
	}
}

func TestBuildRollsBackBelowThreshold(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	if _, err := Build(context.Background(), pool, sampleReader(t), Options{}); err != nil {
		t.Fatal(err)
	}
	before := counts(t, pool)
	_, err := Build(context.Background(), pool, nil, Options{Check: kb.Options{MinCoverage: map[string]float64{"A1": 0.99}}})
	if !errors.Is(err, kb.ErrBelowThreshold) {
		t.Fatalf("ждали ErrBelowThreshold, получили %v", err)
	}
	for tb, n := range before {
		if got := count(t, pool, tb); got != n {
			t.Errorf("%s: %d строк, было %d (сборка с просевшим качеством не должна менять данные)", tb, got, n)
		}
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM kb_build ORDER BY id DESC LIMIT 1`).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("статус последней сборки %q (%v)", status, err)
	}
}

func TestBuildReaderSeesOldDataDuringBuild(t *testing.T) {
	pool := testdb.Pool(t)
	ids := seed(t, pool)
	if _, err := Build(context.Background(), pool, nil, Options{}); err != nil { // сначала только старый словарь
		t.Fatal(err)
	}
	if got := chosen(t, pool, ids["dad"]); !equal(got, []string{"папа"}) {
		t.Fatalf("после первой сборки: %v", got)
	}
	paused, release := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	var buildErr error
	go func() {
		defer wg.Done()
		_, buildErr = Build(context.Background(), pool, sampleReader(t), Options{BeforeCommit: func() {
			close(paused)
			<-release
		}})
	}()
	select {
	case <-paused:
	case <-time.After(30 * time.Second):
		t.Fatal("сборка не дошла до фиксации")
	}
	if got := chosen(t, pool, ids["dad"]); !equal(got, []string{"папа"}) {
		t.Fatalf("во время сборки читатель должен видеть прежние данные, видит %v", got)
	}
	close(release)
	wg.Wait()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	if got := chosen(t, pool, ids["dad"]); !equal(got, []string{"папа", "бать"}) {
		t.Fatalf("после фиксации: %v", got)
	}
}

func TestBuildDoesNotTouchUserTables(t *testing.T) {
	pool := testdb.Pool(t)
	ids := seed(t, pool)
	ctx := context.Background()
	q := db.New(pool)
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO cards (user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, remaining_steps) VALUES ($1, $2, now(), 1, 1, 0, 1, 0, 1, 0)`, user.ID, ids["dad"]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO events (user_id, type, schema_version, occurred_at, payload) VALUES ($1, 'card.added', 1, now(), '{}')`, user.ID); err != nil {
		t.Fatal(err)
	}
	before := map[string]int{"cards": count(t, pool, "cards"), "events": count(t, pool, "events"), "lexemes": count(t, pool, "lexemes"), "users": count(t, pool, "users")}
	if _, err := Build(ctx, pool, sampleReader(t), Options{}); err != nil {
		t.Fatal(err)
	}
	for tb, n := range before {
		if got := count(t, pool, tb); got != n {
			t.Errorf("%s: %d строк, было %d", tb, got, n)
		}
	}
}

func TestBuildWithoutKaikkiKeepsLegacyTranslations(t *testing.T) {
	pool := testdb.Pool(t)
	ids := seed(t, pool)
	if _, err := Build(context.Background(), pool, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := chosen(t, pool, ids["dad"]); !equal(got, []string{"папа"}) {
		t.Fatalf("без Kaikki остаётся старый словарь: %v", got)
	}
	if count(t, pool, "kb_sense") != 0 || count(t, pool, "kb_pronunciation") != 0 {
		t.Fatal("без Kaikki значений и произношений нет")
	}
}

func TestBuildBrokenKaikkiLineChangesNothing(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	if _, err := Build(context.Background(), pool, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	before := counts(t, pool)
	if _, err := Build(context.Background(), pool, strings.NewReader("{\"word\":\"dad\"}\nне json\n"), Options{}); err == nil {
		t.Fatal("битая строка должна давать ошибку")
	}
	for tb, n := range before {
		if got := count(t, pool, tb); got != n {
			t.Errorf("%s: %d строк, было %d", tb, got, n)
		}
	}
}

func TestBuildRecordsSourcesAndReport(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	if _, err := Build(context.Background(), pool, sampleReader(t), Options{Sources: map[string]any{"kaikki": map[string]string{"version": kaikki.DumpDate}}}); err != nil {
		t.Fatal(err)
	}
	b, err := db.New(pool).KbLatestBuild(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sources map[string]map[string]string
	if err := json.Unmarshal(b.Sources, &sources); err != nil || sources["kaikki"]["version"] != kaikki.DumpDate {
		t.Fatalf("источники: %s (%v)", b.Sources, err)
	}
	var report struct{ Levels map[string]kb.LevelStats }
	if err := json.Unmarshal(b.Report, &report); err != nil || report.Levels["A1"].Total != 4 {
		t.Fatalf("отчёт: %s (%v)", b.Report, err)
	}
}

func TestPreviewDoesNotWrite(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	rep, err := Preview(context.Background(), pool, sampleReader(t))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Golden.Checked < 4 {
		t.Fatalf("отчёт: %+v", rep.Golden)
	}
	for tb, n := range counts(t, pool) {
		if n != 0 {
			t.Errorf("%s: %d строк после просмотра без записи", tb, n)
		}
	}
	if count(t, pool, "kb_build") != 0 {
		t.Fatal("просмотр без записи не должен создавать запись о сборке")
	}
}

func TestPrintReport(t *testing.T) {
	pool := testdb.Pool(t)
	seed(t, pool)
	rep, err := Preview(context.Background(), pool, sampleReader(t))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := PrintReport(&sb, rep); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	for _, want := range []string{"A1", "C1", "Контрольный список", "father", "Выборка"} {
		if !strings.Contains(out, want) {
			t.Errorf("в отчёте нет %q:\n%s", want, out)
		}
	}
}
