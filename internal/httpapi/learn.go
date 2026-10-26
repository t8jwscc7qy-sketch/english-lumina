package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/learn"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/mistakes"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/pace"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/srs"
)

const (
	maxLearnBody        = 4 << 10
	maxLearnKey         = 64
	learnBlockSize      = 7  // слов в одном блоке знакомства
	learnCandidateRows  = 80 // кандидатов на одной странице выборки
	learnCandidatePages = 10 // страниц выборки: слова без примеров остаются в начале списка, поэтому дочитываем дальше
	learnExamplesShown  = 2
	learnListCap        = 8 // форм и синонимов в карточке слова
	placementPageSize   = 50
	placementMaxSkip    = 2000
	accuracyWindowDays  = 7
	minAccuracySample   = 10 // меньше повторений за неделю — точность не считается
	placementMoreShare  = 0.8
	placementStopShare  = 0.5

	defaultPerDay   = 12
	defaultCoverage = 0.95
	minCoverage     = 0.90
	maxCoverage     = 0.99
	maxPerDay       = 30
	audioBaseURL    = "https://commons.wikimedia.org/wiki/Special:FilePath/"
)

// isMissingTable: PostgreSQL ответил «таблицы нет» (миграция базы знаний не применена).
func isMissingTable(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "42P01"
}

func (s *authServer) failLearn(w http.ResponseWriter, err error) {
	slog.Error("знакомство", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// learnSetup — всё, что нужно любой ручке знакомства: настройки, предпочтения и суточный лимит.
type learnSetup struct {
	settings   db.Setting
	prefs      db.LearnPreference
	pace       pace.Result
	introduced int // слов введено сегодня
	now        time.Time
	dayStart   time.Time
}

func (s *authServer) learnPrefs(ctx context.Context, userID int64) (db.LearnPreference, error) {
	p, err := s.store.LearnGetPreferences(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.LearnPreference{UserID: userID, NewWordsMode: "auto", NewWordsPerDay: defaultPerDay, CampCoverage: defaultCoverage}, nil
	}
	return p, err
}

// learnContext читает настройки и считает лимит. ok = false — ответ уже записан (нет настроек или ошибка).
func (s *authServer) learnContext(w http.ResponseWriter, r *http.Request, userID int64) (learnSetup, bool) {
	ctx := r.Context()
	st, err := s.store.GetSettings(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeOnboardingRequired, "Сначала настрой цель и уровень")
		return learnSetup{}, false
	}
	if err != nil {
		s.failLearn(w, fmt.Errorf("настройки: %w", err))
		return learnSetup{}, false
	}
	out := learnSetup{settings: st, now: s.now()}
	if out.prefs, err = s.learnPrefs(ctx, userID); err != nil {
		s.failLearn(w, fmt.Errorf("предпочтения: %w", err))
		return learnSetup{}, false
	}
	out.dayStart = mistakes.DayStart(out.now, st.Timezone)
	introduced, err := s.store.LearnIntroducedSince(ctx, db.LearnIntroducedSinceParams{UserID: userID, Since: ts(out.dayStart)})
	if err != nil {
		s.failLearn(w, fmt.Errorf("введено сегодня: %w", err))
		return learnSetup{}, false
	}
	counts, err := s.store.CountCards(ctx, db.CountCardsParams{UserID: userID, Now: ts(out.now)})
	if err != nil {
		s.failLearn(w, fmt.Errorf("просроченные карточки: %w", err))
		return learnSetup{}, false
	}
	stats, err := s.store.LearnReviewStats(ctx, db.LearnReviewStatsParams{UserID: userID, Since: ts(out.now.AddDate(0, 0, -accuracyWindowDays))})
	if err != nil {
		s.failLearn(w, fmt.Errorf("точность повторений: %w", err))
		return learnSetup{}, false
	}
	counts2, err := s.store.LearnStatusCounts(ctx, userID)
	if err != nil {
		s.failLearn(w, fmt.Errorf("слов в учёте: %w", err))
		return learnSetup{}, false
	}
	tracked := 0
	for _, c := range counts2 {
		tracked += int(c.Count)
	}
	in := pace.Input{Tracked: tracked,
		DailyMinutes: int(st.DailyMinMinutes), Due: int(counts.Due), IntroducedToday: int(introduced),
		Mode: out.prefs.NewWordsMode, PerDay: int(out.prefs.NewWordsPerDay), Cap: int(out.prefs.NewWordsPerDay),
	}
	if stats.Total >= minAccuracySample {
		in.HasAccuracy, in.Accuracy7d = true, float64(stats.Good)/float64(stats.Total)
	}
	out.introduced = int(introduced)
	out.pace = pace.NewWordsToday(in)
	return out, true
}

func reasonView(r pace.Reason) api.LearnReason {
	return api.LearnReason{Code: api.LearnReasonCode(r.Code), Text: r.Text}
}

// GetLearnStatus — сводка по изучению и сегодняшний лимит новых слов.
func (s *authServer) GetLearnStatus(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setup, ok := s.learnContext(w, r, user.ID)
	if !ok {
		return
	}
	ctx := r.Context()
	counts, err := s.store.LearnStatusCounts(ctx, user.ID)
	if err != nil {
		s.failLearn(w, fmt.Errorf("счётчики: %w", err))
		return
	}
	out := api.LearnStatus{
		Limit: setup.pace.Limit, Remaining: setup.pace.Remaining, Reason: reasonView(setup.pace.Reason),
		Mode: api.LearnStatusMode(setup.prefs.NewWordsMode), PerDay: int(setup.prefs.NewWordsPerDay),
		NewToday: setup.introduced,
	}
	for _, c := range counts {
		switch c.Status {
		case "learned":
			out.Learned = int(c.Count)
		case "introduced":
			out.Introduced = int(c.Count)
		case "known_at_start":
			out.KnownAtStart = int(c.Count)
		}
	}
	out.TrackedWords = out.Learned + out.Introduced + out.KnownAtStart
	out.TextMinWords = textMinKnown
	out.CampWords = out.Learned + out.KnownAtStart
	out.CampMinWords = campMinWords
	out.CampOpen = out.CampWords >= campMinWords
	switch b, err := s.store.KbLatestBuild(ctx); {
	case err == nil:
		out.KbReady = b.Status == "ok"
	case !errors.Is(err, pgx.ErrNoRows):
		slog.Warn("знакомство: не удалось прочитать сборку базы знаний", "err", err)
	}
	writeJSON(w, http.StatusOK, out)
}

// teachingCard — то, что нужно из карточки слова базы знаний (см. kb.cardPayload).
type teachingCard struct {
	Translations []string `json:"translations"`
	IPA          string   `json:"ipa"`
	Audio        string   `json:"audio"`
	Forms        []string `json:"forms"`
	Synonyms     []string `json:"synonyms"`
}

func parseCard(raw json.RawMessage) (teachingCard, bool) {
	var c teachingCard
	if err := json.Unmarshal(raw, &c); err != nil || len(c.Translations) == 0 {
		return teachingCard{}, false
	}
	return c, true
}

func capList(in []string) []string {
	if len(in) > learnListCap {
		in = in[:learnListCap]
	}
	if in == nil {
		in = []string{}
	}
	return in
}

func audioURL(ref string) *string {
	if ref == "" {
		return nil
	}
	u := audioBaseURL + url.PathEscape(ref)
	return &u
}

func exampleViews(ex []exampleSentence) []api.Example {
	out := []api.Example{}
	for _, e := range ex[:min(len(ex), learnExamplesShown)] {
		out = append(out, api.Example{
			En:        api.ExampleSentence{TatoebaId: e.Row.EnTatoebaID, Text: e.Row.EnText, Author: e.Row.EnAuthor},
			Ru:        api.ExampleSentence{TatoebaId: e.Row.RuTatoebaID, Text: e.Row.RuText, Author: e.Row.RuAuthor},
			Highlight: &api.TextRange{Start: e.Start, End: e.End},
		})
	}
	return out
}

// learnLevels — уровень пользователя и следующий за ним.
func learnLevels(current string) []string {
	if i := slices.Index(allLevels, current); i >= 0 {
		return allLevels[i:min(i+2, len(allLevels))]
	}
	return allLevels[:2]
}

// GetLearnBlock собирает блок новых слов: самые частые слова уровня, которых нет в карточках и в состоянии изучения,
// не больше трёх одной части речи, без синонимов и близких написаний, только с переводом и примером.
func (s *authServer) GetLearnBlock(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setup, ok := s.learnContext(w, r, user.ID)
	if !ok {
		return
	}
	empty := api.LearnBlock{Words: []api.LearnWord{}, Limit: setup.pace.Limit, Remaining: setup.pace.Remaining, Reason: reasonView(setup.pace.Reason)}
	if setup.pace.Remaining == 0 {
		writeJSON(w, http.StatusOK, empty)
		return
	}
	ctx := r.Context()
	levels := learnLevels(setup.settings.CurrentLevel)
	cards := map[int64]teachingCard{}
	byID := map[int64]db.LearnCandidatesRow{}
	related := map[[2]int64]bool{}
	examples := map[int64][]exampleSentence{}
	var cands []learn.Candidate
	var picked []learn.Candidate
	var exErr error
	isRelated := func(a, b int64) bool { return related[[2]int64{a, b}] || related[[2]int64{b, a}] }
	for page := 0; page < learnCandidatePages; page++ {
		rows, err := s.store.LearnCandidates(ctx, db.LearnCandidatesParams{
			UserID: user.ID, Levels: levels, MaxRows: learnCandidateRows, RowOffset: int32(page * learnCandidateRows), //nolint:gosec // страниц мало
		})
		if err != nil {
			if isMissingTable(err) { // базы знаний нет: слов для знакомства нет, это не авария сервера
				slog.Warn("знакомство: таблицы базы знаний недоступны", "err", err)
				writeError(w, http.StatusNotFound, api.ErrorErrorCodeNoWords, "Слов для знакомства пока нет")
				return
			}
			s.failLearn(w, fmt.Errorf("кандидаты: %w", err))
			return
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			byID[row.LexemeID] = row
			c, ok := parseCard(row.Payload)
			if !ok {
				continue
			}
			cards[row.LexemeID] = c
			cands = append(cands, learn.Candidate{ID: row.LexemeID, Lemma: row.Lemma, Pos: row.Pos})
			ids = append(ids, row.LexemeID)
		}
		if len(ids) > 0 {
			rels, err := s.store.LearnRelations(ctx, ids)
			if err != nil {
				s.failLearn(w, fmt.Errorf("связи слов: %w", err))
				return
			}
			for _, rel := range rels {
				if rel.OtherLexemeID != nil {
					related[[2]int64{rel.LexemeID, *rel.OtherLexemeID}] = true
				}
			}
		}
		picked = learn.PickBlock(cands, min(learnBlockSize, setup.pace.Remaining), func(c learn.Candidate) bool {
			if exErr != nil {
				return false
			}
			if ex, seen := examples[c.ID]; seen {
				return len(ex) > 0
			}
			ex, err := exampleSentences(ctx, s.store, c.Lemma, c.Pos)
			if err != nil {
				exErr = err
				return false
			}
			examples[c.ID] = ex
			return len(ex) > 0
		}, isRelated)
		if exErr != nil {
			s.failLearn(w, fmt.Errorf("примеры: %w", exErr))
			return
		}
		if len(picked) >= min(learnBlockSize, setup.pace.Remaining) || len(rows) < learnCandidateRows {
			break
		}
	}
	if len(picked) == 0 {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeNoWords, "Слов для знакомства пока нет")
		return
	}
	for _, c := range picked {
		row, card := byID[c.ID], cards[c.ID]
		word := api.LearnWord{
			LexemeId: c.ID, Lemma: row.Lemma, Pos: row.Pos, Level: api.CefrLevel(row.Level),
			Translations: card.Translations[:min(len(card.Translations), 2)], AudioUrl: audioURL(card.Audio),
			Forms: capList(card.Forms), Synonyms: capList(card.Synonyms), Examples: exampleViews(examples[c.ID]),
		}
		if row.FreqRank != nil {
			rank := int(*row.FreqRank)
			word.Rank = &rank
		}
		if card.IPA != "" {
			word.Ipa = &card.IPA
		}
		empty.Words = append(empty.Words, word)
	}
	writeJSON(w, http.StatusOK, empty)
}

func decodeLearnBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxLearnBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный запрос")
		return false
	}
	return true
}

func validKey(k string) bool { return k != "" && validText(k, maxLearnKey) }

func uniquePositive(ids []int64) bool {
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

var errLimitReached = errors.New("лимит новых слов исчерпан")

// IntroduceWords вводит слова блока: создаёт карточки (FSRS «новая») и строки состояния «введено». Повтор по ключу
// возвращает прежний ответ и ничего не меняет.
func (s *authServer) IntroduceWords(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.IntroduceRequest
	if !decodeLearnBody(w, r, &req) {
		return
	}
	if len(req.LexemeIds) < 1 || len(req.LexemeIds) > learnBlockSize || !uniquePositive(req.LexemeIds) || !validKey(req.Key) {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны от 1 до 7 разных слов и key (до 64 символов)")
		return
	}
	ctx := r.Context()
	eventKey := "learn.introduce:" + req.Key
	var prev api.IntroduceResponse
	if ok, err := s.storedResult(ctx, user.ID, eventKey, &prev); err != nil {
		s.failLearn(w, err)
		return
	} else if ok {
		writeJSON(w, http.StatusOK, prev)
		return
	}
	setup, ok := s.learnContext(w, r, user.ID)
	if !ok {
		return
	}
	cardRows, err := s.store.KbGetTeachingCards(ctx, req.LexemeIds)
	if err != nil {
		s.failLearn(w, fmt.Errorf("карточки слов: %w", err))
		return
	}
	type pair struct{ lemma, pos string }
	known := map[int64]pair{}
	for _, row := range cardRows {
		var head struct{ Lemma, Pos string }
		if err := json.Unmarshal(row.Payload, &head); err == nil && head.Lemma != "" {
			known[row.LexemeID] = pair{head.Lemma, head.Pos}
		}
	}
	for _, id := range req.LexemeIds {
		p, ok := known[id]
		if !ok {
			writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Одного из слов нет в базе знаний")
			return
		}
		ex, err := exampleSentences(ctx, s.store, p.lemma, p.pos)
		if err != nil {
			s.failLearn(w, fmt.Errorf("примеры: %w", err))
			return
		}
		if len(ex) == 0 {
			writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Для одного из слов нет примеров")
			return
		}
	}

	var resp api.IntroduceResponse
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		now := setup.now
		resp = api.IntroduceResponse{CardIds: []int64{}}
		for _, id := range req.LexemeIds {
			if _, err := tx.GetCardByLexeme(ctx, db.GetCardByLexemeParams{UserID: user.ID, LexemeID: id}); err == nil {
				continue // карточка уже есть (старая или из другого места): слово не новое
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			n, err := tx.LearnInsertIntroduced(ctx, db.LearnInsertIntroducedParams{UserID: user.ID, LexemeID: id, IntroducedAt: ts(now)})
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			cardID, err := tx.InsertCard(ctx, insertParams(user.ID, id, srs.New(now)))
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			resp.CardIds = append(resp.CardIds, cardID)
			resp.Introduced++
		}
		if resp.Introduced > setup.pace.Remaining {
			return errLimitReached // транзакция откатывается вместе со вставками
		}
		payload, err := json.Marshal(map[string]any{"introduced": resp.Introduced, "cardIds": resp.CardIds, "lexemeIds": req.LexemeIds})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: user.ID, Type: "word.introduced", SchemaVersion: 1, OccurredAt: ts(now), Payload: payload, IdempotencyKey: &eventKey})
		if errors.Is(err, pgx.ErrNoRows) { // гонка двух одинаковых запросов: этот откатывается, ответ читается из события
			return errReplay
		}
		return err
	})
	switch {
	case errors.Is(err, errLimitReached):
		writeError(w, http.StatusConflict, api.ErrorErrorCodeLimitReached, "На сегодня новых слов уже достаточно")
		return
	case errors.Is(err, errReplay):
		if ok, err := s.storedResult(ctx, user.ID, eventKey, &prev); err != nil || !ok {
			s.failLearn(w, fmt.Errorf("повтор запроса: событие не найдено или нечитаемо (%v)", err))
			return
		}
		writeJSON(w, http.StatusOK, prev)
		return
	case err != nil:
		s.failLearn(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

var errReplay = errors.New("запрос уже применён")

// storedResult читает ответ прежнего запроса из события с этим ключом. ok = false — события нет.
func (s *authServer) storedResult(ctx context.Context, userID int64, key string, into any) (bool, error) {
	ev, err := s.store.GetEventByKey(ctx, db.GetEventByKeyParams{UserID: userID, IdempotencyKey: &key})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("событие по ключу: %w", err)
	}
	if err := json.Unmarshal(ev.Payload, into); err != nil {
		return false, fmt.Errorf("событие по ключу нечитаемо: %w", err)
	}
	return true, nil
}

// placementSkip читает skip из строки запроса: целое от 0 до placementMaxSkip.
func placementSkip(r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("skip")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	return n, err == nil && n >= 0 && n <= placementMaxSkip
}

// GetPlacement отдаёт страницу быстрого теста: самые частые слова уровня, которых пользователь ещё не изучал.
func (s *authServer) GetPlacement(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	skip, ok := placementSkip(r)
	if !ok {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "skip — целое число от 0 до 2000")
		return
	}
	st, err := s.store.GetSettings(r.Context(), user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, api.ErrorErrorCodeOnboardingRequired, "Сначала настрой цель и уровень")
		return
	}
	if err != nil {
		s.failLearn(w, fmt.Errorf("настройки: %w", err))
		return
	}
	rows, err := s.store.LearnPlacementCandidates(r.Context(), db.LearnPlacementCandidatesParams{
		UserID: user.ID, Levels: learnLevels(st.CurrentLevel), RowOffset: int32(skip), MaxRows: placementPageSize + 1, //nolint:gosec // skip не больше 2000
	})
	if err != nil {
		s.failLearn(w, fmt.Errorf("слова теста: %w", err))
		return
	}
	page := api.PlacementPage{Words: []api.PlacementWord{}, Skip: skip, HasMore: len(rows) > placementPageSize}
	for _, row := range rows[:min(len(rows), placementPageSize)] {
		page.Words = append(page.Words, api.PlacementWord{LexemeId: row.LexemeID, Lemma: row.Lemma, Pos: row.Pos})
	}
	writeJSON(w, http.StatusOK, page)
}

func placementNext(known, shown int) api.PlacementResultNext {
	if shown == 0 {
		return api.Stop
	}
	share := float64(known) / float64(shown)
	switch {
	case share >= placementMoreShare:
		return api.More
	case share < placementStopShare:
		return api.Stop
	default:
		return api.Learn
	}
}

// SubmitPlacement принимает ответы быстрого теста: отмеченные «знаю» слова получают состояние known_at_start
// (карточек у них нет). Остальные показанные слова ничем не отмечаются.
func (s *authServer) SubmitPlacement(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.PlacementRequest
	if !decodeLearnBody(w, r, &req) {
		return
	}
	bad := len(req.Shown) > placementPageSize || !uniquePositive(req.Shown) || !uniquePositive(req.Known) || !validKey(req.Key)
	for _, id := range req.Known {
		bad = bad || !slices.Contains(req.Shown, id)
	}
	if bad {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Нужны shown (до 50 разных слов), known из них и key (до 64 символов)")
		return
	}
	ctx := r.Context()
	if len(req.Shown) > 0 {
		levels, err := s.store.ListLexemeLevels(ctx, req.Shown)
		if err != nil {
			s.failLearn(w, fmt.Errorf("слова теста: %w", err))
			return
		}
		have := map[int64]bool{}
		for _, l := range levels {
			have[l.LexemeID] = true
		}
		for _, id := range req.Shown {
			if !have[id] {
				writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Одного из слов нет в словаре")
				return
			}
		}
	}
	eventKey := "learn.placement:" + req.Key
	var res api.PlacementResult
	if ok, err := s.storedResult(ctx, user.ID, eventKey, &res); err != nil {
		s.failLearn(w, err)
		return
	} else if ok {
		writeJSON(w, http.StatusOK, res)
		return
	}
	res = api.PlacementResult{Next: placementNext(len(req.Known), len(req.Shown))}
	err = s.store.InTx(ctx, func(tx AuthStore) error {
		res.Marked = 0
		for _, id := range req.Known {
			n, err := tx.LearnMarkKnown(ctx, db.LearnMarkKnownParams{UserID: user.ID, LexemeID: id})
			if err != nil {
				return err
			}
			res.Marked += int(n)
		}
		if len(req.Shown) == 0 {
			return nil
		}
		payload, err := json.Marshal(map[string]any{"marked": res.Marked, "next": res.Next, "known": req.Known, "shown": len(req.Shown)})
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(ctx, db.AppendEventParams{UserID: user.ID, Type: "word.known_at_start", SchemaVersion: 1, OccurredAt: ts(s.now()), Payload: payload, IdempotencyKey: &eventKey})
		if errors.Is(err, pgx.ErrNoRows) {
			return errReplay
		}
		return err
	})
	if errors.Is(err, errReplay) {
		if ok, err := s.storedResult(ctx, user.ID, eventKey, &res); err != nil || !ok {
			s.failLearn(w, fmt.Errorf("повтор запроса: событие не найдено или нечитаемо (%v)", err))
			return
		}
	} else if err != nil {
		s.failLearn(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func prefsView(p db.LearnPreference) api.LearnPreferences {
	return api.LearnPreferences{NewWordsMode: api.LearnPreferencesNewWordsMode(p.NewWordsMode), NewWordsPerDay: int(p.NewWordsPerDay), CampCoverage: p.CampCoverage}
}

// GetLearnPreferences отдаёт настройки обучения (по умолчанию: авто, 12 слов в день, покрытие 95 %).
func (s *authServer) GetLearnPreferences(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	p, err := s.learnPrefs(r.Context(), user.ID)
	if err != nil {
		s.failLearn(w, fmt.Errorf("предпочтения: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, prefsView(p))
}

// PutLearnPreferences сохраняет настройки обучения.
func (s *authServer) PutLearnPreferences(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var req api.LearnPreferences
	if !decodeLearnBody(w, r, &req) {
		return
	}
	mode := string(req.NewWordsMode)
	const eps = 1e-6
	switch {
	case mode != "auto" && mode != "manual":
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Режим — auto или manual")
		return
	case req.NewWordsPerDay < 1 || req.NewWordsPerDay > maxPerDay:
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Слов в день — от 1 до 30")
		return
	case req.CampCoverage < minCoverage-eps || req.CampCoverage > maxCoverage+eps:
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Покрытие — от 0,90 до 0,99")
		return
	}
	p, err := s.store.LearnUpsertPreferences(r.Context(), db.LearnUpsertPreferencesParams{
		UserID: user.ID, NewWordsMode: mode, NewWordsPerDay: int32(req.NewWordsPerDay), CampCoverage: req.CampCoverage, //nolint:gosec // 1–30
	})
	if err != nil {
		s.failLearn(w, fmt.Errorf("сохранение предпочтений: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, prefsView(p))
}

// markLearned переводит слово в «изучено» после хорошего вспоминания и пишет событие word.learned (один раз на слово).
func markLearned(ctx context.Context, tx AuthStore, userID, lexemeID int64, now time.Time) error {
	n, err := tx.LearnMarkLearned(ctx, db.LearnMarkLearnedParams{UserID: userID, LexemeID: lexemeID, Now: ts(now)})
	if err != nil || n == 0 {
		return err
	}
	payload, err := json.Marshal(map[string]any{"lexemeId": lexemeID})
	if err != nil {
		return err
	}
	key := fmt.Sprintf("word.learned:%d", lexemeID)
	if _, err := tx.AppendEvent(ctx, db.AppendEventParams{UserID: userID, Type: "word.learned", SchemaVersion: 1, OccurredAt: ts(now), Payload: payload, IdempotencyKey: &key}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
