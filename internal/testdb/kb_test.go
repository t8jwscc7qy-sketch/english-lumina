package testdb

import (
	"context"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

var kbTables = []string{"kb_build", "kb_sense", "kb_translation", "kb_pronunciation", "kb_form", "kb_relation", "kb_lexeme_stat", "kb_teaching_card"}

// Миграция 00014: таблицы базы знаний kb_* создаются и удаляются, не затрагивая данные пользователя и слова.
func TestKbTables(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	for _, name := range kbTables {
		if !tableExists(t, pool, name) {
			t.Fatalf("после Up нет таблицы %s", name)
		}
	}

	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "kaikki-wiktextract", Version: "2026-10-01", License: "CC BY-SA", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	var lexID int64
	if err := pool.QueryRow(ctx, `INSERT INTO lexemes (lemma, pos, source_id) VALUES ('dad', 'noun', $1) RETURNING id`, src.ID).Scan(&lexID); err != nil {
		t.Fatal(err)
	}
	ins := func(sql string, args ...any) error { _, err := pool.Exec(ctx, sql, append([]any{lexID}, args...)...); return err }
	insTr := `INSERT INTO kb_translation (lexeme_id, lang, text, score, rank, chosen, flags, source_ids) VALUES ($1, 'ru', $2, 5.0, $3, $4, '{}', ARRAY[]::bigint[])`
	if err := ins(insTr, "папа", 1, true); err != nil {
		t.Fatalf("выбранный перевод: %v", err)
	}
	if err := ins(insTr, "отец", 2, true); err != nil {
		t.Fatalf("второй выбранный перевод: %v", err)
	}
	if err := ins(insTr, "батюшка", 3, false); err != nil {
		t.Fatalf("невыбранный перевод с любым рангом: %v", err)
	}
	if err := ins(insTr, "тятя", 3, true); err == nil {
		t.Fatal("выбранных переводов не больше двух (rank 1 или 2)")
	}
	if err := ins(insTr, "папка", 1, true); err == nil {
		t.Fatal("два выбранных перевода с одним рангом отклоняются")
	}
	if err := ins(`INSERT INTO kb_pronunciation (lexeme_id, accent, ipa, audio_ref, is_primary, source_id) VALUES ($1, 'xx', 'dæd', '', true, $2)`, src.ID); err == nil {
		t.Fatal("акцент вне us/uk/пусто отклоняется")
	}
	if err := ins(`INSERT INTO kb_relation (lexeme_id, kind, other_lemma) VALUES ($1, 'hypernym', 'father')`); err == nil {
		t.Fatal("вид связи только synonym или antonym")
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO kb_teaching_card (lexeme_id, payload) VALUES ($1, '{"lemma":"dad"}')`, lexID); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, downSQL(t, "00014_kb_tables.sql")); err != nil {
		t.Fatalf("откат: %v", err)
	}
	for _, name := range kbTables {
		if tableExists(t, pool, name) {
			t.Fatalf("после отката осталась таблица %s", name)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lexemes WHERE id = $1`, lexID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("слово после отката: %d (%v)", n, err)
	}
	if _, err := q.GetUserByLogin(ctx, "vlad"); err != nil || user.ID == 0 {
		t.Fatalf("пользователь после отката: %v", err)
	}
}
