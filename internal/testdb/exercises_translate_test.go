package testdb

import (
	"context"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Миграция 00013: вид translate проходит проверку kind; откат падает, пока такие упражнения есть.
func TestMigration00013TranslateKind(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	q := db.New(pool)

	u, err := q.CreateUser(ctx, db.CreateUserParams{Login: "vlad", PasswordHash: "$argon2id$x"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "translate", Topics: []string{"skill:translate"}, Payload: []byte(`{}`), Answer: []byte(`{}`)})
	if err != nil {
		t.Fatalf("упражнение translate: %v", err)
	}
	if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "typo", Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
		t.Fatal("неизвестный вид должен отклоняться")
	}

	down := downSQL(t, "00013_exercises_translate.sql")
	_, err = pool.Exec(ctx, down)
	if err == nil || !strings.Contains(err.Error(), "откат 00013 невозможен") {
		t.Fatalf("откат с упражнением translate должен падать с понятным текстом, получили %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM exercises WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("откат без упражнений translate: %v", err)
	}
	if _, err := q.InsertExercise(ctx, db.InsertExerciseParams{UserID: u.ID, Kind: "translate", Topics: []string{"a"}, Payload: []byte(`{}`), Answer: []byte(`{}`)}); err == nil {
		t.Fatal("после отката вид translate запрещён")
	}
}

func downSQL(t *testing.T, name string) string {
	t.Helper()
	for _, m := range readMigrations(t) {
		if m.name == name {
			return m.down
		}
	}
	t.Fatalf("миграция %s не найдена", name)
	return ""
}
