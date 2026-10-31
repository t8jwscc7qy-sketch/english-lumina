package testdb

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestTextQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1.5", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	word := func(lemma, pos string, rank int, translations []string) int64 {
		t.Helper()
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: pos, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO kb_lexeme_stat (lexeme_id, freq_count, freq_rank) VALUES ($1, 1, $2)`, id, rank); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(map[string]any{"lemma": lemma, "translations": translations})
		if _, err := pool.Exec(ctx, `INSERT INTO kb_teaching_card (lexeme_id, payload) VALUES ($1, $2)`, id, payload); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := word("apple", "noun", 30, []string{"яблоко"})
	b := word("bridge", "noun", 10, []string{"мост"})
	known := word("house", "noun", 5, []string{"дом"})
	_ = word("nothing", "noun", 1, []string{})      // нет перевода
	_ = word("Paris", "noun", 2, []string{"Париж"}) // не строчное
	_ = word("under", "preposition", 3, []string{"под"})
	if _, err := pool.Exec(ctx, `INSERT INTO word_learning (user_id, lexeme_id, status, source, learned_at) VALUES ($1, $2, 'learned', 'lesson', now())`, user.ID, known); err != nil {
		t.Fatal(err)
	}

	// Целевые слова: по частоте, без известных, без слов без перевода, не строчных и не из основных частей речи.
	targets, err := q.TextTargetWords(ctx, db.TextTargetWordsParams{UserID: user.ID, MaxRows: 10})
	if err != nil || len(targets) != 2 || targets[0].LexemeID != b || targets[1].LexemeID != a {
		t.Fatalf("целевые слова: %+v %v", targets, err)
	}
	if limited, _ := q.TextTargetWords(ctx, db.TextTargetWordsParams{UserID: user.ID, MaxRows: 1}); len(limited) != 1 {
		t.Fatalf("лимит: %d", len(limited))
	}

	// Подсказки по лемме.
	hints, err := q.TextWordHints(ctx, []string{"apple", "house", "missing"})
	if err != nil || len(hints) != 2 {
		t.Fatalf("подсказки: %+v %v", hints, err)
	}

	// Текст: вставка, чтение владельцем, последние предложения.
	body, _ := json.Marshal(map[string]any{"sentences": []map[string]any{{"id": 11}, {"id": 12}, {"id": 11}}})
	id, err := q.TextInsert(ctx, db.TextInsertParams{UserID: &user.ID, Kind: "stitched", Title: "Текст", Body: body, WordCount: 30, CoverageAtBuild: 0.86, Goal: 0.85})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.TextGet(ctx, db.TextGetParams{ID: id, UserID: &user.ID})
	if err != nil || got.Kind != "stitched" || got.WordCount != 30 {
		t.Fatalf("текст: %+v %v", got, err)
	}
	other := user.ID + 1
	if _, err := q.TextGet(ctx, db.TextGetParams{ID: id, UserID: &other}); err == nil {
		t.Fatal("чужой текст не читается")
	}
	ids, err := q.TextRecentSentenceIDs(ctx, db.TextRecentSentenceIDsParams{UserID: &user.ID, MaxTexts: 5})
	if err != nil || len(ids) != 2 {
		t.Fatalf("последние предложения: %v %v", ids, err)
	}

	// Последний показанный неоценённый текст.
	if err := q.TextShownInsert(ctx, db.TextShownInsertParams{UserID: user.ID, TextID: id, Coverage: 0.86}); err != nil {
		t.Fatal(err)
	}
	last, err := q.TextLastUnrated(ctx, user.ID)
	if err != nil || last.ID != id {
		t.Fatalf("последний неоценённый: %+v %v", last, err)
	}
	// Текст без номеров предложений (будущий авторский) не ломает запрос последних предложений.
	noIDs, _ := json.Marshal(map[string]any{"sentences": []map[string]any{{"en": "x"}}})
	noIDsID, err := q.TextInsert(ctx, db.TextInsertParams{UserID: &user.ID, Kind: "stitched", Title: "Без номеров", Body: noIDs, WordCount: 5, CoverageAtBuild: 0.9, Goal: 0.85})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.TextShownInsert(ctx, db.TextShownInsertParams{UserID: user.ID, TextID: noIDsID, Coverage: 0.9}); err != nil {
		t.Fatal(err)
	}
	if ids2, err := q.TextRecentSentenceIDs(ctx, db.TextRecentSentenceIDsParams{UserID: &user.ID, MaxTexts: 5}); err != nil || len(ids2) != 2 {
		t.Fatalf("без номеров: %v %v", ids2, err)
	}

	// Оценка один раз на текст.
	rate := db.TextRateInsertParams{UserID: user.ID, TextID: id, Mode: "read", Understood: "easy", CoverageAtRead: 0.86}
	if n, err := q.TextRateInsert(ctx, rate); err != nil || n != 1 {
		t.Fatalf("оценка: %d %v", n, err)
	}
	if l, err := q.TextLastUnrated(ctx, user.ID); err != nil || l.ID == id || l.Title != "Без номеров" {
		t.Fatalf("оценённый текст больше не «неоценённый»: %+v %v", l, err)
	}
	if n, err := q.TextRateInsert(ctx, rate); err != nil || n != 0 {
		t.Fatalf("повторная оценка: %d %v", n, err)
	}
	if _, err := q.TextRateInsert(ctx, db.TextRateInsertParams{UserID: user.ID, TextID: id, Mode: "write", Understood: "easy", CoverageAtRead: 0.5}); err == nil {
		t.Fatal("режим вне read/listen отклоняется")
	}

	// Цель понимания: границы 0,80 и 0,90 проходят (real хранит неточно), 0,79 и 0,91 — нет.
	for _, v := range []float32{0.80, 0.90, 0.85} {
		if err := q.TextSetCoverage(ctx, db.TextSetCoverageParams{UserID: user.ID, TextCoverage: v}); err != nil {
			t.Fatalf("цель %v допустима: %v", v, err)
		}
	}
	for _, v := range []float32{0.79, 0.91} {
		if err := q.TextSetCoverage(ctx, db.TextSetCoverageParams{UserID: user.ID, TextCoverage: v}); err == nil {
			t.Fatalf("цель %v вне границ", v)
		}
	}
	p, err := q.LearnGetPreferences(ctx, user.ID)
	if err != nil || p.TextCoverage < 0.849 || p.TextCoverage > 0.851 {
		t.Fatalf("сохранённая цель: %+v %v", p, err)
	}
}

func TestAuthoredTextQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	slug, band := "t100-001", int16(100)
	body, _ := json.Marshal(map[string]any{"sentences": []map[string]any{{"en": "A dog.", "ru": "Собака."}}})
	id1, err := q.TextUpsertAuthored(ctx, db.TextUpsertAuthoredParams{Slug: &slug, Band: &band, Title: "Первый", Body: body, WordCount: 40})
	if err != nil {
		t.Fatal(err)
	}
	// Повторная запись того же slug обновляет текст и возвращает тот же номер.
	id2, err := q.TextUpsertAuthored(ctx, db.TextUpsertAuthoredParams{Slug: &slug, Band: &band, Title: "Первый (правка)", Body: body, WordCount: 41})
	if err != nil || id2 != id1 {
		t.Fatalf("upsert: %d %d %v", id1, id2, err)
	}
	var title string
	if err := pool.QueryRow(ctx, `SELECT title FROM texts WHERE id = $1`, id1).Scan(&title); err != nil || title != "Первый (правка)" {
		t.Fatalf("заголовок после правки: %q %v", title, err)
	}
	// Авторский текст без slug и склеенный со slug не допускаются.
	if _, err := pool.Exec(ctx, `INSERT INTO texts (kind, title, body, word_count, coverage_at_build, goal) VALUES ('authored', 'x', '{}', 5, 0, 0.85)`); err == nil {
		t.Fatal("авторский текст без slug отклоняется")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO texts (kind, slug, title, body, word_count, coverage_at_build, goal) VALUES ('stitched', 'zz', 'x', '{}', 5, 0, 0.85)`); err == nil {
		t.Fatal("склеенный текст со slug отклоняется")
	}
	// Показ и оценка: коврейдж на момент показа попадает в оценку, оценённый текст не «неоценённый» и виден в прочитанных.
	if g, err := q.TextGet(ctx, db.TextGetParams{ID: id1, UserID: &user.ID}); err != nil || g.Kind != "authored" {
		t.Fatalf("авторский текст читается любым пользователем: %+v %v", g, err)
	}
	other := user.ID + 1
	if _, err := q.TextGet(ctx, db.TextGetParams{ID: id1, UserID: &other}); err != nil {
		t.Fatalf("авторский текст общий: %v", err)
	}
	if err := q.TextShownInsert(ctx, db.TextShownInsertParams{UserID: user.ID, TextID: id1, Coverage: 0.83}); err != nil {
		t.Fatal(err)
	}
	if err := q.TextShownInsert(ctx, db.TextShownInsertParams{UserID: user.ID, TextID: id1, Coverage: 0.5}); err != nil {
		t.Fatalf("повторный показ ничего не ломает: %v", err)
	}
	if last, err := q.TextLastUnrated(ctx, user.ID); err != nil || last.ID != id1 || last.Kind != "authored" {
		t.Fatalf("неоценённый авторский: %+v %v", last, err)
	}
	if n, err := q.TextRateInsert(ctx, db.TextRateInsertParams{UserID: user.ID, TextID: id1, Mode: "read", Understood: "ok", CoverageAtRead: 0}); err != nil || n != 1 {
		t.Fatalf("оценка: %d %v", n, err)
	}
	var cov float32
	if err := pool.QueryRow(ctx, `SELECT coverage_at_read FROM text_reads WHERE text_id = $1`, id1).Scan(&cov); err != nil || cov < 0.82 || cov > 0.84 {
		t.Fatalf("покрытие при чтении берётся из показа: %v %v", cov, err)
	}
	if _, err := q.TextLastUnrated(ctx, user.ID); err == nil {
		t.Fatal("после оценки неоценённых нет")
	}
	if ids, err := q.TextReadIDs(ctx, user.ID); err != nil || len(ids) != 1 || ids[0] != id1 {
		t.Fatalf("прочитанные: %v %v", ids, err)
	}
}

func TestSentenceAudioQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba-audio", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sentences (lang, tatoeba_id, text, author, word_count, source_id) VALUES ('en', 11, 'Hello there my friend.', 'tom', 4, $1), ('ru', 12, 'Привет.', 'ivan', 1, $1)`, src.ID); err != nil {
		t.Fatal(err)
	}
	up := func(sentence, audio int64, user, lic string) int64 {
		n, err := q.UpsertSentenceAudio(ctx, db.UpsertSentenceAudioParams{AudioID: audio, SentenceTatoebaID: sentence, Username: user, License: lic, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if up(11, 500, "alice", "CC BY 4.0") != 1 || up(11, 501, "bob", "CC0 1.0") != 1 {
		t.Fatal("запись английского предложения загружается")
	}
	if up(11, 500, "alice", "CC BY 4.0") != 0 {
		t.Fatal("повторная запись ничего не добавляет")
	}
	if up(999, 502, "carol", "CC BY 4.0") != 0 || up(12, 503, "dave", "CC BY 4.0") != 0 {
		t.Fatal("записи несуществующих и русских предложений не нужны")
	}
	rows, err := q.SentenceAudioFor(ctx, db.SentenceAudioForParams{Ids: []int64{11, 12, 999}, Licenses: []string{"CC BY 4.0", "CC0 1.0"}})
	if err != nil || len(rows) != 1 || rows[0].AudioID != 500 || rows[0].Username != "alice" {
		t.Fatalf("одна запись на предложение, с наименьшим номером: %+v %v", rows, err)
	}
	only, err := q.SentenceAudioFor(ctx, db.SentenceAudioForParams{Ids: []int64{11}, Licenses: []string{"CC0 1.0"}})
	if err != nil || len(only) != 1 || only[0].AudioID != 501 {
		t.Fatalf("лицензия ограничивает выбор: %+v %v", only, err)
	}
	if none, _ := q.SentenceAudioFor(ctx, db.SentenceAudioForParams{Ids: []int64{11}, Licenses: []string{"CC BY-NC 4.0"}}); len(none) != 0 {
		t.Fatal("запись с неразрешённой лицензией не отдаётся")
	}
}
