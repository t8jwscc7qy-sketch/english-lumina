package httpapi

import (
	"net/http"
	"net/url"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

// checkOrigin защищает от CSRF поверх SameSite=Strict: изменяющие запросы принимаются
// только со своего источника. Запрос без Origin (curl, тесты) проходит, если браузер
// не пометил его как межсайтовый.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !originAllowed(r) {
			writeError(w, http.StatusForbidden, api.ErrorErrorCodeForbiddenOrigin, "Запрос с чужого источника")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == r.Host
}
