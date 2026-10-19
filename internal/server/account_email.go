package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/mail"
)

// sendMail, auth paketinin istediği e-postayı hazırlar ve arka planda gönderir.
// Bağlantılar her zaman ayarlardaki temel adresten üretilir, isteğin Host
// başlığından asla: saldırgan Host'u değiştirip sıfırlama bağlantısını kendi
// sitesine yönlendiremesin. Gönderim beklenmez; böylece cevap süresi "bu adres
// kayıtlı mı" bilgisini sızdırmaz ve yavaş bir SMTP sunucusu kullanıcıyı bekletmez.
func (s *Server) sendMail(_ context.Context, to, lang, kind, token string) error {
	link := s.cfg.BaseURL + "/login"
	switch kind {
	case "verify", "email_change":
		link = s.cfg.BaseURL + "/verify?token=" + token
	case "reset":
		link = s.cfg.BaseURL + "/reset?token=" + token
	}
	subject, body := mail.Render(lang, mail.Kind(kind), link)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := s.cfg.Mail.Send(ctx, to, subject, body); err != nil {
			log.Printf("e-posta gönderilemedi (%s): %v", kind, err) // adres log'a yazılmaz
		}
	}()
	return nil
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"signup": s.cfg.Signup, "mode": s.cfg.Mode})
}

// accountError, kullanıcının düzeltebileceği hataları 400/403'e, gerisini 500'e çevirir.
func accountError(w http.ResponseWriter, where string, err error) {
	switch {
	case errors.Is(err, auth.ErrPasswordShort), errors.Is(err, auth.ErrPasswordLong), errors.Is(err, auth.ErrPasswordSimple),
		errors.Is(err, auth.ErrPasswordCommon), errors.Is(err, auth.ErrPasswordEmail), errors.Is(err, auth.ErrInvalidEmail),
		errors.Is(err, auth.ErrBadToken), errors.Is(err, auth.ErrSameEmail), errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrWrongPassword):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		internalError(w, where, err)
	}
}

func (s *Server) tooMany(w http.ResponseWriter, r *http.Request, l *limiter) bool {
	if l.allow(s.clientIP(r)) {
		return false
	}
	writeError(w, http.StatusTooManyRequests, "çok fazla deneme yaptınız, biraz sonra tekrar deneyin")
	return true
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Signup {
		writeError(w, http.StatusNotFound, "kayıt kapalı")
		return
	}
	if s.tooMany(w, r, s.mailLimit) {
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Locale   string `json:"locale"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.Signup(r.Context(), in.Email, in.Password, in.Locale, s.sendMail); err != nil {
		accountError(w, "kayıt", err)
		return
	}
	// Adres yeni de olsa kayıtlı da olsa aynı cevap.
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.Verify(r.Context(), in.Token); err != nil {
		accountError(w, "doğrulama", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resendVerification(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if u.Verified {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.tooMany(w, r, s.mailLimit) {
		return
	}
	if err := s.auth.SendVerification(r.Context(), u.ID, u.Email, u.Locale, s.sendMail); err != nil {
		internalError(w, "doğrulama tekrar", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if s.tooMany(w, r, s.mailLimit) {
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.ForgotPassword(r.Context(), in.Email, s.sendMail); err != nil {
		internalError(w, "şifremi unuttum", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
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
	if err := s.auth.ResetPassword(r.Context(), in.Token, in.Password); err != nil {
		accountError(w, "şifre sıfırlama", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var in struct {
		Locale string `json:"locale"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.SetLocale(r.Context(), u.ID, in.Locale); err != nil {
		internalError(w, "hesap", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	c, _ := r.Cookie(sessionCookie) // requireUser'dan geçtiyse çerez var
	if err := s.auth.ChangePassword(r.Context(), u, c.Value, in.Current, in.New); err != nil {
		accountError(w, "şifre değiştir", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changeEmail(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.RequestEmailChange(r.Context(), u, in.Password, in.Email, s.sendMail); err != nil {
		accountError(w, "e-posta değiştir", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"sent": true})
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.auth.DeleteAccount(r.Context(), u, in.Password, s.onAccountDelete); err != nil {
		accountError(w, "hesap sil", err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

// onAccountDelete, hesap silinmeden önce aktif aboneliği iptal eder (bir daha çekim olmasın).
func (s *Server) onAccountDelete(ctx context.Context, userID int64) error {
	if s.cfg.Billing == nil {
		return nil
	}
	return s.cfg.Billing.OnAccountDelete(ctx, userID)
}
