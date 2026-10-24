package httpapi

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// Память вместо базы для запросов упражнений. Откат при ошибке делает fakeStore.InTx (cards_fake_test.go).

type topicKey struct {
	userID int64
	topic  string
}

func (f *fakeStore) InsertExercise(_ context.Context, arg db.InsertExerciseParams) (int64, error) {
	if f.exercises == nil {
		f.exercises = map[int64]db.Exercise{}
	}
	f.nextExerciseID++
	f.exercises[f.nextExerciseID] = db.Exercise{
		ID: f.nextExerciseID, UserID: arg.UserID, Kind: arg.Kind, SentenceID: arg.SentenceID,
		Topics: slices.Clone(arg.Topics), Payload: arg.Payload, Answer: arg.Answer,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	return f.nextExerciseID, nil
}

func (f *fakeStore) GetExerciseForUpdate(_ context.Context, arg db.GetExerciseForUpdateParams) (db.Exercise, error) {
	e, ok := f.exercises[arg.ID]
	if !ok || e.UserID != arg.UserID {
		return db.Exercise{}, pgx.ErrNoRows
	}
	return e, nil
}

func (f *fakeStore) MarkExerciseAnswered(_ context.Context, arg db.MarkExerciseAnsweredParams) error {
	e, ok := f.exercises[arg.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	if !e.AnsweredAt.Valid { // как в базе: повтор время не меняет
		e.AnsweredAt = arg.AnsweredAt
		f.exercises[arg.ID] = e
	}
	return nil
}

// EnsureTopicRating: как в базе, создаёт начальную строку, если её нет (upserts не увеличивает).
func (f *fakeStore) EnsureTopicRating(_ context.Context, arg db.EnsureTopicRatingParams) error {
	if f.topicRatings == nil {
		f.topicRatings = map[topicKey]db.TopicRating{}
	}
	k := topicKey{arg.UserID, arg.Topic}
	if _, ok := f.topicRatings[k]; !ok {
		f.topicRatings[k] = db.TopicRating{UserID: arg.UserID, Topic: arg.Topic, Rating: 1500, Deviation: 350, Volatility: 0.06,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	}
	return nil
}

func (f *fakeStore) ListTopicRatings(_ context.Context, userID int64) ([]db.TopicRating, error) {
	var out []db.TopicRating
	for k, r := range f.topicRatings {
		if k.userID == userID {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b db.TopicRating) int {
		switch {
		case a.Topic < b.Topic:
			return -1
		case a.Topic > b.Topic:
			return 1
		}
		return 0
	})
	return out, nil
}

func (f *fakeStore) GetTopicRatingForUpdate(_ context.Context, arg db.GetTopicRatingForUpdateParams) (db.TopicRating, error) {
	r, ok := f.topicRatings[topicKey{arg.UserID, arg.Topic}]
	if !ok {
		return db.TopicRating{}, pgx.ErrNoRows
	}
	return r, nil
}

func (f *fakeStore) UpsertTopicRating(_ context.Context, arg db.UpsertTopicRatingParams) error {
	f.upserts++
	if f.topicRatings == nil {
		f.topicRatings = map[topicKey]db.TopicRating{}
	}
	k := topicKey{arg.UserID, arg.Topic}
	r := f.topicRatings[k]
	r.UserID, r.Topic, r.Rating, r.Deviation, r.Volatility = arg.UserID, arg.Topic, arg.Rating, arg.Deviation, arg.Volatility
	r.Attempts += arg.AttemptsDelta // как в базе: попытки накапливаются
	r.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	f.topicRatings[k] = r
	return nil
}

// SampleSentences: фейк не фильтрует по словам, длине и уровню, как SQL; строки пула с id >= FromID по возрастанию, не больше MaxRows.
func (f *fakeStore) SampleSentences(_ context.Context, arg db.SampleSentencesParams) ([]db.SampleSentencesRow, error) {
	if f.sampleErr != nil {
		return nil, f.sampleErr
	}
	var out []db.SampleSentencesRow
	for _, s := range f.sentencePool {
		if s.ID >= arg.FromID && int32(len(out)) < arg.MaxRows {
			out = append(out, s)
		}
	}
	return out, nil
}

// SentenceLevels: строки только для слов, которые есть в wordLevels; по алфавиту.
func (f *fakeStore) SentenceLevels(_ context.Context, words []string) ([]db.SentenceLevelsRow, error) {
	f.levelCalls++
	var out []db.SentenceLevelsRow
	for _, w := range words {
		if r, ok := f.wordLevels[w]; ok {
			r.Word = w
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b db.SentenceLevelsRow) int {
		switch {
		case a.Word < b.Word:
			return -1
		case a.Word > b.Word:
			return 1
		}
		return 0
	})
	return out, nil
}

// UsedSentences: предложения, на которые пользователь уже ответил в теме (как в запросе: answered_at IS NOT NULL).
func (f *fakeStore) UsedSentences(_ context.Context, arg db.UsedSentencesParams) ([]int64, error) {
	var out []int64
	for _, e := range f.exercises {
		if e.UserID == arg.UserID && e.AnsweredAt.Valid && slices.Contains(e.Topics, arg.Topic) {
			out = append(out, e.SentenceID)
		}
	}
	slices.Sort(out)
	return out, nil
}

// SentenceIDRange: границы номеров пула (0, 0, если пул пуст).
func (f *fakeStore) SentenceIDRange(context.Context) (db.SentenceIDRangeRow, error) {
	f.rangeCalls++
	if f.rangeOverride != nil {
		return *f.rangeOverride, nil
	}
	var out db.SentenceIDRangeRow
	for i, s := range f.sentencePool {
		if i == 0 || s.ID < out.MinID {
			out.MinID = s.ID
		}
		if s.ID > out.MaxID {
			out.MaxID = s.ID
		}
	}
	return out, nil
}

// RandomLexemesByLevel: первые MaxRows слов уровня из levelLexemes (порядок задаёт тест).
func (f *fakeStore) RandomLexemesByLevel(_ context.Context, arg db.RandomLexemesByLevelParams) ([]db.RandomLexemesByLevelRow, error) {
	rows := f.levelLexemes[arg.Level]
	if int(arg.MaxRows) < len(rows) {
		rows = rows[:arg.MaxRows]
	}
	return slices.Clone(rows), nil
}

// exerciseSnapshot — снимок состояния упражнений и рейтингов для отката в InTx.
type exerciseSnapshot struct {
	exercises map[int64]db.Exercise
	ratings   map[topicKey]db.TopicRating
	nextID    int64
	upserts   int
}

func (f *fakeStore) snapshotExercises() exerciseSnapshot {
	return exerciseSnapshot{maps.Clone(f.exercises), maps.Clone(f.topicRatings), f.nextExerciseID, f.upserts}
}

func (f *fakeStore) restoreExercises(s exerciseSnapshot) {
	f.exercises, f.topicRatings, f.nextExerciseID, f.upserts = s.exercises, s.ratings, s.nextID, s.upserts
}

func (f *fakeStore) GetEventByKey(_ context.Context, arg db.GetEventByKeyParams) (db.Event, error) {
	for _, ev := range f.events {
		if ev.UserID == arg.UserID && ev.IdempotencyKey != nil && arg.IdempotencyKey != nil && *ev.IdempotencyKey == *arg.IdempotencyKey {
			return ev, nil
		}
	}
	return db.Event{}, pgx.ErrNoRows
}
