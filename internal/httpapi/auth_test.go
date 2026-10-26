package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	testLogin    = "vlad"
	testPassword = "correct-password-123"
)

var (
	hashOnce sync.Once
	testHash string
)

// passwordHash считается один раз на весь пакет: argon2id намеренно медленный.
func passwordHash(t *testing.T) string {
	t.Helper()
	hashOnce.Do(func() {
		h, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		testHash = h
	})
	return testHash
}

// fakeStore повторяет условия реальных запросов: активна только неотозванная и непросроченная сессия.
type fakeStore struct {
	user          db.User
	sessions      map[string]db.Session // ключ — string(token_hash)
	nextID        int64
	touched       int
	settings      *db.Setting // nil — онбординг не пройден
	sources       []db.DataSource
	lexemes       []db.SearchLexemesRow
	levels        []db.ListLexemeLevelsRow
	lastSearch    *db.SearchLexemesParams
	searches      int
	examples      []db.ListExamplesRow
	lastExamples  *db.ListExamplesParams
	examplesFor   func(query string) []db.ListExamplesRow // если задано, ListExamples отвечает по запросу
	exampleCalls  int
	lexemesByID   map[int64]db.Lexeme
	cardLevels    map[int64]string // уровень слова для очереди (lexeme_id → уровень)
	cards         map[int64]db.Card
	nextCardID    int64
	events        []db.Event
	eventKeys     map[string]bool
	listCardCalls int
	frequencies   []db.ListWordFrequenciesRow
	freqCalls     int
	lastFreq      *db.ListWordFrequenciesParams
	failEvents    bool // AppendEvent возвращает ошибку (проверка отката)
	upserts       int

	exercises        map[int64]db.Exercise
	nextExerciseID   int64
	topicRatings     map[topicKey]db.TopicRating
	sentencePool     []db.SampleSentencesRow         // для SampleSentences, по возрастанию id
	wordLevels       map[string]db.SentenceLevelsRow // для SentenceLevels
	rangeCalls       int
	enText           map[int64]string  // для ListEnglishAlternatives: id английского предложения -> текст
	enWords          map[int64]int16   // и число слов
	enRu             map[int64][]int64 // и русские переводы (id)
	levelCalls       int
	sampleErr        error
	rangeOverride    *db.SentenceIDRangeRow
	levelLexemes     map[string][]db.RandomLexemesByLevelRow // для RandomLexemesByLevel
	weakWords        []db.ListWeakCardWordsRow               // для ListWeakCardWords
	mistakes         map[int64]db.Mistake                    // банк ошибок (mistakes_fake_test.go)
	nextMistakeID    int64
	missOpenOnce     bool                             // GetOpenMistakeForUpdate один раз не находит строку (гонка)
	activity         []fakeActivity                   // события занятий для ListActivityEvents (progress_fake_test.go)
	levelAnswers     []fakeLevelAnswer                // ответы по темам для ListLevelAnswers (level_fake_test.go)
	translations     []db.ListTranslationsForWordsRow // переводы слов для ListTranslationsForWords (exercises_match_fake_test.go)
	kbCards          map[int64]json.RawMessage        // карточки слов базы знаний (ListTeachingCards)
	kbTranslations   map[int64][]string               // выбранные русские переводы слов в базе знаний, основной первым
	failKb           bool                             // чтение базы знаний падает (например, миграция не применена)
	wl               map[int64]fakeWord               // состояние изучения слов одного пользователя (word_learning)
	learnCands       []db.LearnCandidatesRow
	learnPlacement   []db.LearnPlacementCandidatesRow
	learnRelations   []db.LearnRelationsRow
	learnPrefs       *db.LearnPreference
	kbBuilt          bool  // есть успешная сборка базы знаний
	reviewTotal      int64 // повторения за семь суток для точности
	reviewGood       int64
	failLearn        bool                          // запросы «Знакомства» падают
	kbFlags          map[int64]map[string][]string // пометки выбранных переводов: слово → перевод → пометки
	translationCalls int
	failTranslations bool
}

func newFakeStore(t *testing.T) *fakeStore {
	return &fakeStore{
		user:     db.User{ID: 1, Login: testLogin, PasswordHash: passwordHash(t)},
		sessions: map[string]db.Session{},
		nextID:   1,
	}
}

func (f *fakeStore) GetUserByLogin(_ context.Context, login string) (db.User, error) {
	if login != f.user.Login {
		return db.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}

func (f *fakeStore) GetUserByID(_ context.Context, id int64) (db.User, error) {
	if id != f.user.ID {
		return db.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}

func (f *fakeStore) CreateSession(_ context.Context, p db.CreateSessionParams) (db.Session, error) {
	s := db.Session{ID: f.nextID, UserID: p.UserID, TokenHash: p.TokenHash, ExpiresAt: p.ExpiresAt, UserAgent: p.UserAgent}
	f.nextID++
	f.sessions[string(p.TokenHash)] = s
	return s, nil
}

func (f *fakeStore) GetActiveSession(_ context.Context, hash []byte) (db.Session, error) {
	s, ok := f.sessions[string(hash)]
	if !ok || s.RevokedAt.Valid || !s.ExpiresAt.Time.After(time.Now()) {
		return db.Session{}, pgx.ErrNoRows
	}
	return s, nil
}

func (f *fakeStore) TouchSession(context.Context, int64) error { f.touched++; return nil }

func (f *fakeStore) RevokeSession(_ context.Context, hash []byte) error {
	if s, ok := f.sessions[string(hash)]; ok {
		s.RevokedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		f.sessions[string(hash)] = s
	}
	return nil
}

type env struct {
	h     http.Handler
	store *fakeStore
}

func newEnv(t *testing.T) env {
	store := newFakeStore(t)
	th := auth.NewThrottle(5, time.Minute, time.Now)
	return env{h: NewHandler(fakePinger{}, store, th), store: store}
}

type reqOpt func(*http.Request)

func withCookie(v string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookie, Value: v}) }
}

func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

func (e env) do(method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:51234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func loginBody(login, password string) string {
	b, _ := json.Marshal(map[string]string{"login": login, "password": password})
	return string(b)
}

// sessionFrom достаёт значение cookie сессии из ответа.
func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("в ответе нет cookie %q", sessionCookie)
	return nil
}

func login(t *testing.T, e env) string {
	t.Helper()
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("вход: статус %d, тело %s", rec.Code, rec.Body)
	}
	return sessionFrom(t, rec).Value
}

func TestLoginSuccessSetsSecureCookie(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("статус %d, ожидали 204", rec.Code)
	}
	c := sessionFrom(t, rec)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Errorf("атрибуты cookie: HttpOnly=%v Secure=%v SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}
	if c.MaxAge != 30*24*60*60 {
		t.Errorf("Max-Age = %d, ожидали 2592000", c.MaxAge)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, ожидали /", c.Path)
	}
	// В базе лежит хеш токена, а не сам токен.
	if _, ok := e.store.sessions[string(auth.HashToken(c.Value))]; !ok {
		t.Error("сессия с хешем токена не создана")
	}
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	e := newEnv(t)
	wrongPass := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1"))
	noUser := e.do(http.MethodPost, "/api/v1/auth/login", loginBody("nobody", "wrong-password-1"))
	for name, rec := range map[string]*httptest.ResponseRecorder{"неверный пароль": wrongPass, "нет логина": noUser} {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: статус %d, ожидали 401", name, rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: поставлена cookie", name)
		}
	}
	if wrongPass.Body.String() != noUser.Body.String() {
		t.Errorf("тела различаются: %q и %q", wrongPass.Body, noUser.Body)
	}
}

func TestLoginValidation(t *testing.T) {
	e := newEnv(t)
	tests := []struct{ name, body string }{
		{"пустой логин", loginBody("", testPassword)},
		{"логин длиннее 32", loginBody(strings.Repeat("a", 33), testPassword)},
		{"пустой пароль", loginBody(testLogin, "")},
		{"пароль длиннее 256 байт", loginBody(testLogin, strings.Repeat("p", 257))},
		{"не JSON", "{not json"},
		{"пустое тело", ""},
		{"огромное тело", `{"login":"` + strings.Repeat("a", 1<<20) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.do(http.MethodPost, "/api/v1/auth/login", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("статус %d, ожидали 400", rec.Code)
			}
		})
	}
}

func TestLoginThrottle(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: статус %d, ожидали 401", i+1, rec.Code)
		}
	}
	// Блокировка действует и на верный пароль.
	rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("статус %d, ожидали 429", rec.Code)
	}
	// Другой порт того же адреса — тот же ключ.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody(testLogin, testPassword)))
	req.RemoteAddr = "203.0.113.7:60000"
	out := httptest.NewRecorder()
	e.h.ServeHTTP(out, req)
	if out.Code != http.StatusTooManyRequests {
		t.Fatalf("другой порт: статус %d, ожидали 429", out.Code)
	}
}

func TestLoginSuccessResetsThrottle(t *testing.T) {
	e := newEnv(t)
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, "wrong-password-1"))
		}
		if rec := e.do(http.MethodPost, "/api/v1/auth/login", loginBody(testLogin, testPassword)); rec.Code != http.StatusNoContent {
			t.Fatalf("раунд %d: статус %d, ожидали 204", round, rec.Code)
		}
	}
}

func TestMe(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)

	rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидали 200", rec.Code)
	}
	var me struct{ Login string }
	if err := json.NewDecoder(rec.Body).Decode(&me); err != nil || me.Login != testLogin {
		t.Fatalf("тело: %+v err=%v", me, err)
	}

	// Истёкшая сессия.
	expiredToken, hash, _ := auth.NewToken()
	e.store.sessions[string(hash)] = db.Session{
		ID: 99, UserID: 1, TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	}

	unauth := map[string][]reqOpt{
		"без cookie":           nil,
		"мусорная cookie":      {withCookie("garbage")},
		"пустая cookie":        {withCookie("")},
		"истёкшая сессия":      {withCookie(expiredToken)},
		"чужая длинная cookie": {withCookie(strings.Repeat("x", 5000))},
	}
	for name, opts := range unauth {
		t.Run(name, func(t *testing.T) {
			if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", opts...); rec.Code != http.StatusUnauthorized {
				t.Fatalf("статус %d, ожидали 401", rec.Code)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)

	rec := e.do(http.MethodPost, "/api/v1/auth/logout", "", withCookie(token))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("статус %d, ожидали 204", rec.Code)
	}
	if c := sessionFrom(t, rec); c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cookie не стёрта: MaxAge=%d Value=%q", c.MaxAge, c.Value)
	}
	if rec := e.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(token)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("/me после выхода: статус %d, ожидали 401", rec.Code)
	}

	// Повторный выход и выход без cookie — не ошибки.
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", "", withCookie(token)); rec.Code != http.StatusNoContent {
		t.Errorf("повторный выход: статус %d, ожидали 204", rec.Code)
	}
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", ""); rec.Code != http.StatusNoContent {
		t.Errorf("выход без cookie: статус %d, ожидали 204", rec.Code)
	}
}

func TestOriginCheck(t *testing.T) {
	e := newEnv(t)
	body := loginBody(testLogin, testPassword)
	tests := []struct {
		name string
		opts []reqOpt
		want int
	}{
		{"чужой Origin", []reqOpt{withHeader("Origin", "https://evil.example")}, http.StatusForbidden},
		{"свой Origin", []reqOpt{withHeader("Origin", "http://example.com")}, http.StatusNoContent},
		{"без Origin (curl)", nil, http.StatusNoContent},
		{"без Origin, cross-site", []reqOpt{withHeader("Sec-Fetch-Site", "cross-site")}, http.StatusForbidden},
		{"без Origin, same-origin", []reqOpt{withHeader("Sec-Fetch-Site", "same-origin")}, http.StatusNoContent},
		{"Origin: null", []reqOpt{withHeader("Origin", "null")}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.do(http.MethodPost, "/api/v1/auth/login", body, tt.opts...)
			if rec.Code != tt.want {
				t.Fatalf("статус %d, ожидали %d", rec.Code, tt.want)
			}
		})
	}
	// Выход тоже защищён.
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", "", withHeader("Origin", "https://evil.example")); rec.Code != http.StatusForbidden {
		t.Errorf("logout с чужим Origin: статус %d, ожидали 403", rec.Code)
	}
}

func TestHealthNeedsNoSession(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(http.MethodGet, "/health", ""); rec.Code != http.StatusOK {
		t.Fatalf("статус %d, ожидали 200", rec.Code)
	}
}

func TestRequireSession(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)

	var gotLogin string
	protected := RequireSession(e.store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("пользователя нет в контексте")
		}
		gotLogin = u.Login
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || gotLogin != testLogin {
		t.Fatalf("статус %d, логин %q", rec.Code, gotLogin)
	}

	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без cookie: статус %d, ожидали 401", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("error")) {
		t.Errorf("тело 401 не JSON с полем error: %s", rec.Body)
	}
}
