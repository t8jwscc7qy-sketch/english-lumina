package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

var errBoom = errors.New("бум")

func newTxFixture(t *testing.T) (*PgStore, *pgxpool.Pool, int64) {
	t.Helper()
	pool := testdb.Pool(t)
	u, err := db.New(pool).CreateUser(context.Background(), db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	return NewPgStore(pool), pool, u.ID
}

func appendEvent(t *testing.T, s AuthStore, userID int64, key string) error {
	t.Helper()
	_, err := s.AppendEvent(context.Background(), db.AppendEventParams{
		UserID: userID, Type: "test.event", SchemaVersion: 1, Payload: json.RawMessage(`{}`),
		OccurredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, IdempotencyKey: &key,
	})
	return err
}

func eventCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInTxCommitsOnSuccess(t *testing.T) {
	s, pool, uid := newTxFixture(t)
	if err := s.InTx(context.Background(), func(tx AuthStore) error { return appendEvent(t, tx, uid, "a") }); err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, pool); n != 1 {
		t.Fatalf("событий %d, ожидали 1", n)
	}
	if got := pool.Stat().AcquiredConns(); got != 0 {
		t.Fatalf("соединений не возвращено в пул: %d", got)
	}
}

func TestInTxRollsBackWhenFunctionFailsAfterWriting(t *testing.T) {
	s, pool, uid := newTxFixture(t)
	err := s.InTx(context.Background(), func(tx AuthStore) error {
		if err := appendEvent(t, tx, uid, "a"); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ошибка %v, ожидали errBoom", err)
	}
	if n := eventCount(t, pool); n != 0 {
		t.Fatalf("после ошибки в базе %d событий, ожидали 0", n)
	}
	if got := pool.Stat().AcquiredConns(); got != 0 {
		t.Fatalf("соединений не возвращено в пул: %d", got)
	}
}

func TestInTxRollsBackOnPanicAndDoesNotSwallowIt(t *testing.T) {
	s, pool, uid := newTxFixture(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("паника должна дойти до вызывающего")
			}
		}()
		_ = s.InTx(context.Background(), func(tx AuthStore) error {
			_ = appendEvent(t, tx, uid, "a")
			panic("ой")
		})
	}()
	if n := eventCount(t, pool); n != 0 {
		t.Fatalf("после паники в базе %d событий, ожидали 0", n)
	}
	if got := pool.Stat().AcquiredConns(); got != 0 {
		t.Fatalf("соединений не возвращено в пул: %d", got)
	}
}

func TestNestedInTxUsesTheSameTransaction(t *testing.T) {
	s, pool, uid := newTxFixture(t)
	ctx := context.Background()
	err := s.InTx(ctx, func(outer AuthStore) error {
		if err := appendEvent(t, outer, uid, "outer"); err != nil {
			return err
		}
		return outer.InTx(ctx, func(inner AuthStore) error { return appendEvent(t, inner, uid, "inner") })
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, pool); n != 2 {
		t.Fatalf("событий %d, ожидали 2", n)
	}

	// Ошибка во вложенном вызове откатывает и внешний.
	err = s.InTx(ctx, func(outer AuthStore) error {
		if err := appendEvent(t, outer, uid, "outer2"); err != nil {
			return err
		}
		return outer.InTx(ctx, func(AuthStore) error { return errBoom })
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ошибка %v", err)
	}
	if n := eventCount(t, pool); n != 2 {
		t.Fatalf("после неудачной вложенной транзакции событий %d, ожидали по-прежнему 2", n)
	}
}

func TestInTxSeesItsOwnWritesAndHidesThemUntilCommit(t *testing.T) {
	s, pool, uid := newTxFixture(t)
	ctx := context.Background()
	err := s.InTx(ctx, func(tx AuthStore) error {
		if err := appendEvent(t, tx, uid, "a"); err != nil {
			return err
		}
		// Повтор того же ключа внутри транзакции видит собственную запись: события не добавляется (ErrNoRows).
		if err := appendEvent(t, tx, uid, "a"); err == nil {
			t.Error("повтор ключа внутри транзакции должен давать ErrNoRows")
		}
		// Снаружи записи ещё не видно.
		if n := eventCount(t, pool); n != 0 {
			t.Errorf("до фиксации снаружи видно %d событий", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
