package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
)

// ListSources отдаёт загруженные источники данных: по лицензиям их условия и атрибуцию нужно показывать.
func (s *authServer) ListSources(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticate(r, s.store); err != nil {
		writeAuthError(w, err)
		return
	}
	rows, err := s.store.ListDataSources(r.Context())
	if err != nil {
		slog.Error("список источников", "err", err)
		writeError(w, http.StatusInternalServerError, api.ErrorErrorCodeInternal, "Внутренняя ошибка, попробуй ещё раз")
		return
	}
	out := api.SourceList{Sources: []api.Source{}} // пустой список должен быть [], а не null
	for _, row := range rows {
		if !row.ImportedAt.Valid { // запрос уже отсекает, но недогруженное не должно просочиться
			continue
		}
		count := 0
		if row.RowCount != nil {
			count = int(*row.RowCount)
		}
		out.Sources = append(out.Sources, api.Source{
			Name: row.Name, Version: row.Version, License: row.License, Url: row.Url,
			Attribution: row.Attribution, RowCount: count, ImportedAt: row.ImportedAt.Time,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
