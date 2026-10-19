package server

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

type sentMail struct{ to, subject, body string }

// chanMail, arka planda gönderilen e-postaları teste iletir.
type chanMail chan sentMail

func (c chanMail) Send(_ context.Context, to, subject, body string) error {
	c <- sentMail{to, subject, body}
	return nil
}

func waitMail(t *testing.T, c chanMail) sentMail {
	t.Helper()
	select {
	case m := <-c:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("e-posta gelmedi")
		return sentMail{}
	}
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

func mailServer(t *testing.T, signup bool) (*Server, chanMail) {
	t.Helper()
	s, _ := newTestServer(t)
	box := make(chanMail, 10)
	s.cfg.Mail = box
	s.cfg.BaseURL = "https://nabiz.example.com"
	s.cfg.Signup = signup
	return s, box
}

func TestSignupFlow(t *testing.T) {
	s, box := mailServer(t, true)
	// Host başlığı sahte olsa bile bağlantı ayarlardaki adresi kullanmalı.
	rec := do(s, "POST", "/api/signup", `{"email":"yeni@example.com","password":"kahve-fincanı-7","locale":"tr"}`, nil, map[string]string{"Host": "kotu.com"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("kayıt = %d %s", rec.Code, rec.Body)
	}
	m := waitMail(t, box)
	if m.to != "yeni@example.com" || !strings.Contains(m.body, "https://nabiz.example.com/verify?token=") || strings.Contains(m.body, "kotu.com") {
		t.Fatalf("e-posta = %+v", m)
	}
	token := tokenRe.FindStringSubmatch(m.body)[1]

	// Doğrulanmadan site eklenemez.
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"yeni@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	if rec := do(s, "POST", "/api/sites", `{"domain":"x.com"}`, c, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("doğrulanmamış site ekledi: %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/verify", `{"token":"`+token+`"}`, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("doğrula = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "POST", "/api/sites", `{"domain":"x.com"}`, c, nil); rec.Code != http.StatusCreated {
		t.Fatalf("doğrulandıktan sonra site = %d", rec.Code)
	}

	// Kayıtlı adresle tekrar kayıt: aynı cevap, farklı e-posta.
	rec = do(s, "POST", "/api/signup", `{"email":"yeni@example.com","password":"baska-sifre-77"}`, nil, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("tekrar kayıt = %d", rec.Code)
	}
	if m := waitMail(t, box); !strings.Contains(m.body, "/login") || strings.Contains(m.body, "token=") {
		t.Fatalf("zaten var e-postası = %+v", m)
	}
}

func TestSignupClosed(t *testing.T) {
	s, _ := mailServer(t, false)
	if rec := do(s, "POST", "/api/signup", `{"email":"a@example.com","password":"kahve-fincanı-7"}`, nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("kapalı kayıt = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/config", "", nil, nil); !strings.Contains(rec.Body.String(), `"signup":false`) {
		t.Fatalf("config = %s", rec.Body)
	}
}

func TestSignupWeakPassword(t *testing.T) {
	s, _ := mailServer(t, true)
	rec := do(s, "POST", "/api/signup", `{"email":"a@example.com","password":"12345"}`, nil, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "en az 10") {
		t.Fatalf("zayıf şifre = %d %s", rec.Code, rec.Body)
	}
}

func TestForgotAndReset(t *testing.T) {
	s, box := mailServer(t, false)
	addUser(t, s)
	// Olmayan adres de aynı cevabı alır.
	if rec := do(s, "POST", "/api/password/forgot", `{"email":"yok@example.com"}`, nil, nil); rec.Code != http.StatusAccepted {
		t.Fatalf("olmayan = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/password/forgot", `{"email":"ali@example.com"}`, nil, nil); rec.Code != http.StatusAccepted {
		t.Fatalf("unuttum = %d", rec.Code)
	}
	m := waitMail(t, box)
	if !strings.Contains(m.body, "https://nabiz.example.com/reset?token=") {
		t.Fatalf("e-posta = %+v", m)
	}
	token := tokenRe.FindStringSubmatch(m.body)[1]
	if rec := do(s, "POST", "/api/password/reset", `{"token":"`+token+`","password":"yeni-sifre-2026"}`, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("sıfırla = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "POST", "/api/password/reset", `{"token":"`+token+`","password":"yeni-sifre-2027"}`, nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("aynı bağlantı ikinci kez = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"yeni-sifre-2026"}`, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("yeni şifreyle giriş = %d", rec.Code)
	}
}

func TestMailRateLimit(t *testing.T) {
	s, _ := mailServer(t, false)
	for range 5 {
		do(s, "POST", "/api/password/forgot", `{"email":"yok@example.com"}`, nil, nil)
	}
	if rec := do(s, "POST", "/api/password/forgot", `{"email":"yok@example.com"}`, nil, nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6. istek = %d, 429 olmalı", rec.Code)
	}
}

func TestAccountEndpoints(t *testing.T) {
	s, box := mailServer(t, false)
	addUser(t, s)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil))

	if rec := do(s, "POST", "/api/account/password", `{"current":"yanlis-sifre-1","new":"yeni-sifre-2026"}`, c, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("yanlış mevcut şifre = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/account/password", `{"current":"kahve-fincanı-7","new":"yeni-sifre-2026"}`, c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("şifre değiştir = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "GET", "/api/me", "", c, nil); rec.Code != http.StatusOK {
		t.Fatal("şifre değişince bu oturum kapandı")
	}
	if rec := do(s, "POST", "/api/account/email", `{"password":"yeni-sifre-2026","email":"ali2@example.com"}`, c, nil); rec.Code != http.StatusAccepted {
		t.Fatalf("e-posta değiştir = %d %s", rec.Code, rec.Body)
	}
	if m := waitMail(t, box); m.to != "ali2@example.com" {
		t.Fatalf("e-posta yeni adrese gitmedi: %+v", m)
	}
	if rec := do(s, "PATCH", "/api/account", `{"locale":"en"}`, c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("dil = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/account", `{"password":"yanlis-sifre-1"}`, c, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("yanlış şifreyle sil = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/account", `{"password":"yeni-sifre-2026"}`, c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hesap sil = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "GET", "/api/me", "", c, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("silinen hesap = %d", rec.Code)
	}
}
