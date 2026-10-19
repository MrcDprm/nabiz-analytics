package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/mail"
	"github.com/MrcDprm/nabiz-analytics/internal/team"
)

func teamError(w http.ResponseWriter, where string, err error) {
	switch {
	case errors.Is(err, team.ErrBadRole), errors.Is(err, team.ErrAlreadyIn), errors.Is(err, team.ErrOwner),
		errors.Is(err, team.ErrSelf), errors.Is(err, team.ErrBadInvite), errors.Is(err, team.ErrWrongAccount),
		errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, team.ErrNotMember):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		accountError(w, where, err)
	}
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "viewer")
	if !ok {
		return
	}
	members, invites, err := s.team.Members(r.Context(), st.ID)
	if err != nil {
		internalError(w, "üyeler", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members, "invites": invites})
}

var roleNames = map[string]map[string]string{
	"tr": {"admin": "yönetici", "viewer": "izleyici"},
	"en": {"admin": "admin", "viewer": "viewer"},
}

func (s *Server) invite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	if s.tooMany(w, r, s.mailLimit) {
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	email, err := auth.NormalizeEmail(in.Email)
	if err != nil {
		teamError(w, "davet", err)
		return
	}
	token, err := s.team.CreateInvite(r.Context(), st.ID, email, in.Role, u.ID)
	if err != nil {
		teamError(w, "davet", err)
		return
	}
	// Davet e-postası davet edenin dilinde gider (alıcının dilini henüz bilmiyoruz).
	lang := u.Locale
	if roleNames[lang] == nil {
		lang = "en"
	}
	subject, body := mail.Render(lang, mail.Invite, s.cfg.BaseURL+"/invite?token="+token, st.Domain, roleNames[lang][in.Role])
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := s.cfg.Mail.Send(ctx, email, subject, body); err != nil {
			log.Printf("davet e-postası gönderilemedi: %v", err)
		}
	}()
	writeJSON(w, http.StatusCreated, map[string]bool{"sent": true})
}

func (s *Server) revokeInvite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	found, err := s.team.RevokeInvite(r.Context(), st.ID, r.PathValue("email"))
	if err != nil {
		internalError(w, "davet iptal", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "davet bulunamadı")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setMemberRole(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "admin")
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.team.SetRole(r.Context(), st.ID, r.PathValue("email"), in.Role); err != nil {
		teamError(w, "rol", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeMember: yönetici ve sahip başkasını çıkarabilir; herkes kendisi ayrılabilir.
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, u *auth.User) {
	email := r.PathValue("email")
	want := "admin"
	if email == u.Email {
		want = "viewer"
	}
	st, _, ok := s.siteFor(w, r, u, want)
	if !ok {
		return
	}
	if err := s.team.Remove(r.Context(), st.ID, email); err != nil {
		teamError(w, "üye çıkar", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) transferSite(w http.ResponseWriter, r *http.Request, u *auth.User) {
	st, _, ok := s.siteFor(w, r, u, "owner")
	if !ok {
		return
	}
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	// Geri alınamaz bir işlem: şifreyle onay.
	if err := s.auth.ConfirmPassword(r.Context(), u.ID, in.Password); err != nil {
		accountError(w, "devir", err)
		return
	}
	if err := s.team.Transfer(r.Context(), st.ID, u.ID, in.Email); err != nil {
		teamError(w, "devir", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) inviteInfo(w http.ResponseWriter, r *http.Request) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	info, err := s.team.Info(r.Context(), in.Token)
	if err != nil {
		teamError(w, "davet bilgisi", err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// acceptInvite: giriş yapmış kullanıcı daveti kabul eder. Hesabı olmayan biri
// password vererek hesap açar; adres davet bağlantısıyla kanıtlandığı için
// hesap doğrulanmış açılır ve herkese açık kayıt kapalı olsa da çalışır.
func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	u, err := s.currentUser(r)
	if err != nil {
		internalError(w, "davet kabul", err)
		return
	}
	if u == nil {
		info, err := s.team.Info(ctx, in.Token)
		if err != nil {
			teamError(w, "davet kabul", err)
			return
		}
		if info.HasAccount {
			writeError(w, http.StatusUnauthorized, "giriş yapmanız gerekiyor")
			return
		}
		if u, err = s.auth.CreateUser(ctx, info.Email, in.Password, true); err != nil {
			accountError(w, "davetle hesap", err)
			return
		}
		token, err := s.auth.StartSession(ctx, u.ID)
		if err != nil {
			internalError(w, "davetle oturum", err)
			return
		}
		setSessionCookie(w, token)
	}
	info, err := s.team.Accept(ctx, in.Token, u.ID, u.Email)
	if err != nil {
		teamError(w, "davet kabul", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"domain": info.Domain})
}
