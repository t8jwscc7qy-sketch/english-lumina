package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/rating"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/testdb"
)

// Весь путь на настоящей базе: настоящие запросы sqlc, настоящий пароль и токен.
func TestAuthFlowOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}

	// Вход.
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("вход: статус %d, тело %s", rec.Code, rec.Body)
	}
	token := sessionFrom(t, rec).Value

	// В базе хеш токена, а не сам токен.
	var count, hashLen int
	if err := pool.QueryRow(ctx, "SELECT count(*), coalesce(max(octet_length(token_hash)), 0) FROM sessions").Scan(&count, &hashLen); err != nil {
		t.Fatal(err)
	}
	if count != 1 || hashLen != 32 {
		t.Fatalf("сессий %d, длина хеша %d; ожидали 1 и 32", count, hashLen)
	}
	var tokenStored bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM sessions WHERE token_hash = $1)", []byte(token)).Scan(&tokenStored); err != nil || tokenStored {
		t.Fatalf("сырой токен найден в таблице sessions (err=%v)", err)
	}

	// Текущий пользователь.
	if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), testLogin) {
		t.Fatalf("/me: статус %d, тело %s", rec.Code, rec.Body)
	}

	// Выход отзывает сессию в базе.
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", "", withCookie(token)); rec.Code != http.StatusNoContent {
		t.Fatalf("выход: статус %d", rec.Code)
	}
	var revoked bool
	if err := pool.QueryRow(ctx, "SELECT bool_and(revoked_at IS NOT NULL) FROM sessions").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("сессия не отозвана (err=%v)", err)
	}
	if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("/me после выхода: статус %d, ожидали 401", rec.Code)
	}

	// Неверный пароль и блокировка после серии неудач.
	for i := 0; i < 5; i++ {
		if rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1")); rec.Code != http.StatusUnauthorized {
			t.Fatalf("неверный пароль %d: статус %d, ожидали 401", i+1, rec.Code)
		}
	}
	if rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("после 5 неудач: статус %d, ожидали 429", rec.Code)
	}
}

// Настройки на настоящей базе: upsert sqlc, тип date и CHECK таблицы settings.
func TestSettingsFlowOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)

	if rec := e.do(http.MethodGet, settingsPath, "", withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("GET до онбординга: статус %d, ожидали 404", rec.Code)
	}

	date := time.Now().AddDate(0, 8, 0).Format(dateLayout)
	body := settingsBody(t, func(m map[string]any) { m["targetDate"] = date })
	rec := e.do(http.MethodPut, settingsPath, body, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: статус %d, тело %s", rec.Code, rec.Body)
	}
	got := e.do(http.MethodGet, settingsPath, "", withCookie(token))
	if got.Code != http.StatusOK || got.Body.String() != rec.Body.String() {
		t.Fatalf("GET: статус %d, тело %s; ожидали %s", got.Code, got.Body, rec.Body)
	}

	// Повторный PUT заменяет строку, а не добавляет вторую.
	body = settingsBody(t, func(m map[string]any) { m["dailyMinutes"] = 30; m["targetDate"] = nil })
	if rec := e.do(http.MethodPut, settingsPath, body, withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("второй PUT: статус %d, тело %s", rec.Code, rec.Body)
	}
	var rows, minutes int
	var hasDate bool
	if err := pool.QueryRow(ctx, "SELECT count(*), max(daily_min_minutes), bool_or(target_date IS NOT NULL) FROM settings").Scan(&rows, &minutes, &hasDate); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || minutes != 30 || hasDate {
		t.Fatalf("строк %d, минут %d, дата есть %v; ожидали 1, 30, false", rows, minutes, hasDate)
	}

	// Цель ниже уровня отсекается до базы и ничего не меняет.
	bad := settingsBody(t, func(m map[string]any) { m["currentLevel"] = "C1"; m["targetLevel"] = "A1" })
	if rec := e.do(http.MethodPut, settingsPath, bad, withCookie(token)); rec.Code != http.StatusBadRequest {
		t.Fatalf("цель ниже уровня: статус %d, ожидали 400", rec.Code)
	}
}

// Список источников на настоящей базе: недогруженный источник (без imported_at) в ответ не попадает.
func TestSourcesOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	done, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "done", Version: "1", License: "l", Url: "https://example.com/d", Attribution: "attr done"})
	if err != nil {
		t.Fatal(err)
	}
	n := int32(5)
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: done.ID, RowCount: &n}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "half", Version: "1", License: "l", Url: "https://example.com/h", Attribution: "attr half"}); err != nil {
		t.Fatal(err)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rec := e.do(http.MethodGet, "/api/v1/sources", "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"attribution":"attr done"`) || !strings.Contains(body, `"rowCount":5`) || strings.Contains(body, "half") {
		t.Fatalf("тело %s: ожидали только завершённый источник", body)
	}
}

// Поиск слов на настоящей базе: порядок, регистр, варианты через слэш, спецсимволы, лимит, общий уровень слова.
func TestWordsOnRealDatabase(t *testing.T) {
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
	src := func(name string) int64 {
		ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: name, Version: "1", License: "l", Url: "https://example.com/" + name, Attribution: "a"})
		if err != nil {
			t.Fatal(err)
		}
		return ds.ID
	}
	cefrj, octanove := src("cefrj"), src("octanove")
	add := func(source int64, lemma, pos, level, extra string) {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: pos, SourceID: source})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: source, Level: level, Extra: []byte(extra)}); err != nil {
			t.Fatal(err)
		}
	}
	add(cefrj, "run", "verb", "A1", `{}`)
	add(cefrj, "runway", "noun", "B2", `{}`)
	add(cefrj, "runner", "noun", "B1", `{}`)
	add(cefrj, "arun", "noun", "B1", `{}`) // содержит «run», но не начинается с него
	add(cefrj, "cafe/café", "noun", "A1", `{}`)
	add(cefrj, "complexity", "noun", "B2", `{}`)
	add(octanove, "complexity", "noun", "C1", `{"notes":["сложность"]}`)
	add(cefrj, "50%", "noun", "A2", `{}`)
	for i := 0; i < 60; i++ {
		add(cefrj, "lim"+string(rune('a'+i/26))+string(rune('a'+i%26)), "noun", "A1", `{}`)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	search := func(query string) api.WordList {
		t.Helper()
		rec := e.do(http.MethodGet, wordsPath(query), "", withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("q=%q: статус %d, тело %s", query, rec.Code, rec.Body)
		}
		return decodeWords(t, rec.Body.String())
	}
	lemmas := func(l api.WordList) string {
		var out []string
		for _, w := range l.Words {
			out = append(out, w.Lemma)
		}
		return strings.Join(out, ",")
	}

	// Точное совпадение первым, затем по началу слова по алфавиту; «arun» (подстрока) не находится.
	if got := lemmas(search("run")); got != "run,runner,runway" {
		t.Fatalf("q=run: %s", got)
	}
	if got := lemmas(search("RUN")); got != "run,runner,runway" {
		t.Fatalf("q=RUN (регистр): %s", got)
	}
	// Вариант через слэш находится по началу любого варианта.
	for _, qq := range []string{"cafe", "café", "CAFÉ"} {
		if got := lemmas(search(qq)); got != "cafe/café" {
			t.Fatalf("q=%s: %s", qq, got)
		}
	}
	// Спецсимволы LIKE ищутся как обычные знаки, а не как маски.
	if got := lemmas(search("%")); got != "" {
		t.Fatalf("q=%%: нашлось %q, ожидали пусто (знак процента в начале слова нет)", got)
	}
	if got := lemmas(search("_")); got != "" {
		t.Fatalf("q=_: нашлось %q", got)
	}
	if got := lemmas(search("50%")); got != "50%" {
		t.Fatalf("q=50%%: %q", got)
	}
	// Не больше 50 результатов.
	if n := len(search("lim").Words); n != 50 {
		t.Fatalf("q=lim: %d результатов, ожидали 50", n)
	}
	// Общее слово: уровень самый низкий, оценки по источникам по имени, заметки доехали.
	w := search("complexity").Words
	if len(w) != 1 || w[0].Level != api.CefrLevelB2 || len(w[0].Levels) != 2 {
		t.Fatalf("complexity: %+v", w)
	}
	if w[0].Levels[0].Source != "cefrj" || w[0].Levels[1].Source != "octanove" || w[0].Levels[1].Level != api.CefrLevelC1 ||
		strings.Join(w[0].Levels[1].Notes, "|") != "сложность" {
		t.Fatalf("оценки: %+v", w[0].Levels)
	}
}

// Примеры на настоящей базе: формы слова, отбор по длине, ближе к 8 словам, лучший перевод, подсветка, лимит, безопасность.
func TestExamplesOnRealDatabase(t *testing.T) {
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
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	sentence := func(lang string, id int64, text, author string) int64 {
		row, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{
			Lang: lang, TatoebaID: id, Text: text, Author: author, WordCount: int16(len(strings.Fields(text))), SourceID: ds.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		return row.ID
	}
	link := func(enID, ruID int64) {
		if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: enID, RuID: ruID, SourceID: ds.ID}); err != nil {
			t.Fatal(err)
		}
	}
	// id Tatoeba ниже растут вместе с номером примера; длина в словах указана в комментарии.
	add := func(id int64, en string) int64 {
		e := sentence("en", id, en, "CK")
		link(e, sentence("ru", id+1000, "Перевод "+en, "marafon"))
		return e
	}
	add(1, "Run.")                                                                                 // 1 слово: слишком коротко
	add(2, "We run.")                                                                              // 2
	add(3, "She ran home quickly yesterday.")                                                      // 5
	add(4, "They were running along the river.")                                                   // 6
	add(5, "He runs a small shop in town.")                                                        // 7
	add(6, "I like to run every morning before work.")                                             // 8: ближе всех к 8
	add(7, "I like to run every single morning before work today.")                                // 10
	add(8, "Running in the park every day makes me feel very good now.")                           // 12
	add(9, "I like to run every single morning before work today and then go home to rest a bit.") // 17: слишком длинно
	add(10, "He is a runner and he likes it very much.")                                           // 10, но «runner» — другое слово
	add(11, "A big runaway truck hit the wall near here.")                                         // 9, «runaway» — другое слово
	// Предложение без перевода в примеры не попадает.
	sentence("en", 12, "I like to run with my best friend.", "CK")
	// Перевод выбирается самый короткий.
	e13 := sentence("en", 13, "Do you run in the park on Sundays?", "CM") // 8 слов
	link(e13, sentence("ru", 2013, "Ты бегаешь в парке по воскресеньям, когда погода хорошая?", "a"))
	link(e13, sentence("ru", 2014, "Ты бегаешь в парке?", "b"))

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	get := func(lemma, pos string) api.ExampleList {
		t.Helper()
		rec := e.do(http.MethodGet, examplesPath(lemma, pos), "", withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s/%s: статус %d, тело %s", lemma, pos, rec.Code, rec.Body)
		}
		var list api.ExampleList
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		return list
	}
	ids := func(l api.ExampleList) []int64 {
		var out []int64
		for _, ex := range l.Examples {
			out = append(out, ex.En.TatoebaId)
		}
		return out
	}

	// Глагол run: формы run/runs/running/ran; 4–12 слов; ближе к 8, при равенстве по номеру; не больше 5.
	// Расстояния до 8: №6 → 0, №13 → 0, №5 → 1, №4 → 2, №7 → 2, №3 → 3, №8 → 4.
	got := get("run", "verb")
	if fmt.Sprint(ids(got)) != "[6 13 5 4 7]" {
		t.Fatalf("примеры: %v, ожидали [6 13 5 4 7]", ids(got))
	}
	first := got.Examples[0]
	if first.Ru.Text != "Перевод I like to run every morning before work." || first.En.Author != "CK" || first.Ru.Author != "marafon" || first.Ru.TatoebaId != 1006 {
		t.Fatalf("первый пример: %+v", first)
	}
	if first.Highlight == nil || first.Highlight.Start != 10 || first.Highlight.End != 13 {
		t.Fatalf("подсветка: %+v", first.Highlight)
	}
	// Самый короткий из переводов.
	if second := got.Examples[1]; second.Ru.Text != "Ты бегаешь в парке?" || second.Ru.TatoebaId != 2014 {
		t.Fatalf("перевод должен быть самый короткий: %+v", second.Ru)
	}
	// Подсветка находит форму, а не лемму.
	if h := got.Examples[3].Highlight; h == nil || h.Start != 10 || h.End != 17 { // «They were running…»
		t.Fatalf("подсветка running: %+v", h)
	}

	// У существительного формы только run и runs: «ran» и «running» не подходят.
	if noun := ids(get("run", "noun")); fmt.Sprint(noun) != "[6 13 5 7]" {
		t.Fatalf("run/noun: %v, ожидали [6 13 5 7]", noun)
	}
	// Неизвестное слово и слово без предложений.
	if n := len(get("zebra", "noun").Examples); n != 0 {
		t.Fatalf("zebra: %d примеров", n)
	}
	// Синтаксис tsquery в лемме безопасен.
	if n := len(get("run' | 'x & !y:*", "noun").Examples); n != 0 {
		t.Fatalf("лемма со знаками синтаксиса: %d примеров", n)
	}
	// Фраза ищется как слова подряд: «like to» есть в №6 и №7 (№9 длинный, №12 без перевода, «likes» — другая форма).
	if phrase := ids(get("like to", "verb")); fmt.Sprint(phrase) != "[6 7]" {
		t.Fatalf("фраза like to: %v, ожидали [6 7]", phrase)
	}
}

// Добавление карточки на настоящей базе: состояние, событие, повтор без дублей, слово без примеров.
func TestAddCardOnRealDatabase(t *testing.T) {
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
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range [][2]string{{"run", "verb"}, {"zebra", "noun"}} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w[0], Pos: w[1], SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A2", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	// Слово без строк уровня: его нельзя добавить, иначе очередь падала бы на NULL.
	if _, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: "walk", Pos: "verb", SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}
	en, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I like to run every morning before work.", Author: "CK", WordCount: 8, SourceID: ds.ID})
	if err != nil {
		t.Fatal(err)
	}
	ru, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "ru", TatoebaID: 2, Text: "Я люблю бегать по утрам перед работой.", Author: "a", WordCount: 6, SourceID: ds.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en.ID, RuID: ru.ID, SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	add := func(lemma, pos string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/v1/cards", addBody(lemma, pos), withCookie(token))
	}
	count := func(query string) int {
		var n int
		if err := pool.QueryRow(ctx, query).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	before := time.Now().Add(-time.Second)
	rec := add("run", "verb")
	if rec.Code != http.StatusCreated {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	created := decodeAdd(t, rec.Body.String())
	var state, reps int
	var due time.Time
	var lastReviewNull bool
	err = pool.QueryRow(ctx, `SELECT state, reps, due, last_review IS NULL FROM cards WHERE id = $1`, created.CardId).Scan(&state, &reps, &due, &lastReviewNull)
	if err != nil || state != 0 || reps != 0 || !lastReviewNull || due.Before(before) || due.After(time.Now().Add(time.Second)) {
		t.Fatalf("карточка: state=%d reps=%d due=%v lastReviewNull=%v (%v)", state, reps, due, lastReviewNull, err)
	}
	var payload string
	if err := pool.QueryRow(ctx, `SELECT payload::text FROM events WHERE type = 'card.added'`).Scan(&payload); err != nil ||
		!strings.Contains(payload, `"lemma": "run"`) || !strings.Contains(payload, `"pos": "verb"`) {
		t.Fatalf("событие card.added: %s (%v)", payload, err)
	}

	again := add("run", "verb")
	if again.Code != http.StatusOK || decodeAdd(t, again.Body.String()).Created {
		t.Fatalf("повтор: статус %d, тело %s", again.Code, again.Body)
	}
	if count(`SELECT count(*) FROM cards`) != 1 || count(`SELECT count(*) FROM events WHERE type = 'card.added'`) != 1 {
		t.Fatalf("после повтора карточек %d, событий %d; ожидали по одному", count(`SELECT count(*) FROM cards`), count(`SELECT count(*) FROM events`))
	}

	if rec := add("zebra", "noun"); rec.Code != http.StatusConflict || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeNoExamples {
		t.Fatalf("слово без примеров: статус %d, тело %s", rec.Code, rec.Body)
	}
	if rec := add("nonexistent", "noun"); rec.Code != http.StatusNotFound {
		t.Fatalf("неизвестное слово: статус %d", rec.Code)
	}
	if rec := add("walk", "verb"); rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != api.ErrorErrorCodeWordNotFound {
		t.Fatalf("слово без уровня: статус %d, тело %s", rec.Code, rec.Body)
	}
	if count(`SELECT count(*) FROM cards`) != 1 {
		t.Fatal("отказы создали карточки")
	}
	// Очередь с добавленной карточкой отвечает (раньше этот путь не проверялся).
	qrec := e.do(http.MethodGet, "/api/v1/cards/queue", "", withCookie(token))
	if qrec.Code != http.StatusOK || decodeQueue(t, qrec.Body.String()).Card == nil || decodeQueue(t, qrec.Body.String()).Card.Level != api.CefrLevelA2 {
		t.Fatalf("очередь после добавления: %d %s", qrec.Code, qrec.Body)
	}
}

// Оценка карточек на настоящей базе: очередь, переходы состояний, события по порядку, повтор по ключу.
func TestReviewCardsOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	lexeme := func(lemma, pos, level string) int64 {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: lemma, Pos: pos, SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: level, Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	runID, goID := lexeme("run", "verb", "A2"), lexeme("go", "verb", "A1")
	en, _ := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I like to run every morning before work.", Author: "CK", WordCount: 8, SourceID: ds.ID})
	ru, _ := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "ru", TatoebaID: 2, Text: "Я люблю бегать по утрам перед работой.", Author: "a", WordCount: 6, SourceID: ds.ID})
	if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en.ID, RuID: ru.ID, SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rows := func(query string, args ...any) int {
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// run добавляется через API, go (без предложений) кладём напрямую со сроком раньше.
	if rec := e.do(http.MethodPost, "/api/v1/cards", addBody("run", "verb"), withCookie(token)); rec.Code != http.StatusCreated {
		t.Fatalf("добавление run: %d %s", rec.Code, rec.Body)
	}
	goCard, err := q.InsertCard(ctx, db.InsertCardParams{UserID: user.ID, LexemeID: goID, Due: ts(time.Now().Add(-48 * time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	var runCard int64
	if err := pool.QueryRow(ctx, `SELECT id FROM cards WHERE lexeme_id = $1`, runID).Scan(&runCard); err != nil {
		t.Fatal(err)
	}

	queue := func() api.CardQueue {
		t.Helper()
		rec := e.do(http.MethodGet, "/api/v1/cards/queue", "", withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("очередь: %d %s", rec.Code, rec.Body)
		}
		return decodeQueue(t, rec.Body.String())
	}
	first := queue()
	if first.DueCount != 2 || first.TotalCount != 2 || first.Card == nil || first.Card.CardId != goCard || first.Card.Sentence != nil || first.Card.Level != api.CefrLevelA1 {
		t.Fatalf("очередь: %+v (ожидали go первой, без предложения, уровень A1)", first)
	}

	// Оцениваем go; в ответе следующая карточка — run с предложением и пропуском.
	rec := doReview(e, token, goCard, 3, "g1")
	if rec.Code != http.StatusOK {
		t.Fatalf("оценка go: %d %s", rec.Code, rec.Body)
	}
	next := decodeQueue(t, rec.Body.String())
	if next.Card == nil || next.Card.CardId != runCard || next.Card.Sentence == nil || next.Card.Sentence.Gap.Start != 10 || next.Card.Sentence.Gap.End != 13 || next.Card.Level != api.CefrLevelA2 {
		t.Fatalf("следующая после go: %+v", next.Card)
	}
	type cardRow struct {
		State, Reps, Lapses, Days int
		Due                       time.Time
	}
	read := func(id int64) cardRow {
		var c cardRow
		if err := pool.QueryRow(ctx, `SELECT state, reps, lapses, scheduled_days, due FROM cards WHERE id = $1`, id).Scan(&c.State, &c.Reps, &c.Lapses, &c.Days, &c.Due); err != nil {
			t.Fatal(err)
		}
		return c
	}
	afterGo := read(goCard)
	if afterGo.Reps != 1 || afterGo.State == 0 || !afterGo.Due.After(time.Now()) {
		t.Fatalf("go после оценки: %+v", afterGo)
	}

	// Повтор с тем же ключом ничего не меняет.
	eventsBefore := rows(`SELECT count(*) FROM events WHERE type = 'card.reviewed'`)
	if rec := doReview(e, token, goCard, 1, "g1"); rec.Code != http.StatusOK || read(goCard) != afterGo ||
		rows(`SELECT count(*) FROM events WHERE type = 'card.reviewed'`) != eventsBefore {
		t.Fatalf("повтор по ключу изменил данные: %+v, событий %d", read(goCard), rows(`SELECT count(*) FROM events WHERE type = 'card.reviewed'`))
	}

	// Цикл «хорошо» по run: срок каждый раз сдвигаем в прошлое, как будто время прошло.
	var prevDays int
	for i := 0; i < 5; i++ {
		if _, err := pool.Exec(ctx, `UPDATE cards SET due = now() - interval '1 minute' WHERE id = $1`, runCard); err != nil {
			t.Fatal(err)
		}
		if rec := doReview(e, token, runCard, 3, fmt.Sprintf("r%d", i)); rec.Code != http.StatusOK {
			t.Fatalf("оценка run №%d: %d %s", i, rec.Code, rec.Body)
		}
		c := read(runCard)
		if c.Reps != i+1 || c.Days < prevDays || !c.Due.After(time.Now()) {
			t.Fatalf("после оценки №%d: %+v (дни раньше %d)", i, c, prevDays)
		}
		prevDays = c.Days
	}
	if c := read(runCard); c.State != 2 {
		t.Fatalf("после нескольких «хорошо» карточка должна быть зрелой (2): %+v", c)
	}

	// «Забыл» на зрелой карточке: ошибка и переобучение.
	if _, err := pool.Exec(ctx, `UPDATE cards SET due = now() - interval '1 minute' WHERE id = $1`, runCard); err != nil {
		t.Fatal(err)
	}
	if rec := doReview(e, token, runCard, 1, "lapse"); rec.Code != http.StatusOK {
		t.Fatalf("оценка «забыл»: %d", rec.Code)
	}
	if c := read(runCard); c.Lapses != 1 || c.State != 3 {
		t.Fatalf("после «забыл»: %+v", c)
	}

	// События: по одному на каждую применённую оценку, по порядку, состояния сцеплены.
	evRows, err := pool.Query(ctx, `SELECT payload->>'rating', payload->>'stateBefore', payload->>'stateAfter', payload->>'cardId' FROM events WHERE type = 'card.reviewed' AND (payload->>'cardId')::bigint = $1 ORDER BY id`, runCard)
	if err != nil {
		t.Fatal(err)
	}
	defer evRows.Close()
	var ratings []string
	prevAfter := "0"
	for evRows.Next() {
		var rating, before, after, id string
		if err := evRows.Scan(&rating, &before, &after, &id); err != nil {
			t.Fatal(err)
		}
		if before != prevAfter {
			t.Fatalf("состояния событий не сцеплены: до %s, ожидали %s", before, prevAfter)
		}
		prevAfter = after
		ratings = append(ratings, rating)
	}
	if fmt.Sprint(ratings) != "[3 3 3 3 3 1]" {
		t.Fatalf("оценки в журнале: %v, ожидали [3 3 3 3 3 1]", ratings)
	}
	if n := rows(`SELECT count(*) FROM events WHERE type = 'card.reviewed'`); n != 7 {
		t.Fatalf("всего событий оценки %d, ожидали 7 (go + 6 для run)", n)
	}

	// Чужая или несуществующая карточка.
	if rec := doReview(e, token, 999999, 3, "x"); rec.Code != http.StatusNotFound {
		t.Fatalf("несуществующая карточка: %d", rec.Code)
	}
	// Очередь пуста: обе карточки ушли в будущее.
	if final := queue(); final.DueCount != 0 || final.TotalCount != 2 || final.Card != nil {
		t.Fatalf("итоговая очередь: %+v", final)
	}
}

// Признак inCards в поиске на настоящей базе: появляется после добавления карточки.
func TestWordsInCardsOnRealDatabase(t *testing.T) {
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
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"run", "runner"} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w, Pos: "verb", SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	en, _ := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I like to run every morning before work.", Author: "CK", WordCount: 8, SourceID: ds.ID})
	ru, _ := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "ru", TatoebaID: 2, Text: "Я люблю бегать по утрам.", Author: "a", WordCount: 5, SourceID: ds.ID})
	if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en.ID, RuID: ru.ID, SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	inCards := func() map[string]bool {
		t.Helper()
		rec := e.do(http.MethodGet, wordsPath("run"), "", withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("поиск: %d %s", rec.Code, rec.Body)
		}
		got := map[string]bool{}
		for _, w := range decodeWords(t, rec.Body.String()).Words {
			got[w.Lemma] = w.InCards
		}
		return got
	}
	if got := inCards(); got["run"] || got["runner"] {
		t.Fatalf("до добавления: %v", got)
	}
	if rec := e.do(http.MethodPost, "/api/v1/cards", addBody("run", "verb"), withCookie(token)); rec.Code != http.StatusCreated {
		t.Fatalf("добавление: %d %s", rec.Code, rec.Body)
	}
	if got := inCards(); !got["run"] || got["runner"] {
		t.Fatalf("после добавления: %v, ожидали только run", got)
	}
}

// Частота слова в поиске на настоящей базе: лучшее место среди форм, версия источника — последняя завершённая.
func TestWordsFrequencyOnRealDatabase(t *testing.T) {
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
	cefrj, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "1", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"run", "runway"} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w, Pos: "verb", SourceID: cefrj.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: cefrj.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	freq := func(version string, finished bool, words map[string]int32) {
		ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "frequencywords", Version: version, License: "l", Url: "u", Attribution: "a"})
		if err != nil {
			t.Fatal(err)
		}
		for w, rank := range words {
			if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: ds.ID, Word: w, Count: 100, Rank: rank}); err != nil {
				t.Fatal(err)
			}
		}
		if finished {
			n := int32(len(words))
			if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: ds.ID, RowCount: &n}); err != nil {
				t.Fatal(err)
			}
		}
	}
	freq("2018", true, map[string]int32{"run": 17, "running": 5, "ran": 66})
	freq("2020", false, map[string]int32{"run": 1}) // недогруженная версия не учитывается

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rec := e.do(http.MethodGet, wordsPath("run"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("поиск: %d %s", rec.Code, rec.Body)
	}
	got := wordsByLemma(t, rec.Body.String())
	if got["run"].FrequencyRank == nil || *got["run"].FrequencyRank != 5 {
		t.Fatalf("run: %+v, ожидали место 5 (лучшая из форм в завершённой версии)", got["run"].FrequencyRank)
	}
	if got["runway"].FrequencyRank != nil {
		t.Fatalf("runway не в списке: %+v", got["runway"].FrequencyRank)
	}
}

// Выдача и ответ на упражнение на настоящей базе: повтор по ключу, 409 с другим ключом и два параллельных ответа.
func TestExerciseAnswerOnRealDatabase(t *testing.T) {
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
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"i", "have", "a", "big", "dog"} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w, Pos: "noun", SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	en, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I have a big dog.", Author: "CK", WordCount: 5, SourceID: ds.ID})
	if err != nil {
		t.Fatal(err)
	}
	ru, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "ru", TatoebaID: 2, Text: "У меня большая собака.", Author: "a", WordCount: 3, SourceID: ds.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en.ID, RuID: ru.ID, SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)

	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("next: статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Topic != "grammar:articles" {
		t.Fatalf("упражнение: %+v", ex)
	}

	// Неверный ответ: рейтинги упали, событие одно.
	first := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "the", "k1"), withCookie(token))
	if first.Code != http.StatusOK {
		t.Fatalf("ответ: статус %d, тело %s", first.Code, first.Body)
	}
	res := decodeResult(t, first)
	if res.Correct || res.Answer != "a" || len(res.Ratings) == 0 || res.Ratings[0].Rating >= 1500 || res.Ratings[0].Attempts != 1 {
		t.Fatalf("результат: %+v", res)
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	events := func() int { return count("SELECT count(*) FROM events WHERE type = 'exercise.answered'") }
	attempts := func() int {
		return count("SELECT coalesce(max(attempts), 0) FROM topic_ratings WHERE topic = 'grammar:articles'")
	}

	// Повтор по ключу: тот же ответ, ничего не меняется.
	again := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "the", "k1"), withCookie(token))
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() || events() != 1 || attempts() != 1 {
		t.Fatalf("повтор: статус %d, тело %s; событий %d, попыток %d", again.Code, again.Body, events(), attempts())
	}
	// Другой ключ: 409.
	if rec := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "a", "k2"), withCookie(token)); rec.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d, тело %s", rec.Code, rec.Body)
	}

	// Два параллельных ответа с разными ключами на новое упражнение: один 200, один 409.
	ex2 := seedRealExercise(t, q, en.ID, "grammar:articles", "level:B1")
	codes := make(chan int, 2)
	for _, key := range []string{"p1", "p2"} {
		go func() {
			codes <- e.do(http.MethodPost, answerPath, answerBody(ex2, "a", key), withCookie(token)).Code
		}()
	}
	got := []int{<-codes, <-codes}
	if got[0]+got[1] != 200+409 || got[0] == got[1] {
		t.Fatalf("параллельные ответы: коды %v, ждали один 200 и один 409", got)
	}
	if n := count("SELECT count(*) FROM events WHERE type = 'exercise.answered' AND payload->>'exerciseId' = $1", fmt.Sprint(ex2)); n != 1 {
		t.Fatalf("событий по второму упражнению %d, ждали 1", n)
	}
	if n := count("SELECT attempts FROM topic_ratings WHERE topic = 'level:B1'"); n != 1 {
		t.Fatalf("попыток по level:B1 %d, ждали 1", n)
	}

	// Два параллельных ответа на РАЗНЫЕ упражнения с общей новой темой: оба 200, обновления не теряются.
	a := seedRealExercise(t, q, en.ID, "grammar:modals", "level:C1")
	b := seedRealExercise(t, q, en.ID, "grammar:modals", "level:C1")
	var start, wg sync.WaitGroup
	start.Add(1)
	codes2 := make([]int, 2)
	for i, id := range []int64{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start.Wait()
			codes2[i] = e.do(http.MethodPost, answerPath, answerBody(id, "a", fmt.Sprintf("q%d", i)), withCookie(token)).Code
		}()
	}
	start.Done()
	wg.Wait()
	if codes2[0] != 200 || codes2[1] != 200 {
		t.Fatalf("общая тема: коды %v, ждали 200 и 200", codes2)
	}
	var mu float64
	var n int
	if err := pool.QueryRow(ctx, "SELECT rating, attempts FROM topic_ratings WHERE topic = 'level:C1'").Scan(&mu, &n); err != nil {
		t.Fatal(err)
	}
	opp := rating.Rating{Mu: 1600, Phi: 100, Sigma: 0.06}
	want := rating.Update(rating.Update(rating.New(), opp, 1), opp, 1) // оба обновления одинаковы, порядок не важен
	if n != 2 || math.Abs(mu-want.Mu) > 1e-9 {
		t.Fatalf("level:C1: попыток %d, рейтинг %.6f; ждали 2 и %.6f", n, mu, want.Mu)
	}
}

// seedRealExercise вставляет неотвеченное упражнение с заданными темами.
func seedRealExercise(t *testing.T, q *db.Queries, sentenceID int64, topics ...string) int64 {
	t.Helper()
	payload, _ := json.Marshal(exercisePayload{Prompt: "I have _____ big dog.", Translation: "У меня большая собака.", Choices: []string{"a", "an", "the", "-"}, Topic: "grammar:articles"})
	answer, _ := json.Marshal(exerciseAnswer{Answers: []string{"a"}})
	id, err := q.InsertExercise(context.Background(), db.InsertExerciseParams{
		UserID: 1, Kind: "gap", SentenceID: &sentenceID, Topics: topics, Payload: payload, Answer: answer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// «Порядок слов» на настоящей Postgres: выдача, альтернативный порядок засчитывается, повтор по ключу, 409.
func TestExerciseOrderOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"we", "eat", "red", "apples", "every", "morning"} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w, Pos: "noun", SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	sent := func(lang string, tid int64, text string, wc int16) int64 {
		s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: lang, TatoebaID: tid, Text: text, Author: "a", WordCount: wc, SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	ru := sent("ru", 1, "Мы едим красные яблоки каждое утро.", 6)
	for i, text := range []string{"We eat red apples every morning.", "Every morning we eat red apples."} {
		en := sent("en", int64(10+i), text, 6)
		if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en, RuID: ru, SourceID: ds.ID}); err != nil {
			t.Fatal(err)
		}
	}
	// Грамматические темы уже освоены, так что новая тема skill:order выбирается первой.
	for _, tp := range grammarTopics {
		if err := q.EnsureTopicRating(ctx, db.EnsureTopicRatingParams{UserID: user.ID, Topic: tp}); err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertTopicRating(ctx, db.UpsertTopicRatingParams{UserID: user.ID, Topic: tp, Rating: 1900, Deviation: 80, Volatility: 0.06, AttemptsDelta: 3}); err != nil {
			t.Fatal(err)
		}
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("next: статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Order || ex.Tiles == nil || len(*ex.Tiles) != 6 || len(ex.Choices) != 0 || ex.Prompt != "Мы едим красные яблоки каждое утро." {
		t.Fatalf("упражнение: %+v", ex)
	}
	var answerJSON []byte
	if err := pool.QueryRow(ctx, "SELECT answer FROM exercises WHERE id = $1", ex.Id).Scan(&answerJSON); err != nil {
		t.Fatal(err)
	}
	var ans exerciseAnswer
	if err := json.Unmarshal(answerJSON, &ans); err != nil || len(ans.Answers) != 2 {
		t.Fatalf("допустимые ответы: %s (%v)", answerJSON, err)
	}

	// Засчитывается и исходный порядок, и альтернативный; здесь отвечаем тем, что не был выбран первым.
	given := ans.Answers[1]
	first := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given, "k1"), withCookie(token))
	if first.Code != http.StatusOK {
		t.Fatalf("ответ: статус %d, тело %s", first.Code, first.Body)
	}
	if res := decodeResult(t, first); !res.Correct || res.Answer != ans.Answers[0] {
		t.Fatalf("результат: %+v", res)
	}
	again := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given, "k1"), withCookie(token))
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() {
		t.Fatalf("повтор: статус %d, тело %s", again.Code, again.Body)
	}
	if other := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given, "k2"), withCookie(token)); other.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d, ждали 409", other.Code)
	}
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT attempts FROM topic_ratings WHERE topic = 'skill:order'").Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("попыток skill:order: %d (%v)", attempts, err)
	}
	var events int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE type = 'exercise.answered'").Scan(&events); err != nil || events != 1 {
		t.Fatalf("событий: %d (%v)", events, err)
	}
}

func TestExerciseFormOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// Служебные слова — существительными: в подбор попадают только глаголы и прилагательные.
	var walkID int64
	for _, w := range []struct{ lemma, pos string }{{"walk", "verb"}, {"yesterday", "noun"}, {"she", "noun"}, {"walked", "noun"}, {"home", "noun"}} {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w.lemma, Pos: w.pos, SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if w.lemma == "walk" {
			walkID = id
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	sent := func(lang string, tid int64, text string, wc int16) int64 {
		s, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: lang, TatoebaID: tid, Text: text, Author: "a", WordCount: wc, SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	ru := sent("ru", 1, "Вчера она шла домой.", 4)
	en := sent("en", 10, "Yesterday she walked home.", 4)
	if err := q.UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID: en, RuID: ru, SourceID: ds.ID}); err != nil {
		t.Fatal(err)
	}
	freq, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "frequencywords", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range []string{"walk", "walks", "walked", "walking"} {
		if err := q.UpsertWordFrequency(ctx, db.UpsertWordFrequencyParams{SourceID: freq.ID, Word: w, Count: 100, Rank: int32(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	n := int32(4)
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: freq.ID, RowCount: &n}); err != nil {
		t.Fatal(err)
	}
	// Остальные темы освоены, поэтому новая skill:form выбирается первой.
	for _, tp := range append(append([]string{}, grammarTopics...), orderTopic) {
		if err := q.EnsureTopicRating(ctx, db.EnsureTopicRatingParams{UserID: user.ID, Topic: tp}); err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertTopicRating(ctx, db.UpsertTopicRatingParams{UserID: user.ID, Topic: tp, Rating: 1900, Deviation: 80, Volatility: 0.06, AttemptsDelta: 3}); err != nil {
			t.Fatal(err)
		}
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("next: статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Form || ex.Hint == nil || *ex.Hint != "walk" || ex.Prompt != "Yesterday she _____ home." || ex.Topic != "skill:form" {
		t.Fatalf("упражнение: %+v", ex)
	}

	first := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "walked", "k1"), withCookie(token))
	if first.Code != http.StatusOK {
		t.Fatalf("ответ: статус %d, тело %s", first.Code, first.Body)
	}
	res := decodeResult(t, first)
	if !res.Correct || res.Word == nil || res.Word.Lemma != "walk" || res.Word.InCards {
		t.Fatalf("результат: %+v", res)
	}
	again := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "walked", "k1"), withCookie(token))
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() {
		t.Fatalf("повтор: статус %d, тело %s", again.Code, again.Body)
	}
	if other := e.do(http.MethodPost, answerPath, answerBody(ex.Id, "walked", "k2"), withCookie(token)); other.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d, ждали 409", other.Code)
	}
	var lexemeID int64
	if err := pool.QueryRow(ctx, "SELECT (payload->>'lexemeId')::bigint FROM events WHERE type = 'exercise.answered'").Scan(&lexemeID); err != nil || lexemeID != walkID {
		t.Fatalf("lexemeId в событии: %d, ждали %d (%v)", lexemeID, walkID, err)
	}
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT attempts FROM topic_ratings WHERE topic = 'skill:form'").Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("попыток skill:form: %d (%v)", attempts, err)
	}
}

// Банк ошибок на настоящей Postgres: неверный ответ открывает ошибку, через сдвиг срока приходит повтор,
// два верных закрывают её; повтор по ключу ничего не меняет, другой ключ даёт 409, список отдаёт ошибку.
func TestMistakesOnRealDatabase(t *testing.T) {
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
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "tatoeba", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	en, err := q.UpsertSentence(ctx, db.UpsertSentenceParams{Lang: "en", TatoebaID: 1, Text: "I have a big dog.", Author: "CK", WordCount: 5, SourceID: ds.ID})
	if err != nil {
		t.Fatal(err)
	}
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	first := seedRealExercise(t, q, en.ID, "grammar:articles", "level:A1")
	res1 := e.do(http.MethodPost, answerPath, answerBody(first, "the", "k1"), withCookie(token))
	if res1.Code != http.StatusOK {
		t.Fatalf("ответ: %d %s", res1.Code, res1.Body)
	}
	if n := count("SELECT count(*) FROM mistakes WHERE status = 'open' AND streak = 0"); n != 1 {
		t.Fatalf("открытых ошибок %d, ждали 1", n)
	}
	if again := e.do(http.MethodPost, answerPath, answerBody(first, "the", "k1"), withCookie(token)); again.Body.String() != res1.Body.String() ||
		count("SELECT count(*) FROM mistakes") != 1 || count("SELECT count(*) FROM events WHERE type = 'mistake.opened'") != 1 {
		t.Fatalf("повтор по ключу изменил данные: %s", again.Body)
	}
	if rec := e.do(http.MethodPost, answerPath, answerBody(first, "a", "k2"), withCookie(token)); rec.Code != http.StatusConflict {
		t.Fatalf("другой ключ: %d", rec.Code)
	}
	// Срок ещё не наступил: повтора нет.
	if ex := decodeExercise(t, getNext(t, e, token)); ex.Repeat != nil {
		t.Fatalf("повтор до срока: %+v", ex)
	}
	// Срок наступил: приходит повтор. После повтора два следующих упражнения обычные, затем повтор снова возможен.
	for i, want := range []int{1, 2} {
		if _, err := pool.Exec(ctx, "UPDATE mistakes SET next_due = now() - interval '1 minute' WHERE status = 'open'"); err != nil {
			t.Fatal(err)
		}
		rep := decodeExercise(t, getNext(t, e, token))
		if rep.Repeat == nil || !*rep.Repeat {
			t.Fatalf("повтор %d не выдан: %+v", i, rep)
		}
		if n := count("SELECT count(*) FROM exercises WHERE id = $1 AND mistake_id IS NOT NULL", rep.Id); n != 1 {
			t.Fatal("в exercises нет mistake_id")
		}
		res := decodeResult(t, e.do(http.MethodPost, answerPath, answerBody(rep.Id, "a", fmt.Sprintf("r%d", i)), withCookie(token)))
		if res.Mistake == nil || res.Mistake.Streak != want {
			t.Fatalf("повтор %d: %+v", i, res.Mistake)
		}
		if i == 1 {
			if res.Mistake.Status != api.Closed {
				t.Fatalf("ошибка не закрыта: %+v", res.Mistake)
			}
			break
		}
		// Ошибка снова просрочена, но два упражнения после повтора обычные.
		if _, err := pool.Exec(ctx, "UPDATE mistakes SET next_due = now() - interval '1 minute' WHERE status = 'open'"); err != nil {
			t.Fatal(err)
		}
		for j := range 2 {
			if ex := decodeExercise(t, getNext(t, e, token)); ex.Repeat != nil && *ex.Repeat {
				t.Fatalf("повтор выдан подряд (после повтора %d обычных)", j)
			}
			ord := seedRealExercise(t, q, en.ID, "grammar:articles", "level:A1")
			if rec := e.do(http.MethodPost, answerPath, answerBody(ord, "a", fmt.Sprintf("o%d-%d", i, j)), withCookie(token)); rec.Code != http.StatusOK {
				t.Fatalf("обычный ответ: %d %s", rec.Code, rec.Body)
			}
		}
	}
	if count("SELECT count(*) FROM mistakes WHERE status = 'closed' AND closed_at IS NOT NULL") != 1 ||
		count("SELECT count(*) FROM events WHERE type = 'mistake.closed'") != 1 {
		t.Fatal("закрытие не записано")
	}

	// Новая ошибка видна в списке.
	second := seedRealExercise(t, q, en.ID, "grammar:articles", "level:A1")
	e.do(http.MethodPost, answerPath, answerBody(second, "an", "k3"), withCookie(token))
	rec := e.do(http.MethodGet, mistakesPath, "", withCookie(token))
	var list api.MistakeList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Open) != 1 || list.Counts.Open != 1 ||
		list.Counts.ClosedThisWeek != 1 || list.Open[0].Answer != "a" || list.Open[0].Given != "an" {
		t.Fatalf("список: %d %s (%v)", rec.Code, rec.Body, err)
	}
}

// Прогресс за сегодня на настоящей Postgres: серия и заморозка по событиям за прошлые сутки.
func TestProgressOnRealDatabase(t *testing.T) {
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
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)

	midday := func(back int) time.Time {
		n := time.Now().UTC().AddDate(0, 0, -back)
		return time.Date(n.Year(), n.Month(), n.Day(), 10, 0, 0, 0, time.UTC)
	}
	for _, back := range []int{1, 2, 4} { // сутки 3 назад пропущены: заморозка
		for _, r := range minimumDay(midday(back)) {
			if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: r.Type, SchemaVersion: 1, OccurredAt: r.OccurredAt, Payload: []byte(`{"correct":true}`)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// событие другого типа не должно влиять
	if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: "card.added", SchemaVersion: 1, OccurredAt: ts(midday(1)), Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}

	rec := e.do(http.MethodGet, progressPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var p api.ProgressToday
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	wantFreeze := midday(3).Format("2006-01-02")
	if p.StreakDays != 3 || p.FreezeUsedOn == nil || *p.FreezeUsedOn != wantFreeze || p.GoalMinutes != defaultGoalMinutes || p.DoneToday {
		t.Fatalf("прогресс: %+v (ожидали серию 3, заморозку %s)", p, wantFreeze)
	}
}

// Обзор прогресса на настоящей Postgres: события за несколько суток дают старт, темп и накопленные часы.
func TestOverviewOnRealDatabase(t *testing.T) {
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
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	midday := func(back int) time.Time {
		n := time.Now().UTC().AddDate(0, 0, -back)
		return time.Date(n.Year(), n.Month(), n.Day(), 10, 0, 0, 0, time.UTC)
	}
	for _, back := range []int{1, 5, 20} {
		for _, r := range minimumDay(midday(back)) {
			if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: r.Type, SchemaVersion: 1, OccurredAt: r.OccurredAt, Payload: []byte(`{"correct":true}`)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	rec := e.do(http.MethodGet, overviewPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var o api.ProgressOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	wantStart := midday(20).Format("2006-01-02")
	if o.StartedOn == nil || *o.StartedOn != wantStart || o.DaysSinceStart != 21 || !o.PaceReady || o.StudiedMinutes != 30 || len(o.Daily) != 90 {
		t.Fatalf("обзор: %+v (ожидали старт %s, 21 сутки, 30 минут)", o, wantStart)
	}
	if last := o.Daily[89]; last.CumulativeHours != 0.5 {
		t.Fatalf("накопленные часы к сегодня: %v, ожидали 0.5", last.CumulativeHours)
	}
}

// Вехи на настоящей Postgres: события за несколько суток дают часы, ответы и «первое».
func TestMilestonesOnRealDatabase(t *testing.T) {
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
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	midday := func(back int) time.Time {
		n := time.Now().UTC().AddDate(0, 0, -back)
		return time.Date(n.Year(), n.Month(), n.Day(), 10, 0, 0, 0, time.UTC)
	}
	for _, back := range []int{3, 2, 1} {
		for _, r := range minimumDay(midday(back)) {
			if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: r.Type, SchemaVersion: 1, OccurredAt: r.OccurredAt, Payload: []byte(`{"correct":true}`)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: "mistake.closed", SchemaVersion: 1, OccurredAt: ts(midday(1)), Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	rec := e.do(http.MethodGet, milestonesPath, "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	var l api.MilestoneList
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	get := func(id string) api.Milestone {
		for _, m := range l.Milestones {
			if m.Id == id {
				return m
			}
		}
		t.Fatalf("нет вехи %s", id)
		return api.Milestone{}
	}
	if s3 := get("streak-3"); s3.ReachedOn == nil || *s3.ReachedOn != midday(1).Format("2006-01-02") {
		t.Fatalf("streak-3 достигнута вчера: %+v", s3)
	}
	if h := get("hours-1"); h.ReachedOn != nil {
		t.Fatalf("30 минут: час ещё не накоплен: %+v", h)
	}
	if a := get("answers-100"); a.Progress.Value != 33 {
		t.Fatalf("ответов 33 (3 суток × 11): %+v", a)
	}
	if f := get("first-mistake-closed"); f.ReachedOn == nil || *f.ReachedOn != midday(1).Format("2006-01-02") {
		t.Fatalf("first-mistake-closed: %+v", f)
	}
	if len(l.NewToday) != 0 {
		t.Fatalf("сегодня занятий нет: %v", l.NewToday)
	}
}

// Предложение сменить уровень на настоящей Postgres: поиск темы в payload.topics, предел 30, свежие первыми, отказ из журнала.
func TestLevelSuggestionOnRealDatabase(t *testing.T) {
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
	if _, err := q.UpsertSettings(ctx, db.UpsertSettingsParams{UserID: 1, CurrentLevel: "B1", TargetLevel: "B2", DailyMinMinutes: 30, Timezone: "UTC"}); err != nil {
		t.Fatal(err)
	}
	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	add := func(at time.Time, correct bool, topics ...string) {
		payload, _ := json.Marshal(map[string]any{"correct": correct, "topics": topics})
		if _, err := q.AppendEvent(ctx, db.AppendEventParams{UserID: 1, Type: "exercise.answered", SchemaVersion: 1, OccurredAt: ts(at), Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for i := 0; i < 20; i++ { // старые неверные на уровне B1 (10 суток назад): вытесняются свежими 30
		add(now.AddDate(0, 0, -10).Add(time.Duration(i)*time.Minute), false, "level:B1", "grammar:articles")
	}
	for i := 0; i < 30; i++ { // свежие верные
		add(now.Add(-time.Duration(30-i)*time.Minute), i != 3, "level:B1", "pos:verb")
	}
	for i := 0; i < 40; i++ { // другой уровень не считается
		add(now.Add(-time.Duration(i)*time.Second), false, "level:A2")
	}
	get := func() api.LevelSuggestionResponse {
		rec := e.do(http.MethodGet, levelPath, "", withCookie(token))
		if rec.Code != http.StatusOK {
			t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
		}
		var out api.LevelSuggestionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := get()
	if out.Suggestion == nil || out.Suggestion.Direction != api.Up || out.Suggestion.To != api.LevelB2 || out.Suggestion.Correct != 29 || out.Suggestion.Total != 30 {
		t.Fatalf("ждали повышение до B2, 29 из 30: %+v", out.Suggestion)
	}
	if rec := e.do(http.MethodPost, dismissPath, "", withCookie(token)); rec.Code != http.StatusNoContent {
		t.Fatalf("отказ: статус %d, тело %s", rec.Code, rec.Body)
	}
	if out := get(); out.Suggestion != nil {
		t.Fatalf("после отказа предложения нет: %+v", out.Suggestion)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE user_id = 1 AND type = 'level.suggestion.dismissed'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("событие отказа: n=%d err=%v", n, err)
	}
}

// «Сопоставление» на настоящей Postgres: слова уровня, словарь последнего завершённого импорта, запись без предложения,
// ответ с частичным счётом, повтор по ключу, параллельные ответы.
func TestExerciseMatchOnRealDatabase(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{Login: testLogin, PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "cefrj", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	dict, err := q.UpsertDataSource(ctx, db.UpsertDataSourceParams{Name: "freedict-eng-rus", Version: "t", License: "l", Url: "u", Attribution: "a"})
	if err != nil {
		t.Fatal(err)
	}
	words := []struct{ lemma, pos, dictPos, ru string }{
		{"apple", "noun", "n", "яблоко"}, {"house", "noun", "n", "дом"}, {"water", "noun", "n", "вода"},
		{"book", "noun", "n", "книга"}, {"dog", "noun", "n", "собака"}, {"cat", "noun", "n", "кошка"},
	}
	for _, w := range words {
		id, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w.lemma, Pos: w.pos, SourceID: ds.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID: id, SourceID: ds.ID, Level: "A1", Extra: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertWordTranslation(ctx, db.UpsertWordTranslationParams{SourceID: dict.ID, Lemma: w.lemma, Pos: w.dictPos, SenseNo: 1, Ru: w.ru}); err != nil {
			t.Fatal(err)
		}
	}
	n := int32(len(words))
	if err := q.FinishDataSource(ctx, db.FinishDataSourceParams{ID: dict.ID, RowCount: &n}); err != nil {
		t.Fatal(err)
	}
	for _, tp := range append(append([]string{}, grammarTopics...), orderTopic, formTopic) {
		if err := q.EnsureTopicRating(ctx, db.EnsureTopicRatingParams{UserID: user.ID, Topic: tp}); err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertTopicRating(ctx, db.UpsertTopicRatingParams{UserID: user.ID, Topic: tp, Rating: 1900, Deviation: 80, Volatility: 0.06, AttemptsDelta: 3}); err != nil {
			t.Fatal(err)
		}
	}

	e := env{h: NewHandler(pool, NewPgStore(pool), auth.NewThrottle(5, time.Minute, time.Now))}
	token := login(t, e)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("next: статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Match || ex.Topic != "skill:match" || ex.Words == nil || len(*ex.Words) != 5 || len(*ex.Options) != 5 {
		t.Fatalf("упражнение: %+v", ex)
	}
	var kind string
	var sentence *int64
	var answerRaw []byte
	if err := pool.QueryRow(ctx, "SELECT kind, sentence_id, answer FROM exercises WHERE id = $1", ex.Id).Scan(&kind, &sentence, &answerRaw); err != nil || kind != "match" || sentence != nil {
		t.Fatalf("запись: kind=%q sentence=%v err=%v", kind, sentence, err)
	}
	var stored exerciseAnswer
	if err := json.Unmarshal(answerRaw, &stored); err != nil || len(stored.Pairs) != 5 {
		t.Fatalf("ответ в записи: %s (%v)", answerRaw, err)
	}

	swapped := slices.Clone(stored.Pairs)
	swapped[0].Ru, swapped[1].Ru = swapped[1].Ru, swapped[0].Ru
	first := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given(swapped), "k1"), withCookie(token))
	if first.Code != http.StatusOK {
		t.Fatalf("ответ: статус %d, тело %s", first.Code, first.Body)
	}
	res := decodeResult(t, first)
	if res.Correct || res.Pairs == nil || len(*res.Pairs) != 5 {
		t.Fatalf("результат: %+v", res)
	}
	again := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given(swapped), "k1"), withCookie(token))
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() {
		t.Fatalf("повтор по ключу: статус %d, тело %s, ждали то же, что %s", again.Code, again.Body, first.Body)
	}
	if other := e.do(http.MethodPost, answerPath, answerBody(ex.Id, given(swapped), "k2"), withCookie(token)); other.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d, ждали 409", other.Code)
	}
	var attempts int
	var rating float64
	if err := pool.QueryRow(ctx, "SELECT attempts, rating FROM topic_ratings WHERE user_id = $1 AND topic = 'skill:match'", user.ID).Scan(&attempts, &rating); err != nil || attempts != 1 {
		t.Fatalf("рейтинг skill:match: attempts=%d err=%v", attempts, err)
	}
	var correctPairs int
	if err := pool.QueryRow(ctx, "SELECT (payload->>'correctPairs')::int FROM events WHERE type = 'exercise.answered'").Scan(&correctPairs); err != nil || correctPairs != 3 {
		t.Fatalf("correctPairs в событии: %d (%v)", correctPairs, err)
	}

	// Параллельные ответы на новое упражнение: засчитывается один, остальные получают повтор или 409.
	ex2 := decodeExercise(t, getNext(t, e, token))
	if ex2.Kind != api.Match {
		t.Fatalf("второе упражнение: %+v", ex2)
	}
	pairs2 := storedAnswerFromDB(t, pool, ex2.Id).Pairs
	var wg sync.WaitGroup
	codes := make([]int, 6)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.do(http.MethodPost, answerPath, answerBody(ex2.Id, given(pairs2), "kk"), withCookie(token)).Code
		}()
	}
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("параллельные ответы с одним ключом: коды %v", codes)
		}
	}
	if err := pool.QueryRow(ctx, "SELECT attempts FROM topic_ratings WHERE user_id = $1 AND topic = 'skill:match'", user.ID).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("после параллельных ответов attempts=%d (%v), ждали 2", attempts, err)
	}
}

func storedAnswerFromDB(t *testing.T, pool *pgxpool.Pool, id int64) exerciseAnswer {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), "SELECT answer FROM exercises WHERE id = $1", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var a exerciseAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	return a
}
