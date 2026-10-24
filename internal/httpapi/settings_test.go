package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func (f *fakeStore) GetSettings(_ context.Context, userID int64) (db.Setting, error) {
	if f.settings == nil || f.settings.UserID != userID {
		return db.Setting{}, pgx.ErrNoRows
	}
	return *f.settings, nil
}

func (f *fakeStore) UpsertSettings(_ context.Context, p db.UpsertSettingsParams) (db.Setting, error) {
	f.upserts++
	s := db.Setting{
		UserID: p.UserID, CurrentLevel: p.CurrentLevel, TargetLevel: p.TargetLevel,
		TargetDate: p.TargetDate, DailyMinMinutes: p.DailyMinMinutes, Timezone: p.Timezone,
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.settings = &s
	return s, nil
}

const settingsPath = "/api/v1/settings"

func settingsBody(t *testing.T, mut func(map[string]any)) string {
	t.Helper()
	m := map[string]any{
		"currentLevel": "B1", "targetLevel": "B2", "dailyMinutes": 90,
		"timezone": "Europe/Moscow",
	}
	if mut != nil {
		mut(m)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func errorCode(t *testing.T, body string) api.ErrorErrorCode {
	t.Helper()
	var e api.Error
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("тело ошибки %q: %v", body, err)
	}
	return e.Error.Code
}

func TestSettingsNeedSession(t *testing.T) {
	e := newEnv(t)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		body := ""
		if method == http.MethodPut {
			body = settingsBody(t, nil)
		}
		if rec := e.do(method, settingsPath, body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s без cookie: статус %d, ожидали 401", method, rec.Code)
		}
	}
	if e.store.upserts != 0 {
		t.Fatal("PUT без сессии записал настройки")
	}
}

func TestGetSettingsBeforeOnboarding(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodGet, settingsPath, "", withCookie(token))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ожидали 404", rec.Code)
	}
	if got := errorCode(t, rec.Body.String()); got != api.ErrorErrorCodeOnboardingRequired {
		t.Fatalf("код %q, ожидали onboarding_required", got)
	}
}

func TestPutThenGetSettings(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	date := time.Now().AddDate(0, 8, 0).Format("2006-01-02")
	body := settingsBody(t, func(m map[string]any) { m["targetDate"] = date })

	rec := e.do(http.MethodPut, settingsPath, body, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: статус %d, тело %s", rec.Code, rec.Body)
	}
	for _, want := range []string{`"currentLevel":"B1"`, `"targetLevel":"B2"`, `"dailyMinutes":90`, `"timezone":"Europe/Moscow"`, `"targetDate":"` + date + `"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("в ответе PUT нет %s: %s", want, rec.Body)
		}
	}

	got := e.do(http.MethodGet, settingsPath, "", withCookie(token))
	if got.Code != http.StatusOK || got.Body.String() != rec.Body.String() {
		t.Fatalf("GET после PUT: статус %d, тело %s; ожидали то же, что вернул PUT (%s)", got.Code, got.Body, rec.Body)
	}
}

func TestPutSettingsWithoutDate(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodPut, settingsPath, settingsBody(t, nil), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "targetDate") && !strings.Contains(rec.Body.String(), `"targetDate":null`) {
		t.Fatalf("без даты targetDate должен отсутствовать или быть null: %s", rec.Body)
	}
}

func TestPutSettingsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body func(t *testing.T) string
	}{
		{"уровень D1", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["currentLevel"] = "D1" })
		}},
		{"цель ниже уровня", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["currentLevel"] = "B2"; m["targetLevel"] = "B1" })
		}},
		{"241 минута", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["dailyMinutes"] = 241 })
		}},
		{"4 минуты", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["dailyMinutes"] = 4 })
		}},
		{"неизвестный пояс", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["timezone"] = "Mars/Base" })
		}},
		{"дата в прошлом", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["targetDate"] = "2020-01-01" })
		}},
		{"дата не в формате", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["targetDate"] = "завтра" })
		}},
		{"лишнее поле", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["admin"] = true })
		}},
		{"битый JSON", func(*testing.T) string { return `{"currentLevel":` }},
		{"тело больше лимита", func(t *testing.T) string {
			return settingsBody(t, func(m map[string]any) { m["timezone"] = strings.Repeat("x", maxSettingsBody) })
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			token := login(t, e)
			rec := e.do(http.MethodPut, settingsPath, tc.body(t), withCookie(token))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("статус %d, ожидали 400; тело %s", rec.Code, rec.Body)
			}
			if got := errorCode(t, rec.Body.String()); got != api.ErrorErrorCodeInvalidRequest {
				t.Fatalf("код %q, ожидали invalid_request", got)
			}
			if e.store.upserts != 0 {
				t.Fatal("некорректный ввод дошёл до базы")
			}
		})
	}
}

func TestPutSettingsForeignOrigin(t *testing.T) {
	e := newEnv(t)
	token := login(t, e)
	rec := e.do(http.MethodPut, settingsPath, settingsBody(t, nil), withCookie(token), withHeader("Origin", "https://evil.example"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("статус %d, ожидали 403", rec.Code)
	}
	if e.store.upserts != 0 {
		t.Fatal("чужой Origin дошёл до базы")
	}
}
