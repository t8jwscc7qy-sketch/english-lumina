package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
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
		if _, err := q.UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma: w[0], Pos: w[1], SourceID: ds.ID}); err != nil {
			t.Fatal(err)
		}
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
	if count(`SELECT count(*) FROM cards`) != 1 {
		t.Fatal("отказы создали карточки")
	}
}
