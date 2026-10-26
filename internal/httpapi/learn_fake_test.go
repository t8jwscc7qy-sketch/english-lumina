package httpapi

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// fakeWord — строка word_learning в памяти.
type fakeWord struct {
	status, source string
	introducedAt   time.Time
}

// errLearnDown — как будто таблиц базы знаний нет (код PostgreSQL 42P01).
var errLearnDown error = &pgconn.PgError{Code: "42P01", Message: "relation \"kb_teaching_card\" does not exist"}

func (f *fakeStore) hasCard(userID, lexemeID int64) bool {
	for _, c := range f.cards {
		if c.UserID == userID && c.LexemeID == lexemeID {
			return true
		}
	}
	return false
}

func (f *fakeStore) KbLatestBuild(context.Context) (db.KbBuild, error) {
	if f.failLearn {
		return db.KbBuild{}, errLearnDown
	}
	if !f.kbBuilt {
		return db.KbBuild{}, pgx.ErrNoRows
	}
	return db.KbBuild{ID: 1, Status: "ok"}, nil
}

// LearnCandidates повторяет SQL: уровень из списка, нет карточки и состояния изучения, порядок и лимит.
func (f *fakeStore) LearnCandidates(_ context.Context, arg db.LearnCandidatesParams) ([]db.LearnCandidatesRow, error) {
	if f.failLearn {
		return nil, errLearnDown
	}
	if f.candidatesErr != nil {
		return nil, f.candidatesErr
	}
	var out []db.LearnCandidatesRow
	for _, r := range f.learnCands {
		if _, learned := f.wl[r.LexemeID]; learned || f.hasCard(arg.UserID, r.LexemeID) || !slices.Contains(arg.Levels, r.Level) {
			continue
		}
		out = append(out, r)
	}
	start := min(int(arg.RowOffset), len(out))
	return out[start:min(start+int(arg.MaxRows), len(out))], nil
}

func (f *fakeStore) LearnPlacementCandidates(_ context.Context, arg db.LearnPlacementCandidatesParams) ([]db.LearnPlacementCandidatesRow, error) {
	if f.failLearn {
		return nil, errLearnDown
	}
	var all []db.LearnPlacementCandidatesRow
	for _, r := range f.learnPlacement {
		if _, ok := f.wl[r.LexemeID]; ok || f.hasCard(arg.UserID, r.LexemeID) {
			continue
		}
		all = append(all, r)
	}
	start := min(int(arg.RowOffset), len(all))
	end := min(start+int(arg.MaxRows), len(all))
	return all[start:end], nil
}

func (f *fakeStore) LearnRelations(_ context.Context, ids []int64) ([]db.LearnRelationsRow, error) {
	var out []db.LearnRelationsRow
	for _, r := range f.learnRelations {
		if slices.Contains(ids, r.LexemeID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) LearnStatusCounts(_ context.Context, _ int64) ([]db.LearnStatusCountsRow, error) {
	counts := map[string]int64{}
	for _, w := range f.wl {
		counts[w.status]++
	}
	var out []db.LearnStatusCountsRow
	for status, n := range counts {
		out = append(out, db.LearnStatusCountsRow{Status: status, Count: n})
	}
	if f.extraTracked > 0 { // слова «в учёте» без отдельных строк: выводят тест из «холодного старта»
		out = append(out, db.LearnStatusCountsRow{Status: "extra", Count: int64(f.extraTracked)})
	}
	return out, nil
}

func (f *fakeStore) LearnIntroducedSince(_ context.Context, arg db.LearnIntroducedSinceParams) (int64, error) {
	var n int64
	for _, w := range f.wl {
		if w.source == "lesson" && !w.introducedAt.Before(arg.Since.Time) {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) LearnInsertIntroduced(_ context.Context, arg db.LearnInsertIntroducedParams) (int64, error) {
	if _, ok := f.wl[arg.LexemeID]; ok {
		return 0, nil
	}
	if f.wl == nil {
		f.wl = map[int64]fakeWord{}
	}
	f.wl[arg.LexemeID] = fakeWord{status: "introduced", source: "lesson", introducedAt: arg.IntroducedAt.Time}
	return 1, nil
}

func (f *fakeStore) LearnMarkKnown(_ context.Context, arg db.LearnMarkKnownParams) (int64, error) {
	if _, ok := f.wl[arg.LexemeID]; ok || f.hasCard(arg.UserID, arg.LexemeID) {
		return 0, nil
	}
	if f.wl == nil {
		f.wl = map[int64]fakeWord{}
	}
	f.wl[arg.LexemeID] = fakeWord{status: "known_at_start", source: "placement"}
	return 1, nil
}

// LearnMarkLearned повторяет SQL: нет строки — создаётся learned/card; introduced → learned; иначе без изменений.
func (f *fakeStore) LearnMarkLearned(_ context.Context, arg db.LearnMarkLearnedParams) (int64, error) {
	if f.markLearnedErr != nil {
		return 0, f.markLearnedErr
	}
	w, ok := f.wl[arg.LexemeID]
	if f.wl == nil {
		f.wl = map[int64]fakeWord{}
	}
	switch {
	case !ok:
		f.wl[arg.LexemeID] = fakeWord{status: "learned", source: "card"}
		return 1, nil
	case w.status == "introduced":
		w.status = "learned"
		f.wl[arg.LexemeID] = w
		return 1, nil
	}
	return 0, nil
}

func (f *fakeStore) LearnReviewStats(_ context.Context, _ db.LearnReviewStatsParams) (db.LearnReviewStatsRow, error) {
	return db.LearnReviewStatsRow{Total: f.reviewTotal, Good: f.reviewGood}, nil
}

func (f *fakeStore) LearnGetPreferences(_ context.Context, userID int64) (db.LearnPreference, error) {
	if f.learnPrefs == nil || f.learnPrefs.UserID != userID {
		return db.LearnPreference{}, pgx.ErrNoRows
	}
	return *f.learnPrefs, nil
}

func (f *fakeStore) LearnUpsertPreferences(_ context.Context, arg db.LearnUpsertPreferencesParams) (db.LearnPreference, error) {
	p := db.LearnPreference{UserID: arg.UserID, NewWordsMode: arg.NewWordsMode, NewWordsPerDay: arg.NewWordsPerDay, CampCoverage: arg.CampCoverage}
	f.learnPrefs = &p
	return p, nil
}

func (f *fakeStore) LearnKnownWords(_ context.Context, _ int64) ([]db.LearnKnownWordsRow, error) {
	var out []db.LearnKnownWordsRow
	for id, w := range f.wl {
		if w.status == "learned" || w.status == "known_at_start" {
			lx := f.lexemesByID[id]
			out = append(out, db.LearnKnownWordsRow{LexemeID: id, Lemma: lx.Lemma, Pos: lx.Pos})
		}
	}
	slices.SortFunc(out, func(a, b db.LearnKnownWordsRow) int { return int(a.LexemeID - b.LexemeID) })
	return out, nil
}

func (f *fakeStore) TextKnownWords(_ context.Context, _ int64) ([]db.TextKnownWordsRow, error) {
	var out []db.TextKnownWordsRow
	for id, w := range f.wl {
		if w.status == "learned" || w.status == "known_at_start" || w.status == "introduced" {
			lx := f.lexemesByID[id]
			out = append(out, db.TextKnownWordsRow{LexemeID: id, Lemma: lx.Lemma, Pos: lx.Pos})
		}
	}
	slices.SortFunc(out, func(a, b db.TextKnownWordsRow) int { return int(a.LexemeID - b.LexemeID) })
	return out, nil
}
