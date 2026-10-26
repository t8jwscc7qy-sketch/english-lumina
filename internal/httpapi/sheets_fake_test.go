package httpapi

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) SheetStates(_ context.Context, userID int64) ([]db.SheetStatesRow, error) {
	var out []db.SheetStatesRow
	for id, st := range f.sheetStates {
		out = append(out, db.SheetStatesRow{SheetID: id, FavoriteAt: st.FavoriteAt, ReviewedAt: st.ReviewedAt, OpenedAt: st.OpenedAt})
	}
	_ = userID
	return out, nil
}

func (f *fakeStore) sheetState(id string) db.SheetStatesRow {
	if f.sheetStates == nil {
		f.sheetStates = map[string]db.SheetStatesRow{}
	}
	st := f.sheetStates[id]
	st.SheetID = id
	return st
}

func (f *fakeStore) SheetSetFavorite(_ context.Context, a db.SheetSetFavoriteParams) (int64, error) {
	st := f.sheetState(a.SheetID)
	if st.FavoriteAt.Valid {
		return 0, nil
	}
	st.FavoriteAt = a.At
	f.sheetStates[a.SheetID] = st
	return 1, nil
}

func (f *fakeStore) SheetClearFavorite(_ context.Context, a db.SheetClearFavoriteParams) (int64, error) {
	st, ok := f.sheetStates[a.SheetID]
	if !ok || !st.FavoriteAt.Valid {
		return 0, nil
	}
	st.FavoriteAt = pgtype.Timestamptz{}
	f.sheetStates[a.SheetID] = st
	return 1, nil
}

func (f *fakeStore) SheetSetReviewed(_ context.Context, a db.SheetSetReviewedParams) (int64, error) {
	st := f.sheetState(a.SheetID)
	if st.ReviewedAt.Valid {
		return 0, nil
	}
	st.ReviewedAt = a.At
	f.sheetStates[a.SheetID] = st
	return 1, nil
}

func (f *fakeStore) SheetTouchOpened(_ context.Context, a db.SheetTouchOpenedParams) error {
	st := f.sheetState(a.SheetID)
	st.OpenedAt = a.At
	f.sheetStates[a.SheetID] = st
	return nil
}

// SheetWordStatuses повторяет SQL: по каждому слову леммы — состояние изучения, карточка и число забываний.
func (f *fakeStore) SheetWordStatuses(_ context.Context, a db.SheetWordStatusesParams) ([]db.SheetWordStatusesRow, error) {
	var out []db.SheetWordStatusesRow
	for id, lx := range f.lexemesByID {
		if !slices.Contains(a.Lemmas, lx.Lemma) {
			continue
		}
		row := db.SheetWordStatusesRow{Lemma: lx.Lemma}
		if w, ok := f.wl[id]; ok {
			s := w.status
			row.LearningStatus = &s
		}
		for _, c := range f.cards {
			if c.UserID == a.UserID && c.LexemeID == id {
				row.HasCard, row.Lapses = true, c.Lapses
			}
		}
		out = append(out, row)
	}
	return out, nil
}
