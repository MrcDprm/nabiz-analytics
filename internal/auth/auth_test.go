package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("doğru-şifre-42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=2,p=1$") {
		t.Fatalf("özet biçimi: %s", h)
	}
	if ok, err := VerifyPassword("doğru-şifre-42", h); !ok || err != nil {
		t.Fatalf("doğru şifre reddedildi: %v", err)
	}
	if ok, _ := VerifyPassword("yanlış-şifre-42", h); ok {
		t.Fatal("yanlış şifre kabul edildi")
	}
	h2, _ := HashPassword("doğru-şifre-42")
	if h == h2 {
		t.Fatal("aynı şifre aynı özeti verdi; tuz çalışmıyor")
	}
}

func TestVerifyRejectsBadHashes(t *testing.T) {
	for _, h := range []string{
		"",
		"düz-metin",
		"$argon2i$v=19$m=65536,t=2,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=99999999,t=2,p=1$c2FsdA$a2V5", // dev bellek isteği
		"$argon2id$v=19$m=65536,t=2,p=1$!!!$a2V5",
	} {
		if _, err := VerifyPassword("x", h); err == nil {
			t.Errorf("bozuk özet kabul edildi: %q", h)
		}
	}
}

func TestCheckPassword(t *testing.T) {
	cases := map[string]error{
		"kisa1":                  ErrPasswordShort,
		"sadeceharfler":          ErrPasswordSimple,
		"12345678901":            ErrPasswordSimple,
		"Galatasaray1905":        ErrPasswordCommon,
		"miracdeprem99":          ErrPasswordEmail,
		strings.Repeat("a1", 65): ErrPasswordLong,
		"kahve-fincanı-7":        nil,
		"Ağaç9Gölge3Rüzgar":      nil,
	}
	for pw, want := range cases {
		if got := CheckPassword(pw, "miracdeprem@example.com"); got != want {
			t.Errorf("%q → %v, beklenen %v", pw, got, want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if e, err := NormalizeEmail("  Ali@Example.COM "); err != nil || e != "ali@example.com" {
		t.Fatalf("→ %q, %v", e, err)
	}
	for _, bad := range []string{"", "ali", "ali@", "Ali <ali@example.com>", "ali@localhost", "a@b@c.com"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Errorf("%q kabul edildi", bad)
		}
	}
}

func newService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	s, err := NewService(st, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return s, &now
}

func TestLoginFlow(t *testing.T) {
	s, now := newService(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "Ali@Example.com", "kahve-fincanı-7", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "ali@example.com", "baska-sifre-77", true); err != ErrEmailTaken {
		t.Fatalf("aynı e-posta: %v", err)
	}

	if _, _, err := s.Login(ctx, "ali@example.com", "yanlis-sifre-1"); err != ErrInvalidLogin {
		t.Fatalf("yanlış şifre: %v", err)
	}
	if _, _, err := s.Login(ctx, "yok@example.com", "kahve-fincanı-7"); err != ErrInvalidLogin {
		t.Fatalf("olmayan kullanıcı: %v", err)
	}

	u, token, err := s.Login(ctx, "ALI@example.com", "kahve-fincanı-7")
	if err != nil || u.Email != "ali@example.com" || !u.Verified {
		t.Fatalf("giriş: %+v %v", u, err)
	}
	if got, err := s.UserBySession(ctx, token); err != nil || got.ID != u.ID {
		t.Fatalf("oturum: %+v %v", got, err)
	}

	// Veritabanında token'ın kendisi değil, sadece özeti durmalı.
	var stored []byte
	s.st.R.QueryRow(`SELECT token_hash FROM auth_sessions`).Scan(&stored)
	if string(stored) == token {
		t.Fatal("token düz metin saklandı")
	}

	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, token); err != ErrNoSession {
		t.Fatalf("çıkıştan sonra oturum: %v", err)
	}

	_, token, _ = s.Login(ctx, "ali@example.com", "kahve-fincanı-7")
	*now = now.Add(SessionLifetime + time.Second)
	if _, err := s.UserBySession(ctx, token); err != ErrNoSession {
		t.Fatalf("süresi dolan oturum: %v", err)
	}
	s.Sweep(ctx)
	var n int
	s.st.R.QueryRow(`SELECT count(*) FROM auth_sessions`).Scan(&n)
	if n != 0 {
		t.Fatalf("temizlik sonrası %d oturum kaldı", n)
	}
}

func TestCreateUserRejectsWeak(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.CreateUser(context.Background(), "ali@example.com", "kisa", true); err != ErrPasswordShort {
		t.Fatalf("zayıf şifre: %v", err)
	}
	if _, err := s.CreateUser(context.Background(), "gecersiz", "kahve-fincanı-7", true); err != ErrInvalidEmail {
		t.Fatalf("geçersiz e-posta: %v", err)
	}
}
