package testdb

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestListActivityEvents(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	me, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	// В схеме один пользователь; для проверки «чужих» событий снимаем ограничение во временной схеме.
	if _, err := pool.Exec(ctx, `DROP INDEX users_single_row`); err != nil {
		t.Fatal(err)
	}
	other, err := q.CreateUser(ctx, db.CreateUserParams{Login: "other", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	add := func(uid int64, typ string, at time.Time, payload string) {
		t.Helper()
		_, err := q.AppendEvent(ctx, db.AppendEventParams{
			UserID: uid, Type: typ, SchemaVersion: 1, OccurredAt: pgtype.Timestamptz{Time: at, Valid: true}, Payload: []byte(payload),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(me.ID, "exercise.answered", base.Add(2*time.Minute), `{"correct":true}`)
	add(me.ID, "exercise.answered", base, `{"correct":false}`)
	add(me.ID, "card.reviewed", base.Add(time.Minute), `{"rating":3}`)
	add(me.ID, "mistake.opened", base.Add(3*time.Minute), `{}`)
	add(me.ID, "mistake.closed", base.Add(4*time.Minute), `{}`)
	add(me.ID, "card.added", base, `{}`)                                         // не учитывается
	add(me.ID, "exercise.answered", base.Add(-48*time.Hour), `{"correct":true}`) // раньше since
	add(other.ID, "exercise.answered", base, `{"correct":true}`)                 // чужое

	rows, err := q.ListActivityEvents(ctx, db.ListActivityEventsParams{UserID: me.ID, Since: pgtype.Timestamptz{Time: base.Add(-time.Hour), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Type)
	}
	want := []string{"exercise.answered", "card.reviewed", "exercise.answered", "mistake.opened", "mistake.closed"}
	if len(got) != len(want) {
		t.Fatalf("события %v, ожидали %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("порядок или состав: %v, ожидали %v", got, want)
		}
	}
	// correct берётся из payload; без булева поля — false
	if rows[0].Correct {
		t.Errorf("первое событие: correct=false")
	}
	if rows[1].Correct {
		t.Errorf("card.reviewed без поля correct должно давать false")
	}
	if !rows[2].Correct {
		t.Errorf("третье событие: correct=true")
	}
}
