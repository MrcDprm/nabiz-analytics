package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
)

// "__Host-" öneki tarayıcıya şunu zorunlu kılar: çerez sadece HTTPS'te, sadece
// bu alan adında (alt alan adları göremez) ve bütün yollarda geçerli.
const sessionCookie = "__Host-nabiz"

// readJSON gövdeyi sınırlı boyutta okur; bilinmeyen alan varsa reddeder
// (sadece izin verilen alanlar kabul edilir).
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "istek okunamadı")
		return false
	}
	return true
}

// writeError kullanıcıya gösterilebilecek kısa bir mesaj döndürür; iç ayrıntı
// (SQL hatası, dosya yolu...) asla dışarı çıkmaz.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func internalError(w http.ResponseWriter, where string, err error) {
	log.Printf("%s: %v", where, err)
	writeError(w, http.StatusInternalServerError, "beklenmeyen bir hata oldu, biraz sonra tekrar deneyin")
}

type userJSON struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Locale   string `json:"locale"`
}

func toJSON(u *auth.User) userJSON {
	return userJSON{Email: u.Email, Verified: u.Verified, Locale: u.Locale}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimit.allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "çok fazla deneme yaptınız, bir dakika sonra tekrar deneyin")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	u, token, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if errors.Is(err, auth.ErrInvalidLogin) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		internalError(w, "login", err)
		return
	}
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, toJSON(u))
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(auth.SessionLifetime),
		HttpOnly: true, // JavaScript okuyamaz: XSS olsa bile oturum çalınamaz
		Secure:   true,
		SameSite: http.SameSiteStrictMode, // başka sitelerden gelen isteklere eklenmez
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			internalError(w, "logout", err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

// currentUser çerezdeki oturumun sahibini döndürür; yoksa nil.
func (s *Server) currentUser(r *http.Request) (*auth.User, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, nil
	}
	u, err := s.auth.UserBySession(r.Context(), c.Value)
	if errors.Is(err, auth.ErrNoSession) {
		return nil, nil
	}
	return u, err
}

// requireUser, giriş gerektiren uçları sarar.
func (s *Server) requireUser(next func(http.ResponseWriter, *http.Request, *auth.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.currentUser(r)
		if err != nil {
			internalError(w, "oturum", err)
			return
		}
		if u == nil {
			writeError(w, http.StatusUnauthorized, "giriş yapmanız gerekiyor")
			return
		}
		next(w, r, u)
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, u *auth.User) {
	writeJSON(w, http.StatusOK, toJSON(u))
}
