package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

func TestExerciseStubs(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/exercises/next", ""},
		{http.MethodPost, "/api/v1/exercises/answer", `{"exerciseId":1,"given":"x","key":"k"}`},
		{http.MethodGet, "/api/v1/exercises/topics", ""},
	}
	for _, rt := range routes {
		t.Run(rt.path, func(t *testing.T) {
			e := newEnv(t)
			if rec := e.do(rt.method, rt.path, rt.body); rec.Code != http.StatusUnauthorized {
				t.Fatalf("без сессии: статус %d, ждали 401", rec.Code)
			}
			token := login(t, e)
			rec := e.do(rt.method, rt.path, rt.body, withCookie(token))
			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("статус %d, ждали 501", rec.Code)
			}
			var got api.Error
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("тело %q: %v", rec.Body.String(), err)
			}
			if got.Error.Code != api.ErrorErrorCodeNotImplemented || got.Error.Message == "" {
				t.Fatalf("тело: %+v", got)
			}
		})
	}
}
