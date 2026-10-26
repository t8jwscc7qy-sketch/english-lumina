package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sheets"
)

const maxSheetBody = 1 << 10

// sheetCatalog — встроенные шпаргалки, проверенные при первом обращении. Битый файл ловит тест содержимого
// (`go test ./content`); в рабочем сервере ошибка пишется в журнал, а каталог остаётся пустым.
var sheetCatalog = sync.OnceValue(func() []sheets.Sheet {
	all, err := sheets.LoadAll(content.Sheets)
	if err != nil {
		slog.Error("шпаргалки не загружены", "err", err)
		return nil
	}
	return all
})

func findSheet(id string) (sheets.Sheet, bool) {
	for _, s := range sheetCatalog() {
		if s.ID == id {
			return s, true
		}
	}
	return sheets.Sheet{}, false
}

func (s *authServer) failSheets(w http.ResponseWriter, err error) {
	slog.Error("шпаргалки", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// sheetData — данные пользователя, нужные для подкраски таблиц.
type sheetData struct {
	states    map[string]db.SheetStatesRow
	words     map[string]string  // лемма → статус слова
	topics    map[string]float64 // тема → рейтинг
	ratings   []float64
	mistakes  []api.Mistake
	needMiner bool
}

func (s *authServer) loadSheetData(ctx context.Context, userID int64, which []sheets.Sheet) (sheetData, error) {
	d := sheetData{states: map[string]db.SheetStatesRow{}, words: map[string]string{}, topics: map[string]float64{}}
	rows, err := s.store.SheetStates(ctx, userID)
	if err != nil {
		return d, fmt.Errorf("состояния шпаргалок: %w", err)
	}
	for _, r := range rows {
		d.states[r.SheetID] = r
	}
	// Леммы таблиц с привязкой word группируются по допустимым частям речи: один запрос на группу.
	groups := map[string][]string{}
	poss := map[string][]string{}
	for _, sh := range which {
		for _, b := range sh.Blocks {
			switch {
			case b.Type == sheets.BlockTable && b.Bind == sheets.BindMistake:
				d.needMiner = true
			case b.Type == sheets.BlockTable && b.Bind == sheets.BindWord:
				pk := posKey(b.Pos)
				poss[pk] = b.Pos
				for _, r := range b.Rows {
					if !slices.Contains(groups[pk], r.Key) {
						groups[pk] = append(groups[pk], r.Key)
					}
				}
			}
		}
	}
	for pk, lemmas := range groups {
		ws, err := s.store.SheetWordStatuses(ctx, db.SheetWordStatusesParams{UserID: userID, Lemmas: lemmas, Poss: nonNil(poss[pk])})
		if err != nil {
			return d, fmt.Errorf("статусы слов: %w", err)
		}
		per := map[string][]string{}
		for _, w := range ws {
			per[w.Lemma] = append(per[w.Lemma], sheets.WordStatus(w.LearningStatus, w.HasCard, int(w.Lapses)))
		}
		for lemma, st := range per {
			d.words[wordKey(lemma, pk)] = sheets.BestStatus(st...)
		}
	}
	ratings, err := s.store.ListTopicRatings(ctx, userID)
	if err != nil {
		return d, fmt.Errorf("рейтинги тем: %w", err)
	}
	for _, r := range ratings {
		d.topics[r.Topic] = r.Rating
		d.ratings = append(d.ratings, r.Rating)
	}
	if d.needMiner {
		open, err := s.store.ListOpenMistakes(ctx, db.ListOpenMistakesParams{UserID: userID, MaxRows: maxListedMistakes})
		if err != nil {
			return d, fmt.Errorf("открытые ошибки: %w", err)
		}
		for _, m := range open {
			d.mistakes = append(d.mistakes, mistakeToAPI(m))
		}
	}
	return d, nil
}

// posKey — ключ группы допустимых частей речи; пустой набор — любые.
func posKey(pos []string) string { return strings.Join(pos, ",") }

func wordKey(lemma, pk string) string { return lemma + "|" + pk }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// status: «проверено» действует, пока текст шпаргалки такой же, как при проверке (по хешу файла). Правка текста
// возвращает шпаргалку в черновик: сервис не выдаёт непроверенное за проверенное.
func (d sheetData) status(sh sheets.Sheet) api.SheetStatus {
	if st, ok := d.states[sh.ID]; ok && st.ReviewedAt.Valid && st.ReviewedHash != nil && *st.ReviewedHash == sh.Hash {
		return "reviewed"
	}
	return "draft"
}

// rowsFor подкрашивает строки таблицы данными пользователя и считает «слабые».
func (d sheetData) rowsFor(b sheets.Block) (rows []api.SheetRow, weak int) {
	if b.Bind == sheets.BindMistake {
		rows = []api.SheetRow{} // [], а не null
		mr := make([]sheets.MistakeRow, len(d.mistakes))
		for i, m := range d.mistakes {
			mr[i] = sheets.MistakeRow{Prompt: m.Prompt, Given: m.Given, Answer: m.Answer}
		}
		for _, r := range sheets.MineMistakes(mr) {
			rows = append(rows, api.SheetRow{Cells: r.Cells})
		}
		return rows, len(rows)
	}
	rows = make([]api.SheetRow, 0, len(b.Rows))
	for _, r := range b.Rows {
		row := api.SheetRow{Cells: r.Cells}
		if r.Key != "" {
			k := r.Key
			row.Key = &k
		}
		if r.NeedsCheck {
			t := true
			row.NeedsCheck = &t
		}
		switch b.Bind {
		case sheets.BindWord:
			st, ok := d.words[wordKey(r.Key, posKey(b.Pos))]
			if !ok {
				st = sheets.WordUnseen
			}
			v := api.SheetRowStatus(st)
			row.Status = &v
			if st == sheets.WordHard {
				weak++
			}
		case sheets.BindTopic:
			v := api.SheetRowStatus("unseen")
			if rating, ok := d.topics[r.Key]; ok {
				f := float32(rating)
				row.Rating = &f
				v = "ok"
				if sheets.TopicWeak(rating, d.ratings) {
					v = "weak"
					weak++
				}
			}
			row.Status = &v
		}
		rows = append(rows, row)
	}
	return rows, weak
}

func (d sheetData) weakCount(sh sheets.Sheet) int {
	n := 0
	for _, b := range sh.Blocks {
		if b.Type == sheets.BlockTable && b.Bind != "" {
			_, w := d.rowsFor(b)
			n += w
		}
	}
	return n
}

func (d sheetData) summary(sh sheets.Sheet) api.SheetSummary {
	out := api.SheetSummary{
		Id: sh.ID, Title: sh.Title, Summary: sh.Summary, Level: api.SheetSummaryLevel(sh.Level), Tags: sh.Tags,
		Favorite: d.states[sh.ID].FavoriteAt.Valid, Status: d.status(sh), WeakCount: d.weakCount(sh),
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if at := d.states[sh.ID].OpenedAt; at.Valid {
		t := at.Time
		out.OpenedAt = &t
	}
	return out
}

func (d sheetData) detail(sh sheets.Sheet) api.Sheet {
	sum := d.summary(sh)
	out := api.Sheet{
		Id: sum.Id, Title: sum.Title, Summary: sum.Summary, Level: api.SheetLevel(sh.Level), Tags: sum.Tags,
		Favorite: sum.Favorite, Status: sum.Status, WeakCount: sum.WeakCount, Sources: sh.Sources,
		Blocks: make([]api.SheetBlock, 0, len(sh.Blocks)),
	}
	for _, b := range sh.Blocks {
		blk := api.SheetBlock{Type: api.SheetBlockType(b.Type)}
		if b.Title != "" {
			t := b.Title
			blk.Title = &t
		}
		if b.Text != "" {
			t := b.Text
			blk.Text = &t
		}
		switch b.Type {
		case sheets.BlockTable:
			if b.Bind != "" {
				bind := api.SheetBlockBind(b.Bind)
				blk.Bind = &bind
			}
			cols := make([]api.SheetColumn, len(b.Columns))
			for i, c := range b.Columns {
				cols[i] = api.SheetColumn{Title: c.Title}
			}
			rows, _ := d.rowsFor(b)
			blk.Columns, blk.Rows = &cols, &rows
		case sheets.BlockMistakes:
			pairs := make([]api.SheetPair, len(b.Pairs))
			for i, p := range b.Pairs {
				pairs[i] = api.SheetPair{Wrong: p.Wrong, Right: p.Right, Note: p.Note}
			}
			blk.Pairs = &pairs
		case sheets.BlockExamples:
			ex := make([]api.SheetExample, len(b.Examples))
			for i, e := range b.Examples {
				ex[i] = api.SheetExample{En: e.En, Ru: e.Ru}
			}
			blk.Examples = &ex
		}
		out.Blocks = append(out.Blocks, blk)
	}
	return out
}

// ListSheets отдаёт все шпаргалки с признаками пользователя.
func (s *authServer) ListSheets(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	all := sheetCatalog()
	d, err := s.loadSheetData(r.Context(), user.ID, all)
	if err != nil {
		s.failSheets(w, err)
		return
	}
	out := api.SheetList{Sheets: make([]api.SheetSummary, 0, len(all))}
	for _, sh := range all {
		out.Sheets = append(out.Sheets, d.summary(sh))
	}
	writeJSON(w, http.StatusOK, out)
}

// decodeSheetBody читает тело запроса строго: неизвестные поля и лишний текст после объекта — ошибка 400.
func decodeSheetBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSheetBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный запрос")
		return false
	}
	return true
}

const maxSheetID = 64

func validSheetID(id string) bool { return id != "" && len(id) <= maxSheetID }

func badSheetRequest(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны id шпаргалки (до 64 символов) и все поля запроса")
}

func sheetNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, api.ErrorErrorCodeNotFound, "Такой шпаргалки нет")
}

// OpenSheet отдаёт шпаргалку с данными пользователя и запоминает время открытия.
func (s *authServer) OpenSheet(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.SheetRef
	if !decodeSheetBody(w, r, &req) {
		return
	}
	if !validSheetID(req.Id) {
		badSheetRequest(w)
		return
	}
	sh, ok := findSheet(req.Id)
	if !ok {
		sheetNotFound(w)
		return
	}
	ctx := r.Context()
	if err := s.store.SheetTouchOpened(ctx, db.SheetTouchOpenedParams{UserID: user.ID, SheetID: sh.ID, At: ts(s.now())}); err != nil {
		s.failSheets(w, fmt.Errorf("время открытия: %w", err))
		return
	}
	d, err := s.loadSheetData(ctx, user.ID, []sheets.Sheet{sh})
	if err != nil {
		s.failSheets(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.detail(sh))
}

func (s *authServer) sheetEvent(ctx context.Context, tx AuthStore, userID int64, typ string, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: userID, Type: typ, SchemaVersion: 1, OccurredAt: ts(s.now()), Payload: payload})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

// SetSheetFavorite закрепляет или открепляет шпаргалку; событие пишется только при изменении.
func (s *authServer) SetSheetFavorite(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req struct {
		Id       string `json:"id"`
		Favorite *bool  `json:"favorite"`
	}
	if !decodeSheetBody(w, r, &req) {
		return
	}
	if !validSheetID(req.Id) || req.Favorite == nil {
		badSheetRequest(w)
		return
	}
	favorite := *req.Favorite
	sh, ok := findSheet(req.Id)
	if !ok {
		sheetNotFound(w)
		return
	}
	ctx := r.Context()
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		var n int64
		var err error
		if favorite {
			n, err = tx.SheetSetFavorite(ctx, db.SheetSetFavoriteParams{UserID: user.ID, SheetID: sh.ID, At: ts(s.now())})
		} else {
			n, err = tx.SheetClearFavorite(ctx, db.SheetClearFavoriteParams{UserID: user.ID, SheetID: sh.ID})
		}
		if err != nil || n == 0 {
			return err
		}
		return s.sheetEvent(ctx, tx, user.ID, "sheet.favorited", map[string]any{"sheetId": sh.ID, "favorite": favorite})
	})
	if err != nil {
		s.failSheets(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.SheetFavoriteRequest{Id: sh.ID, Favorite: favorite})
}

// ReviewSheet отмечает шпаргалку проверенной самим пользователем (один раз).
func (s *authServer) ReviewSheet(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.SheetRef
	if !decodeSheetBody(w, r, &req) {
		return
	}
	if !validSheetID(req.Id) {
		badSheetRequest(w)
		return
	}
	sh, ok := findSheet(req.Id)
	if !ok {
		sheetNotFound(w)
		return
	}
	ctx := r.Context()
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		n, err := tx.SheetSetReviewed(ctx, db.SheetSetReviewedParams{UserID: user.ID, SheetID: sh.ID, At: ts(s.now()), Hash: sh.Hash})
		if err != nil || n == 0 {
			return err
		}
		return s.sheetEvent(ctx, tx, user.ID, "sheet.reviewed", map[string]any{"sheetId": sh.ID})
	})
	if err != nil {
		s.failSheets(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.SheetReviewed{Id: sh.ID, Status: "reviewed"})
}
