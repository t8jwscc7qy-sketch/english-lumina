package httpapi

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) TextInsert(_ context.Context, a db.TextInsertParams) (int64, error) {
	if f.texts == nil {
		f.texts = map[int64]db.TextGetRow{}
	}
	f.nextTextID++
	f.texts[f.nextTextID] = db.TextGetRow{ID: f.nextTextID, Kind: a.Kind, Title: a.Title, Body: a.Body, WordCount: a.WordCount, CoverageAtBuild: a.CoverageAtBuild, Goal: a.Goal, CreatedAt: pgtype.Timestamptz{}}
	return f.nextTextID, nil
}

func (f *fakeStore) TextGet(_ context.Context, a db.TextGetParams) (db.TextGetRow, error) {
	t, ok := f.texts[a.ID]
	if !ok {
		return db.TextGetRow{}, pgx.ErrNoRows
	}
	return t, nil
}

func (f *fakeStore) TextRateInsert(_ context.Context, a db.TextRateInsertParams) (int64, error) {
	if f.textRated == nil {
		f.textRated = map[int64]bool{}
	}
	if f.textRated[a.TextID] {
		return 0, nil
	}
	f.textRated[a.TextID] = true
	return 1, nil
}

func (f *fakeStore) TextRecentSentenceIDs(_ context.Context, a db.TextRecentSentenceIDsParams) ([]int64, error) {
	var out []int64
	for id := f.nextTextID; id > 0 && f.nextTextID-id < int64(a.MaxTexts); id-- {
		var body struct {
			Sentences []struct {
				ID int64 `json:"id"`
			} `json:"sentences"`
		}
		if err := json.Unmarshal(f.texts[id].Body, &body); err != nil {
			return nil, err
		}
		for _, s := range body.Sentences {
			if !slices.Contains(out, s.ID) {
				out = append(out, s.ID)
			}
		}
	}
	return out, nil
}

func (f *fakeStore) TextTargetWords(_ context.Context, a db.TextTargetWordsParams) ([]db.TextTargetWordsRow, error) {
	var out []db.TextTargetWordsRow
	for _, r := range f.textTargets {
		if w, ok := f.wl[r.LexemeID]; ok && (w.status == "learned" || w.status == "known_at_start" || w.status == "introduced") {
			continue
		}
		out = append(out, r)
	}
	if int32(len(out)) > a.MaxRows {
		out = out[:a.MaxRows]
	}
	return out, nil
}

func (f *fakeStore) TextWordHints(_ context.Context, lemmas []string) ([]db.TextWordHintsRow, error) {
	var out []db.TextWordHintsRow
	for _, h := range f.textHints {
		if slices.Contains(lemmas, h.Lemma) {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakeStore) TextSetCoverage(_ context.Context, a db.TextSetCoverageParams) error {
	p := db.LearnPreference{UserID: a.UserID, NewWordsMode: "auto", NewWordsPerDay: 12, CampCoverage: 0.95}
	if f.learnPrefs != nil {
		p = *f.learnPrefs
	}
	p.TextCoverage = a.TextCoverage
	f.learnPrefs = &p
	return nil
}

func (f *fakeStore) TextLastUnrated(_ context.Context, _ int64) (db.TextLastUnratedRow, error) {
	best := int64(0)
	for id := range f.textShown {
		if !f.textRated[id] && f.textShown[id] && id > best {
			best = id
		}
	}
	if best == 0 {
		return db.TextLastUnratedRow{}, pgx.ErrNoRows
	}
	t := f.texts[best]
	return db.TextLastUnratedRow{ID: t.ID, Kind: t.Kind, Title: t.Title, Body: t.Body, WordCount: t.WordCount, CoverageAtBuild: t.CoverageAtBuild, Goal: t.Goal}, nil
}

func (f *fakeStore) TextShownInsert(_ context.Context, a db.TextShownInsertParams) error {
	if f.textShown == nil {
		f.textShown = map[int64]bool{}
	}
	f.textShown[a.TextID] = true
	return nil
}

func (f *fakeStore) TextUpsertAuthored(_ context.Context, a db.TextUpsertAuthoredParams) (int64, error) {
	if f.texts == nil {
		f.texts = map[int64]db.TextGetRow{}
	}
	for id, t := range f.texts {
		if t.Kind == "authored" && t.Title == a.Title {
			return id, nil
		}
	}
	f.nextTextID++
	f.texts[f.nextTextID] = db.TextGetRow{ID: f.nextTextID, Kind: "authored", Title: a.Title, Body: a.Body, WordCount: a.WordCount}
	return f.nextTextID, nil
}

func (f *fakeStore) TextReadIDs(_ context.Context, _ int64) ([]int64, error) {
	var out []int64
	for id, rated := range f.textRated {
		if rated {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakeStore) SentenceAudioFor(_ context.Context, a db.SentenceAudioForParams) ([]db.SentenceAudioForRow, error) {
	var out []db.SentenceAudioForRow
	for _, row := range f.audio {
		if slices.Contains(a.Ids, row.SentenceTatoebaID) && f.audioOK(a.Licenses, row) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) audioOK(licenses []string, row db.SentenceAudioForRow) bool {
	return slices.Contains(licenses, row.License)
}
