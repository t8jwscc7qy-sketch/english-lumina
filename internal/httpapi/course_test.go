package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

const (
	courseAPIPath   = "/api/v1/course"
	lessonPath    = "/api/v1/course/lesson"
	completePath  = "/api/v1/course/complete"
	completeBody1 = `{"lessonId":"u1-l1","key":"k1","seconds":420,"mistakes":2}`
)

// ListCompletedLessons повторяет SQL: номера уроков из событий lesson.completed без повторов.
func (f *fakeStore) ListCompletedLessons(_ context.Context, userID int64) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, ev := range f.events {
		if ev.UserID != userID || ev.Type != "lesson.completed" {
			continue
		}
		var p struct {
			LessonID string `json:"lessonId"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil && p.LessonID != "" && !seen[p.LessonID] {
			seen[p.LessonID] = true
			out = append(out, p.LessonID)
		}
	}
	return out, nil
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("ответ нечитаем: %v (%s)", err, body)
	}
	return v
}

func statesOf(p api.CoursePath) []string {
	var out []string
	for _, u := range p.Units {
		for _, l := range u.Lessons {
			out = append(out, l.Id+"="+string(l.State))
		}
	}
	return out
}

func TestCoursePathAndLessonAccess(t *testing.T) {
	e, token := learnEnv(t)
	rec := e.do(http.MethodGet, courseAPIPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("путь: %d %s", rec.Code, rec.Body)
	}
	p := decode[api.CoursePath](t, rec.Body.Bytes())
	got := statesOf(p)
	if len(got) < 3 || got[0] != "u1-l1=current" || got[1] != "u1-l2=locked" || p.Next == nil || p.Next.Id != "u1-l1" {
		t.Fatalf("в начале открыт только первый урок: %v", got)
	}

	rec = e.do(http.MethodPost, lessonPath, `{}`, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("текущий урок: %d %s", rec.Code, rec.Body)
	}
	l := decode[api.Lesson](t, rec.Body.Bytes())
	if l.Id != "u1-l1" || l.Done || len(l.Words) == 0 || len(l.Drills) < 4 || l.Glossary["hello"] == "" || l.Words[0].Example == nil {
		t.Fatalf("урок собран неполно: %+v", l)
	}
	if rec := e.do(http.MethodPost, lessonPath, `{"id":"u1-l2"}`, withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("закрытый урок: %d", rec.Code)
	}
	if rec := e.do(http.MethodPost, lessonPath, `{"id":"u9-l9"}`, withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("неизвестный урок: %d", rec.Code)
	}
	if rec := e.do(http.MethodPost, lessonPath, `{"id":"u1-l1","x":1}`, withCookie(token)); rec.Code != http.StatusBadRequest {
		t.Fatalf("лишнее поле: %d", rec.Code)
	}
	if rec := e.do(http.MethodGet, courseAPIPath, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", rec.Code)
	}
}

func TestCompleteLesson(t *testing.T) {
	e, token := learnEnv(t)
	e.store.addLexeme(901, "hello", "noun", "A1")
	e.store.addLexeme(902, "I", "pronoun", "A1")
	e.store.addLexeme(903, "am", "be-verb", "A1") // you/are нет в словаре: урок всё равно проходится

	rec := e.do(http.MethodPost, completePath, completeBody1, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("завершение: %d %s", rec.Code, rec.Body)
	}
	res := decode[api.CompleteLessonResponse](t, rec.Body.Bytes())
	if res.CardsAdded != 3 || res.NewWords != 5 || res.Repeated || res.Seconds != 420 || res.Next == nil || res.Next.Id != "u1-l2" {
		t.Fatalf("итог: %+v", res)
	}
	for _, id := range []int64{901, 902, 903} {
		if !e.store.hasCard(1, id) || e.store.wl[id].status != "introduced" {
			t.Errorf("слово %d: нет карточки или состояния «введено»", id)
		}
	}
	events := len(e.store.events)

	// Повтор по ключу: тот же ответ, ничего не добавлено.
	rec = e.do(http.MethodPost, completePath, completeBody1, withCookie(token))
	if again := decode[api.CompleteLessonResponse](t, rec.Body.Bytes()); rec.Code != http.StatusOK || again.CardsAdded != 3 || len(e.store.events) != events {
		t.Fatalf("повтор по ключу: %d %+v, событий %d → %d", rec.Code, again, events, len(e.store.events))
	}

	// Повторное прохождение с новым ключом: карточек не добавляет.
	rec = e.do(http.MethodPost, completePath, `{"lessonId":"u1-l1","key":"k2","seconds":99999}`, withCookie(token))
	if again := decode[api.CompleteLessonResponse](t, rec.Body.Bytes()); rec.Code != http.StatusOK || !again.Repeated || again.CardsAdded != 0 || again.Seconds != maxLessonSeconds {
		t.Fatalf("повторное прохождение: %d %+v", rec.Code, again)
	}

	p := decode[api.CoursePath](t, e.do(http.MethodGet, courseAPIPath, "", withCookie(token)).Body.Bytes())
	if got := statesOf(p); got[0] != "u1-l1=done" || got[1] != "u1-l2=current" || got[2] != "u1-l3=locked" {
		t.Fatalf("после первого урока: %v", got)
	}
	if rec := e.do(http.MethodPost, completePath, `{"lessonId":"u1-l3","key":"k3","seconds":1}`, withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("закрытый урок нельзя завершить: %d", rec.Code)
	}
	for name, body := range map[string]string{
		"без ключа":           `{"lessonId":"u1-l2","key":"","seconds":1}`,
		"отрицательное время": `{"lessonId":"u1-l2","key":"k4","seconds":-1}`,
		"неизвестный урок":    `{"lessonId":"nope","key":"k5","seconds":1}`,
	} {
		if rec := e.do(http.MethodPost, completePath, body, withCookie(token)); rec.Code == http.StatusOK {
			t.Errorf("%s: принят", name)
		}
	}
	l := decode[api.Lesson](t, e.do(http.MethodPost, lessonPath, `{"id":"u1-l1"}`, withCookie(token)).Body.Bytes())
	if !l.Done {
		t.Fatal("пройденный урок открывается с пометкой done")
	}
}
