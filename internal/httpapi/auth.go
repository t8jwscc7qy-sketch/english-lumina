package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/auth"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

const (
	sessionCookie = "session"
	sessionTTL    = 30 * 24 * time.Hour

	maxLoginBody     = 4 << 10 // логин + пароль заведомо короче
	maxPasswordBytes = 256
	maxUserAgentLen  = 512 // CHECK в таблице sessions

	// Каждая проверка пароля занимает ~19 МиБ (argon2id), поэтому одновременных проверок немного.
	maxConcurrentVerify = 4
)

// Тот же шаблон, что в CHECK таблицы users: всё остальное не может быть логином,
// а нулевой байт PostgreSQL не принимает вовсе.
var loginRe = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

// AuthStore — то, что нужно авторизации от базы. Его реализует *db.Queries.
type AuthStore interface {
	GetUserByLogin(ctx context.Context, login string) (db.User, error)
	GetUserByID(ctx context.Context, id int64) (db.User, error)
	CreateSession(ctx context.Context, arg db.CreateSessionParams) (db.Session, error)
	GetActiveSession(ctx context.Context, tokenHash []byte) (db.Session, error)
	TouchSession(ctx context.Context, id int64) error
	RevokeSession(ctx context.Context, tokenHash []byte) error
	GetSettings(ctx context.Context, userID int64) (db.Setting, error)
	UpsertSettings(ctx context.Context, arg db.UpsertSettingsParams) (db.Setting, error)
	ListDataSources(ctx context.Context) ([]db.DataSource, error)
	SearchLexemes(ctx context.Context, arg db.SearchLexemesParams) ([]db.SearchLexemesRow, error)
	ListLexemeLevels(ctx context.Context, ids []int64) ([]db.ListLexemeLevelsRow, error)
	ListExamples(ctx context.Context, arg db.ListExamplesParams) ([]db.ListExamplesRow, error)

	GetLexemeByPair(ctx context.Context, arg db.GetLexemeByPairParams) (db.Lexeme, error)
	InsertCard(ctx context.Context, arg db.InsertCardParams) (int64, error)
	GetCardByLexeme(ctx context.Context, arg db.GetCardByLexemeParams) (db.Card, error)
	GetCardForUpdate(ctx context.Context, arg db.GetCardForUpdateParams) (db.GetCardForUpdateRow, error)
	UpdateCardState(ctx context.Context, arg db.UpdateCardStateParams) error
	NextDueCard(ctx context.Context, arg db.NextDueCardParams) (db.NextDueCardRow, error)
	CountCards(ctx context.Context, arg db.CountCardsParams) (db.CountCardsRow, error)
	ListCardLexemeIDs(ctx context.Context, arg db.ListCardLexemeIDsParams) ([]int64, error)
	AppendEvent(ctx context.Context, arg db.AppendEventParams) (int64, error)
	GetEventByKey(ctx context.Context, arg db.GetEventByKeyParams) (db.Event, error)
	ListWordFrequencies(ctx context.Context, arg db.ListWordFrequenciesParams) ([]db.ListWordFrequenciesRow, error)

	InsertExercise(ctx context.Context, arg db.InsertExerciseParams) (int64, error)
	GetExerciseForUpdate(ctx context.Context, arg db.GetExerciseForUpdateParams) (db.Exercise, error)
	MarkExerciseAnswered(ctx context.Context, arg db.MarkExerciseAnsweredParams) error
	EnsureTopicRating(ctx context.Context, arg db.EnsureTopicRatingParams) error
	ListTopicRatings(ctx context.Context, userID int64) ([]db.TopicRating, error)
	GetTopicRatingForUpdate(ctx context.Context, arg db.GetTopicRatingForUpdateParams) (db.TopicRating, error)
	UpsertTopicRating(ctx context.Context, arg db.UpsertTopicRatingParams) error
	SampleSentences(ctx context.Context, arg db.SampleSentencesParams) ([]db.SampleSentencesRow, error)
	SentenceLevels(ctx context.Context, words []string) ([]db.SentenceLevelsRow, error)
	UsedSentences(ctx context.Context, arg db.UsedSentencesParams) ([]int64, error)
	SentenceIDRange(ctx context.Context) (db.SentenceIDRangeRow, error)
	RandomLexemesByLevel(ctx context.Context, arg db.RandomLexemesByLevelParams) ([]db.RandomLexemesByLevelRow, error)
	ListEnglishAlternatives(ctx context.Context, arg db.ListEnglishAlternativesParams) ([]db.ListEnglishAlternativesRow, error)
	ListWeakCardWords(ctx context.Context, arg db.ListWeakCardWordsParams) ([]db.ListWeakCardWordsRow, error)

	InsertMistake(ctx context.Context, arg db.InsertMistakeParams) (int64, error)
	GetOpenMistakeForUpdate(ctx context.Context, arg db.GetOpenMistakeForUpdateParams) (db.Mistake, error)
	GetMistakeForUpdate(ctx context.Context, arg db.GetMistakeForUpdateParams) (db.Mistake, error)
	ResetMistake(ctx context.Context, arg db.ResetMistakeParams) error
	UpdateMistakeProgress(ctx context.Context, arg db.UpdateMistakeProgressParams) error
	NextDueMistake(ctx context.Context, arg db.NextDueMistakeParams) (db.Mistake, error)
	CountRepeatsToday(ctx context.Context, arg db.CountRepeatsTodayParams) (int64, error)
	LastTwoExerciseRepeatFlags(ctx context.Context, userID int64) ([]bool, error)
	GetUnansweredRepeat(ctx context.Context, arg db.GetUnansweredRepeatParams) (db.Exercise, error)
	ListOpenMistakes(ctx context.Context, arg db.ListOpenMistakesParams) ([]db.Mistake, error)
	ListActivityEvents(ctx context.Context, arg db.ListActivityEventsParams) ([]db.ListActivityEventsRow, error)
	ListLevelAnswers(ctx context.Context, arg db.ListLevelAnswersParams) ([]bool, error)
	LastLevelSuggestionDismissal(ctx context.Context, userID int64) (pgtype.Timestamptz, error)
	CountMistakes(ctx context.Context, arg db.CountMistakesParams) (db.CountMistakesRow, error)

	// InTx выполняет fn в одной транзакции: ошибка или паника в fn откатывают всё, успех фиксирует.
	// Хранилище, переданное в fn, работает на этой транзакции; вложенный InTx использует ту же транзакцию.
	InTx(ctx context.Context, fn func(AuthStore) error) error
}

var errNoSession = errors.New("нет действующей сессии")

type ctxKey struct{}

// UserFromContext возвращает пользователя, которого положил RequireSession.
func UserFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(db.User)
	return u, ok
}

// RequireSession пропускает дальше только запросы с действующей сессией.
func RequireSession(store AuthStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := authenticate(r, store)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

// authenticate находит пользователя по cookie сессии. Нет cookie, мусор, истёкшая
// или отозванная сессия — одинаково errNoSession.
func authenticate(r *http.Request, store AuthStore) (db.User, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return db.User{}, errNoSession
	}
	sess, err := store.GetActiveSession(r.Context(), auth.HashToken(c.Value))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, errNoSession
	}
	if err != nil {
		return db.User{}, err
	}
	if err := store.TouchSession(r.Context(), sess.ID); err != nil {
		slog.Warn("не удалось обновить last_seen_at", "err", err)
	}
	user, err := store.GetUserByID(r.Context(), sess.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, errNoSession
	}
	return user, err
}

func writeAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoSession) {
		writeError(w, http.StatusUnauthorized, api.ErrorErrorCodeUnauthorized, "Нужно войти")
		return
	}
	slog.Error("проверка сессии", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

// authServer реализует api.ServerInterface.
type authServer struct {
	store    AuthStore
	throttle *auth.Throttle
	// Хеш для несуществующего логина: проверка пароля занимает столько же времени,
	// и по скорости ответа нельзя узнать, есть ли такой логин.
	dummyHash func() (string, error)
	// Семафор проверок пароля: занято место — идёт проверка.
	verifySem chan struct{}
	// Источник случайности упражнений: число от 0 до n-1 (n > 0). В тестах подменяется.
	rnd func(n int64) int64
	// Часы сервера; в тестах подменяются.
	now func() time.Time
}

func newAuthServer(store AuthStore, throttle *auth.Throttle) *authServer {
	return &authServer{
		store:     store,
		throttle:  throttle,
		verifySem: make(chan struct{}, maxConcurrentVerify),
		dummyHash: sync.OnceValues(func() (string, error) { return auth.HashPassword("dummy-password") }),
		rnd:       rand.Int64N,
		now:       time.Now,
	}
}

func (s *authServer) Login(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Некорректный запрос")
		return
	}
	if !loginRe.MatchString(req.Login) || len(req.Password) == 0 || len(req.Password) > maxPasswordBytes {
		writeError(w, http.StatusBadRequest, api.ErrorErrorCodeInvalidRequest, "Проверь логин и пароль: логин 3–32 символа, пароль до 256 байт")
		return
	}

	// Попытка занимается до проверки пароля, иначе параллельные запросы обходят лимит.
	ip := clientIP(r)
	if !s.throttle.Allow(ip) {
		writeError(w, http.StatusTooManyRequests, api.ErrorErrorCodeTooManyAttempts, "Слишком много попыток, попробуй через минуту")
		return
	}

	user, err := s.store.GetUserByLogin(r.Context(), req.Login)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		s.fail(w, err)
		return
	}

	release, ok := s.acquireVerify()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, api.ErrorErrorCodeBusy, "Сервер занят, повтори через несколько секунд")
		return
	}
	hash := user.PasswordHash
	if errors.Is(err, pgx.ErrNoRows) {
		user = db.User{}
		hash, err = s.dummyHash()
	}
	var matched bool
	if err == nil {
		matched, err = auth.VerifyPassword(req.Password, hash)
	}
	release()
	if err != nil {
		s.fail(w, err)
		return
	}
	if !matched || user.ID == 0 {
		// В журнал идёт адрес, а не логин: в поле логина могут набрать пароль.
		slog.Info("неудачный вход", "ip", ip)
		writeError(w, http.StatusUnauthorized, api.ErrorErrorCodeInvalidCredentials, "Неверный логин или пароль")
		return
	}

	token, tokenHash, err := auth.NewToken()
	if err != nil {
		s.fail(w, err)
		return
	}
	_, err = s.store.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionTTL), Valid: true},
		UserAgent: userAgent(r),
	})
	if err != nil {
		s.fail(w, err)
		return
	}

	s.throttle.Reset(ip)
	setSessionCookie(w, token, int(sessionTTL/time.Second))
	w.WriteHeader(http.StatusNoContent)
}

func (s *authServer) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := s.store.RevokeSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.fail(w, err)
			return
		}
	}
	setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (s *authServer) GetMe(w http.ResponseWriter, r *http.Request) {
	user, err := authenticate(r, s.store)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.Me{Login: user.Login})
}

// acquireVerify занимает место для проверки пароля; false — все места заняты.
func (s *authServer) acquireVerify() (release func(), ok bool) {
	select {
	case s.verifySem <- struct{}{}:
		return func() { <-s.verifySem }, true
	default:
		return nil, false
	}
}

func (s *authServer) fail(w http.ResponseWriter, err error) {
	slog.Error("ошибка авторизации", "err", err)
	writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
}

func setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clientIP — ключ клиента для ограничителя: адрес без порта. Для IPv6 берётся сеть /64,
// иначе один клиент обходил бы лимит сменой адреса внутри своей сети.
// Сервер стоит без прокси, поэтому RemoteAddr надёжен; за обратным прокси здесь
// понадобится доверенный заголовок.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return host
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func userAgent(r *http.Request) *string {
	ua := r.UserAgent()
	if ua == "" {
		return nil
	}
	if len(ua) > maxUserAgentLen {
		ua = ua[:maxUserAgentLen]
	}
	// Заголовок присылает клиент и не обязан быть UTF-8, а PostgreSQL такую строку отвергнет.
	ua = strings.ToValidUTF8(ua, "")
	return &ua
}
