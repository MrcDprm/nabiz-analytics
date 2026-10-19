package server

import (
	"errors"
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

func (s *Server) createSite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.Verified {
		writeError(w, http.StatusForbidden, "site eklemeden önce e-posta adresinizi doğrulayın")
		return
	}
	var in struct {
		Domain string `json:"domain"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	st, err := site.Create(r.Context(), s.store, in.Domain, u.ID)
	switch {
	case errors.Is(err, site.ErrInvalidDomain), errors.Is(err, site.ErrDomainTaken):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		internalError(w, "site ekle", err)
	default:
		writeJSON(w, http.StatusCreated, siteJSON{ID: st.ID, Domain: st.Domain, Role: "owner"})
	}
}

// siteFor, adresteki {id} sitesini bulur ve kullanıcının en az want rolünde
// olduğunu doğrular. Üye değilse site yokmuş gibi 404 döner.
func (s *Server) siteFor(w http.ResponseWriter, r *http.Request, u *auth.User, want string) (*site.Site, string, bool) {
	st, err := site.ByID(r.Context(), s.store.R, r.PathValue("id"))
	if errors.Is(err, site.ErrNotFound) {
		writeError(w, http.StatusNotFound, "site bulunamadı")
		return nil, "", false
	}
	if err != nil {
		internalError(w, "site", err)
		return nil, "", false
	}
	role, err := site.Role(r.Context(), s.store.R, st.ID, u.ID)
	if err != nil {
		internalError(w, "rol", err)
		return nil, "", false
	}
	if role == "" {
		writeError(w, http.StatusNotFound, "site bulunamadı")
		return nil, "", false
	}
	if !site.AtLeast(role, want) {
		writeError(w, http.StatusForbidden, "bu işlem için yetkiniz yok")
		return nil, "", false
	}
	return st, role, true
}

func (s *Server) updateSite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, role, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	var in struct {
		Public *bool `json:"public"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Public != nil {
		if err := site.SetPublic(r.Context(), s.store.W, st.ID, *in.Public); err != nil {
			internalError(w, "site güncelle", err)
			return
		}
		st.Public = *in.Public
	}
	writeJSON(w, http.StatusOK, siteJSON{ID: st.ID, Domain: st.Domain, Public: st.Public, Role: role})
}

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "owner")
	if !ok {
		return
	}
	if err := site.Delete(r.Context(), s.store.W, st.ID); err != nil {
		internalError(w, "site sil", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listGoals(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "viewer")
	if !ok {
		return
	}
	goals, err := site.Goals(r.Context(), s.store.R, st.ID)
	if err != nil {
		internalError(w, "hedefler", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"goals": goals})
}

func (s *Server) addGoal(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	var in struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	g, err := site.AddGoal(r.Context(), s.store.W, st.ID, in.Kind, in.Value)
	switch {
	case errors.Is(err, site.ErrInvalidGoal), errors.Is(err, site.ErrGoalExists), errors.Is(err, site.ErrTooMany):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		internalError(w, "hedef ekle", err)
	default:
		writeJSON(w, http.StatusCreated, g)
	}
}

func (s *Server) deleteGoal(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	found, err := site.DeleteGoal(r.Context(), s.store.W, st.ID, r.PathValue("goal"))
	if err != nil {
		internalError(w, "hedef sil", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "hedef bulunamadı")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listFunnels(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "viewer")
	if !ok {
		return
	}
	fs, err := site.Funnels(r.Context(), s.store.R, st.ID)
	if err != nil {
		internalError(w, "huniler", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"funnels": fs})
}

func (s *Server) addFunnel(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	var in struct {
		Name  string      `json:"name"`
		Steps []site.Step `json:"steps"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	f, err := site.AddFunnel(r.Context(), s.store.W, st.ID, in.Name, in.Steps)
	switch {
	case errors.Is(err, site.ErrInvalidFunnel), errors.Is(err, site.ErrTooManyFunnel):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		internalError(w, "huni ekle", err)
	default:
		writeJSON(w, http.StatusCreated, f)
	}
}

func (s *Server) deleteFunnel(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	found, err := site.DeleteFunnel(r.Context(), s.store.W, st.ID, r.PathValue("funnel"))
	if err != nil {
		internalError(w, "huni sil", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "huni bulunamadı")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
