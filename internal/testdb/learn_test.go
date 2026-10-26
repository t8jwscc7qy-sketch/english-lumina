package testdb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func upSQL(t *testing.T, name string) string {
	t.Helper()
	for _, m := range readMigrations(t) {
		if m.name == name {
			return m.up
		}
	}
	t.Fatalf("миграция %s не найдена", name)
	return ""
}

func TestLearnTables(t *testing.T) {
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
	word := func(lemma, level string, rank int, translations []string) int64 {
		t.Helper()
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: "noun", SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: src.ID, Level: level, Extra: json.RawMessage(`{}`)}); err != nil {
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
	a := word("alpha", "A1", 10, []string{"альфа"})
	b := word("bravo", "A1", 5, []string{"браво"})
	c := word("charlie", "A1", 1, []string{}) // нет перевода
	d := word("delta", "A2", 1, []string{"дельта"})
	e := word("echo", "A1", 2, []string{"эхо"}) // есть карточка
	f := word("foxtrot", "A1", 3, []string{"фокстрот"})
	if _, err := pool.Exec(ctx, `INSERT INTO cards (user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, remaining_steps) VALUES ($1, $2, now(), 1, 1, 0, 1, 0, 1, 0)`, user.ID, e); err != nil {
		t.Fatal(err)
	}

	// Ограничения таблицы.
	now := time.Now()
	if _, err := pool.Exec(ctx, `INSERT INTO word_learning (user_id, lexeme_id, status, source) VALUES ($1, $2, 'bogus', 'lesson')`, user.ID, f); err == nil {
		t.Fatal("статус вне списка отклоняется")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO word_learning (user_id, lexeme_id, status, source) VALUES ($1, $2, 'learned', 'lesson')`, user.ID, f); err == nil {
		t.Fatal("learned без learned_at отклоняется")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO word_learning (user_id, lexeme_id, status, source) VALUES ($1, $2, 'introduced', 'wish')`, user.ID, f); err == nil {
		t.Fatal("источник вне списка отклоняется")
	}

	// Кандидаты: по уровню, затем по рангу частоты; без слов с карточкой, с изученным состоянием и без перевода.
	if n, err := q.LearnInsertIntroduced(ctx, db.LearnInsertIntroducedParams{UserID: user.ID, LexemeID: f, IntroducedAt: ts(now)}); err != nil || n != 1 {
		t.Fatalf("введено слово: %d %v", n, err)
	}
	if n, err := q.LearnInsertIntroduced(ctx, db.LearnInsertIntroducedParams{UserID: user.ID, LexemeID: f, IntroducedAt: ts(now)}); err != nil || n != 0 {
		t.Fatalf("повторный ввод ничего не добавляет: %d %v", n, err)
	}
	cands, err := q.LearnCandidates(ctx, db.LearnCandidatesParams{UserID: user.ID, Levels: []string{"A1", "A2"}, MaxRows: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, r := range cands {
		got = append(got, r.LexemeID)
	}
	if len(got) != 3 || got[0] != b || got[1] != a || got[2] != d {
		t.Fatalf("кандидаты %v, ждали [bravo alpha delta] = [%d %d %d]", got, b, a, d)
	}
	if cands[0].Level != "A1" || cands[2].Level != "A2" || !strings.Contains(string(cands[0].Payload), "браво") {
		t.Fatalf("уровень и карточка: %+v", cands[0])
	}
	_ = c

	// Переходы в «изучено».
	if n, err := q.LearnMarkLearned(ctx, db.LearnMarkLearnedParams{UserID: user.ID, LexemeID: f, Now: ts(now)}); err != nil || n != 1 {
		t.Fatalf("introduced → learned: %d %v", n, err)
	}
	if n, err := q.LearnMarkLearned(ctx, db.LearnMarkLearnedParams{UserID: user.ID, LexemeID: f, Now: ts(now)}); err != nil || n != 0 {
		t.Fatalf("повторно ничего не меняется: %d %v", n, err)
	}
	if n, err := q.LearnMarkLearned(ctx, db.LearnMarkLearnedParams{UserID: user.ID, LexemeID: a, Now: ts(now)}); err != nil || n != 1 {
		t.Fatalf("нет строки → learned/card: %d %v", n, err)
	}
	var status, source string
	if err := pool.QueryRow(ctx, `SELECT status, source FROM word_learning WHERE user_id = $1 AND lexeme_id = $2`, user.ID, a).Scan(&status, &source); err != nil || status != "learned" || source != "card" {
		t.Fatalf("строка слова: %s/%s (%v)", status, source, err)
	}
	if n, err := q.LearnMarkKnown(ctx, db.LearnMarkKnownParams{UserID: user.ID, LexemeID: b}); err != nil || n != 1 {
		t.Fatalf("known_at_start: %d %v", n, err)
	}
	if n, err := q.LearnMarkKnown(ctx, db.LearnMarkKnownParams{UserID: user.ID, LexemeID: b}); err != nil || n != 0 {
		t.Fatalf("повторно: %d %v", n, err)
	}
	counts, err := q.LearnStatusCounts(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	byStatus := map[string]int64{}
	for _, r := range counts {
		byStatus[r.Status] = r.Count
	}
	if byStatus["learned"] != 2 || byStatus["known_at_start"] != 1 || byStatus["introduced"] != 0 {
		t.Fatalf("по статусам: %v", byStatus)
	}
	if n, err := q.LearnIntroducedSince(ctx, db.LearnIntroducedSinceParams{UserID: user.ID, Since: ts(now.Add(-time.Hour))}); err != nil || n != 1 {
		t.Fatalf("введено за час: %d %v", n, err)
	}
	if n, err := q.LearnIntroducedSince(ctx, db.LearnIntroducedSinceParams{UserID: user.ID, Since: ts(now.Add(time.Hour))}); err != nil || n != 0 {
		t.Fatalf("введено после: %d %v", n, err)
	}

	// Точность повторений из журнала.
	for i, rating := range []int{1, 3, 4, 2} {
		payload, _ := json.Marshal(map[string]any{"rating": rating})
		if _, err := pool.Exec(ctx, `INSERT INTO events (user_id, type, schema_version, occurred_at, payload) VALUES ($1, 'card.reviewed', 1, $2, $3)`, user.ID, now.Add(-time.Duration(i)*time.Minute), payload); err != nil {
			t.Fatal(err)
		}
	}
	st, err := q.LearnReviewStats(ctx, db.LearnReviewStatsParams{UserID: user.ID, Since: ts(now.Add(-time.Hour))})
	if err != nil || st.Total != 4 || st.Good != 2 {
		t.Fatalf("точность: %+v %v", st, err)
	}

	// Настройки обучения: нет строки — запрос возвращает её отсутствие, Upsert создаёт и обновляет.
	if _, err := q.LearnGetPreferences(ctx, user.ID); err == nil {
		t.Fatal("до сохранения строки настроек нет")
	}
	p, err := q.LearnUpsertPreferences(ctx, db.LearnUpsertPreferencesParams{UserID: user.ID, NewWordsMode: "manual", NewWordsPerDay: 9, CampCoverage: 0.97})
	if err != nil || p.NewWordsMode != "manual" || p.NewWordsPerDay != 9 {
		t.Fatalf("настройки: %+v %v", p, err)
	}
	if _, err := q.LearnUpsertPreferences(ctx, db.LearnUpsertPreferencesParams{UserID: user.ID, NewWordsMode: "turbo", NewWordsPerDay: 9, CampCoverage: 0.95}); err == nil {
		t.Fatal("режим вне auto/manual отклоняется")
	}
	if _, err := q.LearnUpsertPreferences(ctx, db.LearnUpsertPreferencesParams{UserID: user.ID, NewWordsMode: "auto", NewWordsPerDay: 31, CampCoverage: 0.95}); err == nil {
		t.Fatal("число слов в день от 1 до 30")
	}
	if _, err := q.LearnUpsertPreferences(ctx, db.LearnUpsertPreferencesParams{UserID: user.ID, NewWordsMode: "auto", NewWordsPerDay: 10, CampCoverage: 0.5}); err == nil {
		t.Fatal("покрытие от 0,90 до 0,99")
	}
}

// Перенос старых карточек: карточка в состоянии «повторение» даёт изученное слово, остальные нет.
func TestLearnBackfillFromCards(t *testing.T) {
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
	ids := map[string]int64{}
	for lemma, state := range map[string]int{"review": 2, "learning": 1, "relearning": 3, "fresh": 0} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: "noun", SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		ids[lemma] = id
		if _, err := pool.Exec(ctx, `INSERT INTO cards (user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, remaining_steps) VALUES ($1, $2, now(), 1, 1, 0, 1, 0, $3, 0)`, user.ID, id, state); err != nil {
			t.Fatal(err)
		}
	}
	// Миграция уже применена к пустым карточкам: откатываем таблицы и накатываем снова, чтобы перенос увидел карточки.
	if _, err := pool.Exec(ctx, downSQL(t, "00015_word_learning.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, upSQL(t, "00015_word_learning.sql")); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT lexeme_id, status, source FROM word_learning WHERE user_id = $1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id int64
		var status, source string
		if err := rows.Scan(&id, &status, &source); err != nil {
			t.Fatal(err)
		}
		n++
		if id != ids["review"] || status != "learned" || source != "legacy_card" {
			t.Fatalf("перенесена не та строка: %d %s %s", id, status, source)
		}
	}
	if n != 1 {
		t.Fatalf("перенесено %d строк, ждали 1 (только карточка в состоянии повторения)", n)
	}
}
