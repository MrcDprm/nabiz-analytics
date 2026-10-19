package server

import (
	"net/http"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

type siteJSON struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Public bool   `json:"public"`
	Role   string `json:"role"`
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request, u *auth.User) {
	ms, err := site.ForUser(r.Context(), s.store.R, u.ID)
	if err != nil {
		internalError(w, "siteler", err)
		return
	}
	out := make([]siteJSON, 0, len(ms))
	for _, m := range ms {
		out = append(out, siteJSON{ID: m.ID, Domain: m.Domain, Public: m.Public, Role: m.Role})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": out})
}
