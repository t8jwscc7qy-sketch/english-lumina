package testdb

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestMistakesMigrationUpDownUp(t *testing.T) {
	pool := Pool(t)
	columnExists := func() bool {
		var ok bool
		if err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'exercises' AND column_name = 'mistake_id')`).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !tableExists(t, pool, "mistakes") || !columnExists() {
		t.Fatal("после Up нет таблицы mistakes или exercises.mistake_id")
	}
	Down(t, pool)
	if tableExists(t, pool, "mistakes") || tableExists(t, pool, "exercises") {
		t.Fatal("после Down остались таблицы")
	}
	Up(t, pool)
	if !tableExists(t, pool, "mistakes") || !columnExists() {
		t.Fatal("после повторного Up нет таблицы mistakes или exercises.mistake_id")
	}
}

func TestMistakesQueries(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)
	me, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	// В схеме разрешён один пользователь; для проверки «чужих» ошибок снимаем ограничение во временной схеме.
	if _, err := pool.Exec(ctx, `DROP INDEX users_single_row`); err != nil {
		t.Fatal(err)
	}
	other, err := q.CreateUser(ctx, db.CreateUserParams{Login: "other", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	sent := func(n int64) int64 {
		s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: n, Text: "I went home.", Author: "a", WordCount: 3, SourceID: src.ID})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	s1, s2, s3, s4 := sent(1), sent(2), sent(3), sent(4)

	base := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	ts := func(d time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: d, Valid: true} }
	add := func(user, sentence int64, kind string, due time.Time) int64 {
		id, err := q.InsertMistake(ctx, db.InsertMistakeParams{
			UserID: user, Kind: kind, SentenceID: sentence, Topics: []string{"grammar:articles"},
			Payload: []byte(`{"p":1}`), Answer: []byte(`{"a":["a"]}`), Given: "the",
			NextDue: ts(due), FirstWrongAt: ts(base),
		})
		if err != nil {
			t.Fatalf("InsertMistake: %v", err)
		}
		return id
	}

	t.Run("уникальность открытой ошибки", func(t *testing.T) {
		first := add(me.ID, s1, "gap", base.Add(24*time.Hour))
		_, err := q.InsertMistake(ctx, db.InsertMistakeParams{
			UserID: me.ID, Kind: "gap", SentenceID: s1, Topics: []string{"x"}, Payload: []byte(`{}`), Answer: []byte(`{}`),
			Given: "g", NextDue: ts(base), FirstWrongAt: ts(base),
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Fatalf("вторая открытая на (user, sentence, kind): ожидали 23505, получили %v", err)
		}
		// другой вид и чужой пользователь допустимы
		add(me.ID, s1, "order", base)
		add(other.ID, s1, "gap", base)

		got, err := q.GetOpenMistakeForUpdate(ctx, db.GetOpenMistakeForUpdateParams{UserID: me.ID, SentenceID: s1, Kind: "gap"})
		if err != nil || got.ID != first || got.Streak != 0 || got.Status != "open" || got.ClosedAt.Valid {
			t.Fatalf("GetOpenMistakeForUpdate: %+v (%v)", got, err)
		}
		closedAt := base.Add(time.Hour)
		if err := q.UpdateMistakeProgress(ctx, db.UpdateMistakeProgressParams{
			ID: first, Streak: 2, Status: "closed", NextDue: ts(base), ClosedAt: ts(closedAt), Given: "the",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := q.GetOpenMistakeForUpdate(ctx, db.GetOpenMistakeForUpdateParams{UserID: me.ID, SentenceID: s1, Kind: "gap"}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("закрытая не должна находиться как открытая: %v", err)
		}
		second := add(me.ID, s1, "gap", base) // после закрытия первой допускается
		if second == first {
			t.Fatal("ожидали новую строку")
		}
		cl, err := q.GetMistakeForUpdate(ctx, db.GetMistakeForUpdateParams{ID: first, UserID: me.ID})
		if err != nil || cl.Status != "closed" || !cl.ClosedAt.Valid || cl.Streak != 2 {
			t.Fatalf("GetMistakeForUpdate: %+v (%v)", cl, err)
		}
		if _, err := q.GetMistakeForUpdate(ctx, db.GetMistakeForUpdateParams{ID: first, UserID: other.ID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("чужая ошибка: %v", err)
		}
		// сброс открытой: серия 0, новый срок, новый неверный ответ
		if err := q.UpdateMistakeProgress(ctx, db.UpdateMistakeProgressParams{ID: second, Streak: 1, Status: "open", NextDue: ts(base), Given: "the"}); err != nil {
			t.Fatal(err)
		}
		if err := q.ResetMistake(ctx, db.ResetMistakeParams{ID: second, Given: "an", NextDue: ts(base.Add(48 * time.Hour))}); err != nil {
			t.Fatal(err)
		}
		r, _ := q.GetMistakeForUpdate(ctx, db.GetMistakeForUpdateParams{ID: second, UserID: me.ID})
		if r.Streak != 0 || r.Given != "an" || !r.NextDue.Time.Equal(base.Add(48*time.Hour)) || r.Status != "open" {
			t.Fatalf("ResetMistake: %+v", r)
		}
	})

	t.Run("NextDueMistake и ListOpenMistakes", func(t *testing.T) {
		now := base.Add(10 * 24 * time.Hour)
		old := add(me.ID, s2, "gap", base.Add(-24*time.Hour)) // старше всех строк предыдущего подтеста
		recent := add(me.ID, s3, "gap", now.Add(-time.Hour))
		future := add(me.ID, s4, "gap", now.Add(time.Hour))
		closed := add(me.ID, s4, "form", now.Add(-72*time.Hour))
		ca := now
		if err := q.UpdateMistakeProgress(ctx, db.UpdateMistakeProgressParams{ID: closed, Streak: 2, Status: "closed", NextDue: ts(now.Add(-72 * time.Hour)), ClosedAt: ts(ca), Given: "x"}); err != nil {
			t.Fatal(err)
		}
		// чужая, самая старая
		add(other.ID, s2, "gap", now.Add(-100*time.Hour))

		m, err := q.NextDueMistake(ctx, db.NextDueMistakeParams{UserID: me.ID, Now: ts(now)})
		if err != nil || m.ID != old {
			t.Fatalf("NextDueMistake: %+v (%v), ожидали %d", m, err, old)
		}
		if _, err := q.NextDueMistake(ctx, db.NextDueMistakeParams{UserID: me.ID, Now: ts(now.Add(-1000 * time.Hour))}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("до наступления срока ошибок нет: %v", err)
		}
		rows, err := q.ListOpenMistakes(ctx, db.ListOpenMistakesParams{UserID: me.ID, MaxRows: 100})
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, r := range rows {
			if r.UserID != me.ID || r.Status != "open" {
				t.Fatalf("лишняя строка %+v", r)
			}
			ids = append(ids, r.ID)
		}
		// Порядок по сроку: old раньше recent раньше future.
		pos := func(id int64) int { return slices.Index(ids, id) }
		if pos(old) < 0 || pos(old) >= pos(recent) || pos(recent) >= pos(future) {
			t.Fatalf("порядок %v: old=%d recent=%d future=%d", ids, old, recent, future)
		}
		if slices.Contains(ids, closed) {
			t.Fatal("закрытая в списке открытых")
		}
		lim, err := q.ListOpenMistakes(ctx, db.ListOpenMistakesParams{UserID: me.ID, MaxRows: 1})
		if err != nil || len(lim) != 1 {
			t.Fatalf("лимит 1: %d (%v)", len(lim), err)
		}

		c, err := q.CountMistakes(ctx, db.CountMistakesParams{UserID: me.ID, WeekAgo: ts(now.Add(-7 * 24 * time.Hour)), Now: ts(now)})
		if err != nil {
			t.Fatal(err)
		}
		// открытые у me: s1/gap (second), s1/order, s2, s3, s4/gap = 5; наступили (<= now): все кроме future = 4;
		// закрытые за неделю: closed (ca = now) и первая из прошлого теста (base+1ч, это 10 дней назад) — только closed.
		if c.Open != 5 || c.DueNow != 4 || c.ClosedThisWeek != 1 {
			t.Fatalf("CountMistakes: %+v", c)
		}
	})

	t.Run("повторы в exercises", func(t *testing.T) {
		now := base.Add(20 * 24 * time.Hour)
		m1 := add(me.ID, sent(10), "gap", now)
		m2 := add(me.ID, sent(11), "gap", now)
		ex := func(user int64, mistake *int64, answered *time.Time) int64 {
			id, err := q.InsertExercise(ctx, db.InsertExerciseParams{
				UserID: user, Kind: "gap", SentenceID: s1, Topics: []string{"grammar:articles"},
				Payload: []byte(`{}`), Answer: []byte(`{}`), MistakeID: mistake,
			})
			if err != nil {
				t.Fatal(err)
			}
			if answered != nil {
				if err := q.MarkExerciseAnswered(ctx, db.MarkExerciseAnsweredParams{ID: id, AnsweredAt: ts(*answered)}); err != nil {
					t.Fatal(err)
				}
			}
			return id
		}
		if _, err := q.GetUnansweredRepeat(ctx, db.GetUnansweredRepeatParams{UserID: me.ID, MistakeID: &m1}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("пока копии нет: %v", err)
		}
		t1, t2, t3 := now.Add(time.Hour), now.Add(2*time.Hour), now.Add(3*time.Hour)
		ex(me.ID, nil, &t1)    // обычное, отвечено
		ex(me.ID, &m1, &t2)    // повтор, отвечен
		ex(me.ID, &m2, &t3)    // повтор, отвечен (последний)
		ex(other.ID, &m1, &t3) // чужой, не считается
		open := ex(me.ID, &m1, nil)
		ex(me.ID, nil, nil) // обычное неотвеченное

		n, err := q.CountRepeatsToday(ctx, db.CountRepeatsTodayParams{UserID: me.ID, Since: ts(now)})
		if err != nil || n != 2 {
			t.Fatalf("CountRepeatsToday = %d (%v), ожидали 2", n, err)
		}
		n, _ = q.CountRepeatsToday(ctx, db.CountRepeatsTodayParams{UserID: me.ID, Since: ts(t3)})
		if n != 1 {
			t.Fatalf("CountRepeatsToday с t3 = %d, ожидали 1", n)
		}
		flags, err := q.LastTwoExerciseRepeatFlags(ctx, me.ID)
		if err != nil || !slices.Equal(flags, []bool{true, true}) {
			t.Fatalf("флаги %v (%v), ожидали [true true]", flags, err)
		}
		got, err := q.GetUnansweredRepeat(ctx, db.GetUnansweredRepeatParams{UserID: me.ID, MistakeID: &m1})
		if err != nil || got.ID != open || got.MistakeID == nil || *got.MistakeID != m1 {
			t.Fatalf("GetUnansweredRepeat: %+v (%v)", got, err)
		}

		// Удаление ошибки обнуляет ссылку (ON DELETE SET NULL).
		if _, err := pool.Exec(ctx, `DELETE FROM mistakes WHERE id = $1`, m2); err != nil {
			t.Fatal(err)
		}
		n, _ = q.CountRepeatsToday(ctx, db.CountRepeatsTodayParams{UserID: me.ID, Since: ts(now)})
		if n != 1 {
			t.Fatalf("после удаления ошибки CountRepeatsToday = %d, ожидали 1", n)
		}
	})
}
