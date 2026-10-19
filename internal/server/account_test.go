package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func addUser(t *testing.T, s *Server) {
	t.Helper()
	if _, err := s.auth.CreateUser(context.Background(), "ali@example.com", "kahve-fincanı-7", true); err != nil {
		t.Fatal(err)
	}
}

func do(s *Server, method, path, body string, cookie *http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "81.214.1.1:5555"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("oturum çerezi yok")
	return nil
}

func TestLoginMeLogout(t *testing.T) {
	s, _ := newTestServer(t)
	addUser(t, s)

	if rec := do(s, "GET", "/api/me", "", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("girişsiz /api/me = %d", rec.Code)
	}

	rec := do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("giriş = %d %s", rec.Code, rec.Body)
	}
	c := sessionFrom(t, rec)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Fatalf("çerez ayarları zayıf: %+v", c)
	}

	if rec := do(s, "GET", "/api/me", "", c, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ali@example.com") {
		t.Fatalf("/api/me = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "POST", "/api/logout", "", c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("çıkış = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/me", "", c, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("çıkıştan sonra /api/me = %d", rec.Code)
	}
}

func TestLoginWrongPasswordGeneric(t *testing.T) {
	s, _ := newTestServer(t)
	addUser(t, s)
	wrong := do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"yanlis-sifre-1"}`, nil, nil)
	missing := do(s, "POST", "/api/login", `{"email":"yok@example.com","password":"yanlis-sifre-1"}`, nil, nil)
	if wrong.Code != http.StatusUnauthorized || missing.Code != http.StatusUnauthorized {
		t.Fatalf("durumlar = %d, %d", wrong.Code, missing.Code)
	}
	if wrong.Body.String() != missing.Body.String() {
		t.Fatal("yanlış şifre ile olmayan kullanıcı farklı mesaj veriyor")
	}
}

func TestLoginRejectsUnknownFields(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(s, "POST", "/api/login", `{"email":"a@b.com","password":"x","admin":true}`, nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bilinmeyen alan: %d", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	s, _ := newTestServer(t)
	now := time.Now()
	s.loginLimit = newLimiter(5.0/60, 5, func() time.Time { return now })
	for range 5 {
		do(s, "POST", "/api/login", `{"email":"a@b.com","password":"yanlis-sifre-1"}`, nil, nil)
	}
	rec := do(s, "POST", "/api/login", `{"email":"a@b.com","password":"yanlis-sifre-1"}`, nil, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6. deneme = %d, 429 olmalı", rec.Code)
	}
}

func TestCrossSiteLoginBlocked(t *testing.T) {
	s, _ := newTestServer(t)
	addUser(t, s)
	// Başka bir siteden gönderilen form (CSRF) reddedilmeli.
	rec := do(s, "POST", "/api/logout", "", nil, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://kotu.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("siteler arası istek = %d, 403 olmalı", rec.Code)
	}
	// Takip ucu bilerek başka sitelerden çağrılır; CSRF korumasına takılmamalı.
	rec = do(s, "POST", "/api/event", validBody, nil, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://a.com", "User-Agent": chromeUA})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("takip ucu = %d, 202 olmalı", rec.Code)
	}
}
