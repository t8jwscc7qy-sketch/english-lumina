package testdb

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestExercisesFormKind(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I went home yesterday.", Author: "a", WordCount: 4, SourceID: src.ID})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(kind string) error {
		_, err := pool.Exec(ctx, `INSERT INTO exercises (user_id, kind, sentence_id, topics, payload, answer)
			VALUES ($1, $2, $3, ARRAY['skill:form'], '{}', '{}')`, u.ID, kind, s.ID)
		return err
	}
	for _, k := range []string{"form", "order", "gap"} {
		if err := insert(k); err != nil {
			t.Fatalf("вид %s должен быть допустим: %v", k, err)
		}
	}
	if err := insert("x"); err == nil {
		t.Fatal("неизвестный вид должен отклоняться")
	}

	var down string
	for _, m := range readMigrations(t) {
		if strings.HasPrefix(m.name, "00009") {
			down = m.down
		}
	}
	if down == "" {
		t.Fatal("миграция 00009 не найдена")
	}
	if _, err := pool.Exec(ctx, down); err == nil {
		t.Fatal("откат с упражнением вида form должен падать")
	} else if !strings.Contains(err.Error(), "form") {
		t.Fatalf("ошибка отката должна называть причину: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM exercises WHERE kind = 'form'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("откат без упражнений form: %v", err)
	}
	if err := insert("form"); err == nil {
		t.Fatal("после отката вид form не должен быть допустим")
	}
	if err := insert("order"); err != nil {
		t.Fatalf("order должен остаться допустимым: %v", err)
	}
}

func TestListWeakCardWords(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	me, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	// В схеме разрешён один пользователь; для проверки «чужих» карточек снимаем ограничение во временной схеме.
	if _, err := pool.Exec(ctx, `DROP INDEX users_single_row`); err != nil {
		t.Fatal(err)
	}
	other, err := q.CreateUser(ctx, db.CreateUserParams{Login: "other", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	lex := func(lemma, pos string) int64 {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: pos, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	now := time.Now()
	card := func(user, lexeme int64, state int16, lapses int32, due time.Time) {
		_, err := q.InsertCard(ctx, db.InsertCardParams{
			UserID: user, LexemeID: lexeme, Due: pgtype.Timestamptz{Time: due, Valid: true},
			State: state, Lapses: lapses, Stability: 1, Difficulty: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	card(me.ID, lex("go", "verb"), 3, 0, now)                      // state 3
	card(me.ID, lex("run", "verb"), 2, 2, now)                     // lapses 2
	card(me.ID, lex("big", "adjective"), 2, 5, now.Add(time.Hour)) // lapses 5, первым
	card(me.ID, lex("walk", "verb"), 0, 0, now)                    // новая, нет
	card(me.ID, lex("dog", "noun"), 2, 3, now)                     // существительное, нет
	card(me.ID, lex("quickly", "adverb"), 2, 3, now)               // наречие, нет
	card(me.ID, lex("ice cream", "verb"), 2, 3, now)               // не однословная, нет
	card(me.ID, lex("Paris", "verb"), 2, 3, now)                   // не строчная, нет
	card(other.ID, lex("eat", "verb"), 2, 9, now)                  // чужая, нет

	rows, err := q.ListWeakCardWords(ctx, db.ListWeakCardWordsParams{UserID: me.ID, MaxRows: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Lemma+"/"+r.Pos)
	}
	// big (5 лапсов), run (2), go (0 лапсов, state 3) — по убыванию лапсов.
	if want := []string{"big/adjective", "run/verb", "go/verb"}; !slices.Equal(got, want) {
		t.Fatalf("слабые слова %v, ожидали %v", got, want)
	}
	if rows[0].LexemeID == 0 {
		t.Fatal("LexemeID не заполнен")
	}
	rows, err = q.ListWeakCardWords(ctx, db.ListWeakCardWordsParams{UserID: me.ID, MaxRows: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("лимит 1: %+v (%v)", rows, err)
	}
}
