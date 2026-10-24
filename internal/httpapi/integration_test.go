package httpapi

import (
	"context"
	"net/http"
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
	e := env{h: NewHandler(pool, q, auth.NewThrottle(5, time.Minute, time.Now))}

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
	e := env{h: NewHandler(pool, q, auth.NewThrottle(5, time.Minute, time.Now))}
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

	e := env{h: NewHandler(pool, q, auth.NewThrottle(5, time.Minute, time.Now))}
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

	e := env{h: NewHandler(pool, q, auth.NewThrottle(5, time.Minute, time.Now))}
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
