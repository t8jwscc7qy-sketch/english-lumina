package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/course"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
)

const (
	maxLessonSeconds = 3600
	maxLessonID      = 32
)

// courseData — встроенный курс, проверенный при первом обращении. Битый курс ловит тест содержимого
// (`go test ./content`); в рабочем сервере ошибка пишется в журнал, а путь остаётся пустым.
var courseData = sync.OnceValue(func() course.Course {
	c, err := course.Load(content.Course)
	if err != nil {
		slog.Error("курс не загружен", "err", err)
		return course.Course{}
	}
	return c
})

// loadCourse подменяется в тестах обработчиков.
var loadCourse = courseData

func (s *authServer) failCourse(w http.ResponseWriter, err error) {
	slog.Error("курс", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// completedLessons — номера пройденных уроков пользователя.
func (s *authServer) completedLessons(ctx context.Context, userID int64) (map[string]bool, error) {
	ids, err := s.store.ListCompletedLessons(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("пройденные уроки: %w", err)
	}
	done := make(map[string]bool, len(ids))
	for _, id := range ids {
		done[id] = true
	}
	return done, nil
}

// coursePath раскладывает уроки по состояниям: пройден, текущий (первый непройденный), закрыт.
func coursePath(c course.Course, done map[string]bool) api.CoursePath {
	out := api.CoursePath{Units: []api.CourseUnit{}}
	foundCurrent := false
	for _, u := range c.Units {
		unit := api.CourseUnit{Unit: u.ID, Title: u.Title, Lessons: []api.CourseLessonRef{}}
		for i, l := range u.Lessons {
			ref := api.CourseLessonRef{Id: l.ID, Title: l.Title, Number: i + 1, Unit: u.ID, NewWords: len(l.Words), State: api.CourseLessonRefStateLocked}
			switch {
			case done[l.ID]:
				ref.State = api.CourseLessonRefStateDone
			case !foundCurrent:
				ref.State = api.CourseLessonRefStateCurrent
				foundCurrent = true
				next := ref
				out.Next = &next
			}
			unit.Lessons = append(unit.Lessons, ref)
		}
		out.Units = append(out.Units, unit)
	}
	return out
}

// lessonState — состояние урока по номеру; ok = false, если такого урока нет.
func lessonState(path api.CoursePath, id string) (api.CourseLessonRefState, bool) {
	for _, u := range path.Units {
		for _, l := range u.Lessons {
			if l.Id == id {
				return l.State, true
			}
		}
	}
	return "", false
}

// GetCourse отдаёт путь: блоки и уроки с состоянием.
func (s *authServer) GetCourse(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	done, err := s.completedLessons(r.Context(), user.ID)
	if err != nil {
		s.failCourse(w, err)
		return
	}
	writeJSON(w, http.StatusOK, coursePath(loadCourse(), done))
}

// GetLesson отдаёт урок целиком: без id — текущий; пройденный можно открыть снова, закрытый — нет.
func (s *authServer) GetLesson(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.LessonRef
	if !decodeLearnBody(w, r, &req) {
		return
	}
	id := ""
	if req.Id != nil {
		id = strings.TrimSpace(*req.Id)
	}
	if len(id) > maxLessonID {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный номер урока")
		return
	}
	done, err := s.completedLessons(r.Context(), user.ID)
	if err != nil {
		s.failCourse(w, err)
		return
	}
	c := loadCourse()
	path := coursePath(c, done)
	if id == "" {
		if path.Next == nil {
			writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Все уроки пройдены. Новые скоро появятся")
			return
		}
		id = path.Next.Id
	}
	state, ok := lessonState(path, id)
	switch {
	case !ok:
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такого урока нет")
		return
	case state == api.CourseLessonRefStateLocked:
		writeError(w, http.StatusForbidden, api.ErrorErrorCodeLessonLocked, "Этот урок откроется, когда пройдёшь предыдущие")
		return
	}
	l, u, index := c.Find(id)
	writeJSON(w, http.StatusOK, lessonResponse(c, l, u, index, state == api.CourseLessonRefStateDone))
}

// lessonResponse собирает урок для экрана: слова с примером из текста, задания, словарик подсказок.
func lessonResponse(c course.Course, l course.Lesson, u course.Unit, index int, done bool) api.Lesson {
	number := 1
	for i, x := range u.Lessons {
		if x.ID == l.ID {
			number = i + 1
		}
	}
	out := api.Lesson{
		Id: l.ID, Title: l.Title, Number: number, Unit: u.ID, UnitTitle: u.Title, Done: done,
		Words: []api.LessonWord{}, Sentences: []api.LessonSentence{}, Questions: []api.LessonQuestion{},
		Drills: []api.LessonDrill{}, Glossary: course.Glossary(c, index),
	}
	for _, s := range l.Sentences {
		out.Sentences = append(out.Sentences, apiSentence(s))
	}
	for _, w := range l.Words {
		lw := api.LessonWord{Lemma: w.Lemma, Pos: w.Pos, Ru: w.Ru, Forms: append([]string{}, w.Forms...)}
		if s, ok := exampleFor(l, w); ok {
			ex := apiSentence(s)
			lw.Example = &ex
		}
		out.Words = append(out.Words, lw)
	}
	for _, q := range l.Questions {
		out.Questions = append(out.Questions, api.LessonQuestion{Prompt: q.Prompt, Options: q.Options, Answer: q.Answer})
	}
	for _, d := range course.Drills(c, index) {
		ad := api.LessonDrill{Kind: d.Kind, Old: d.Old}
		if d.Prompt != "" {
			ad.Prompt = &d.Prompt
		}
		if d.Text != "" {
			ad.Text = &d.Text
		}
		if d.Answer != "" {
			ad.Answer = &d.Answer
		}
		if len(d.Options) > 0 {
			ad.Options = &d.Options
		}
		if len(d.Tiles) > 0 {
			ad.Tiles = &d.Tiles
		}
		if len(d.Pairs) > 0 {
			pairs := make([]api.LessonPair, len(d.Pairs))
			for i, p := range d.Pairs {
				pairs[i] = api.LessonPair{En: p.En, Ru: p.Ru}
			}
			ad.Pairs = &pairs
		}
		out.Drills = append(out.Drills, ad)
	}
	if l.Note != nil {
		out.Note = &api.LessonNote{Title: l.Note.Title, Text: l.Note.Text}
	}
	return out
}

func apiSentence(s course.Sentence) api.LessonSentence {
	out := api.LessonSentence{En: s.En, Ru: s.Ru}
	if s.Speaker != "" {
		sp := s.Speaker
		out.Speaker = &sp
	}
	return out
}

// exampleFor — первая строка текста, где встречается слово (лемма или его форма).
func exampleFor(l course.Lesson, w course.Word) (course.Sentence, bool) {
	forms := map[string]bool{strings.ToLower(w.Lemma): true}
	for _, f := range w.Forms {
		forms[strings.ToLower(f)] = true
	}
	for _, s := range l.Sentences {
		for _, t := range s.Tokens() {
			if forms[strings.ToLower(strings.ReplaceAll(t, "’", "'"))] {
				return s, true
			}
		}
	}
	return course.Sentence{}, false
}

// CompleteLesson отмечает урок пройденным: новые слова получают карточки повторения и состояние «введено»,
// пишется событие lesson.completed. Повтор по ключу отдаёт прежний ответ.
func (s *authServer) CompleteLesson(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.CompleteLessonRequest
	if !decodeLearnBody(w, r, &req) {
		return
	}
	if !validKey(req.Key) || req.LessonId == "" || len(req.LessonId) > maxLessonID || req.Seconds < 0 || (req.Mistakes != nil && *req.Mistakes < 0) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны lessonId, key и seconds ≥ 0")
		return
	}
	ctx := r.Context()
	c := loadCourse()
	eventKey := "course.complete:" + req.Key
	var prev api.CompleteLessonResponse
	if ok, err := s.storedResult(ctx, user.ID, eventKey, &prev); err != nil {
		s.failCourse(w, err)
		return
	} else if ok {
		s.writeCompleted(w, r, user.ID, c, prev)
		return
	}
	done, err := s.completedLessons(ctx, user.ID)
	if err != nil {
		s.failCourse(w, err)
		return
	}
	state, ok := lessonState(coursePath(c, done), req.LessonId)
	switch {
	case !ok:
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такого урока нет")
		return
	case state == api.CourseLessonRefStateLocked:
		writeError(w, http.StatusForbidden, api.ErrorErrorCodeLessonLocked, "Этот урок откроется, когда пройдёшь предыдущие")
		return
	}
	l, _, _ := c.Find(req.LessonId)
	resp := api.CompleteLessonResponse{LessonId: l.ID, NewWords: len(l.Words), Seconds: min(req.Seconds, maxLessonSeconds), Repeated: state == api.CourseLessonRefStateDone}
	mistakes := 0
	if req.Mistakes != nil {
		mistakes = *req.Mistakes
	}
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		now := s.now()
		resp.CardsAdded = 0
		var cardIDs []int64
		if !resp.Repeated {
			for _, word := range l.Words {
				lx, err := tx.GetLexemeByPair(ctx, db.GetLexemeByPairParams{Lemma: word.Lemma, Pos: word.Pos})
				if errors.Is(err, pgx.ErrNoRows) {
					slog.Warn("слова урока нет в словаре: карточка не создана", "lesson", l.ID, "lemma", word.Lemma, "pos", word.Pos)
					continue
				}
				if err != nil {
					return err
				}
				if _, err := tx.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: user.ID, LexemeID: lx.ID}); err == nil {
					continue // карточка уже есть: слово знакомо по другим разделам
				} else if !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				if _, err := tx.LearnInsertIntroduced(ctx, db.LearnInsertIntroducedParams{UserID: user.ID, LexemeID: lx.ID, IntroducedAt: ts(now)}); err != nil {
					return err
				}
				id, err := tx.InsertCard(ctx, insertParams(user.ID, lx.ID, srs.New(now)))
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				if err != nil {
					return err
				}
				cardIDs = append(cardIDs, id)
				resp.CardsAdded++
			}
		}
		payload, err := json.Marshal(map[string]any{
			"lessonId": resp.LessonId, "newWords": resp.NewWords, "cardsAdded": resp.CardsAdded, "seconds": resp.Seconds,
			"repeated": resp.Repeated, "mistakes": mistakes, "cardIds": cardIDs,
		})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: user.ID, Type: "lesson.completed", SchemaVersion: 1, OccurredAt: ts(now), Payload: payload, IdempotencyKey: &eventKey})
		if errors.Is(err, pgx.ErrNoRows) {
			return errReplay
		}
		return err
	})
	switch {
	case errors.Is(err, errReplay):
		if ok, err := s.storedResult(ctx, user.ID, eventKey, &prev); err != nil || !ok {
			s.failCourse(w, fmt.Errorf("повтор запроса: событие не найдено или нечитаемо (%v)", err))
			return
		}
		s.writeCompleted(w, r, user.ID, c, prev)
		return
	case err != nil:
		s.failCourse(w, err)
		return
	}
	s.writeCompleted(w, r, user.ID, c, resp)
}

// writeCompleted дописывает к итогу следующий урок (по свежему прогрессу) и отвечает.
func (s *authServer) writeCompleted(w http.ResponseWriter, r *http.Request, userID int64, c course.Course, resp api.CompleteLessonResponse) {
	done, err := s.completedLessons(r.Context(), userID)
	if err != nil {
		s.failCourse(w, err)
		return
	}
	resp.Next = coursePath(c, done).Next
	writeJSON(w, http.StatusOK, resp)
}
