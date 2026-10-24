package httpapi

import "net/http"

// GetProgressToday — заглушка до Task 3 плана «Маршрут дня».
func (s *authServer) GetProgressToday(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	writeError(w, http.StatusNotImplemented, "internal", "не реализовано")
}
