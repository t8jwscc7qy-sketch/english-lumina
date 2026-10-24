// Package srs — интервальные повторения: тонкая обёртка над библиотекой FSRS (go-fsrs/v4).
// Только этот пакет знает о библиотеке; остальной код работает с типами ниже. Параметры по умолчанию
// (удержание 0,9; максимальный интервал 36 500 дней), случайный разброс интервалов выключен:
// результат воспроизводим.
package srs

import (
	"errors"
	"fmt"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Rating — самооценка ответа.
type Rating int

const (
	Again Rating = 1 // забыл
	Hard  Rating = 2 // трудно
	Good  Rating = 3 // хорошо
	Easy  Rating = 4 // легко
)

// State — стадия карточки.
type State int

const (
	StateNew        State = 0
	StateLearning   State = 1
	StateReview     State = 2
	StateRelearning State = 3
)

// ErrBadRating — оценка вне 1–4 (в том числе «ручная» 0 из библиотеки).
var ErrBadRating = errors.New("оценка должна быть от 1 до 4")

// Card — состояние карточки, которое хранится в базе.
type Card struct {
	Due            time.Time
	Stability      float64
	Difficulty     float64
	ScheduledDays  int
	Reps           int
	Lapses         int
	State          State
	LastReview     time.Time // нулевое значение — карточку ещё не повторяли
	RemainingSteps int
}

// New возвращает новую карточку со сроком «сейчас».
func New(now time.Time) Card {
	return Card{Due: now, State: StateNew}
}

func newScheduler() *fsrs.FSRS { return fsrs.NewFSRS(fsrs.DefaultParam()) }

// Review применяет оценку и возвращает новое состояние; переданная карточка не меняется.
func Review(c Card, r Rating, now time.Time) (Card, error) {
	if r < Again || r > Easy {
		return Card{}, ErrBadRating
	}
	lib, err := toLib(c)
	if err != nil {
		return Card{}, err
	}
	info, err := newScheduler().Next(lib, now, fsrs.Rating(r))
	if err != nil {
		return Card{}, fmt.Errorf("srs: %w", err)
	}
	return fromLib(info.Card), nil
}

// Preview возвращает, через сколько вернётся карточка при каждой из четырёх оценок.
func Preview(c Card, now time.Time) (map[Rating]time.Duration, error) {
	lib, err := toLib(c)
	if err != nil {
		return nil, err
	}
	log, err := newScheduler().Repeat(lib, now)
	if err != nil {
		return nil, fmt.Errorf("srs: %w", err)
	}
	out := make(map[Rating]time.Duration, 4)
	for _, r := range []Rating{Again, Hard, Good, Easy} {
		out[r] = log[fsrs.Rating(r)].Card.Due.Sub(now)
	}
	return out, nil
}

func toLib(c Card) (fsrs.Card, error) {
	if c.ScheduledDays < 0 || c.Reps < 0 || c.Lapses < 0 || c.RemainingSteps < 0 || c.State < StateNew || c.State > StateRelearning {
		return fsrs.Card{}, fmt.Errorf("srs: недопустимая карточка: %+v", c)
	}
	return fsrs.Card{
		Due: c.Due, Stability: c.Stability, Difficulty: c.Difficulty,
		ScheduledDays: uint64(c.ScheduledDays), Reps: uint64(c.Reps), Lapses: uint64(c.Lapses),
		State: fsrs.State(c.State), LastReview: c.LastReview, RemainingSteps: c.RemainingSteps,
	}, nil
}

func fromLib(c fsrs.Card) Card {
	return Card{
		Due: c.Due, Stability: c.Stability, Difficulty: c.Difficulty,
		ScheduledDays: int(c.ScheduledDays), Reps: int(c.Reps), Lapses: int(c.Lapses), //nolint:gosec // значения малы: сроки и счётчики повторений
		State: State(c.State), LastReview: c.LastReview, RemainingSteps: c.RemainingSteps,
	}
}
