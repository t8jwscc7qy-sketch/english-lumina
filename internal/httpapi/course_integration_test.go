package httpapi

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

// Две вкладки одновременно завершают один урок с разными ключами: одно «первое прохождение», карточки без дублей.
func TestCompleteLessonOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range [][2]string{{"hello", "noun"}, {"I", "pronoun"}, {"am", "be-verb"}} {
		if _, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w[0], Pos: w[1], SourceID: ds.ID}); err != nil {
			t.Fatal(err)
		}
	}
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)

	var wg sync.WaitGroup
	results := make([]api.CompleteLessonResponse, 2)
	codes := make([]int, 2)
	for i, key := range []string{"tab-a", "tab-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := e.do(http.MethodPost, completePath, `{"lessonId":"u1-l1","key":"`+key+`","seconds":300}`, withCookie(token))
			codes[i] = rec.Code
			results[i] = decode[api.CompleteLessonResponse](t, rec.Body.Bytes())
		}()
	}
	wg.Wait()
	first := 0
	cards := 0
	for i, r := range results {
		if codes[i] != http.StatusOK {
			t.Fatalf("вкладка %d: %d", i, codes[i])
		}
		if !r.Repeated {
			first++
		}
		cards += r.CardsAdded
	}
	if first != 1 || cards != 3 {
		t.Fatalf("первое прохождение должно быть одно, карточек 3: %+v", results)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cards`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("карточек в базе %d (%v)", n, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM word_learning WHERE status = 'introduced'`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("слов «введено» %d (%v)", n, err)
	}
	done, err := q.ListCompletedLessons(ctx, 1)
	if err != nil || len(done) != 1 || done[0] != "u1-l1" {
		t.Fatalf("пройденные уроки: %v (%v)", done, err)
	}
	rows, err := q.ListActivityEvents(ctx, db.ListActivityEventsParams{UserID: 1, Since: ts(time.Now().Add(-time.Hour))})
	if err != nil || len(rows) != 2 || rows[0].Type != "lesson.completed" || rows[0].Seconds != 300 {
		t.Fatalf("события урока для минут дня: %+v (%v)", rows, err)
	}
	today := decode[api.ProgressToday](t, e.do(http.MethodGet, "/api/v1/progress/today", "", withCookie(token)).Body.Bytes())
	if today.Minutes != 10 || !today.DoneToday {
		t.Fatalf("два урока по 5 минут дают минимум дня: %+v", today)
	}
	p := decode[api.CoursePath](t, e.do(http.MethodGet, courseAPIPath, "", withCookie(token)).Body.Bytes())
	if p.Next == nil || p.Next.Id != "u1-l2" {
		t.Fatalf("следующий урок: %+v", p.Next)
	}
}
