package site

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"example.com":                     "example.com",
		"  https://www.Example.com/ ":     "example.com",
		"http://quiz.miracdeprem.com/oda": "quiz.miracdeprem.com",
	}
	for in, want := range cases {
		if got, err := NormalizeDomain(in); err != nil || got != want {
			t.Errorf("%q → %q (%v), beklenen %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "localhost", "a_b.com", "-a.com", "a..com", "ex ample.com"} {
		if _, err := NormalizeDomain(bad); err == nil {
			t.Errorf("%q kabul edildi", bad)
		}
	}
}

func TestCreate(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.W.Exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'a@b.com', 'x', 0)`)

	s, err := Create(context.Background(), st, "https://www.example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(s.ID) {
		t.Fatalf("UUID biçimi yanlış: %s", s.ID)
	}
	var role string
	st.R.QueryRow(`SELECT role FROM memberships WHERE site_id = ? AND user_id = 1`, s.ID).Scan(&role)
	if role != "owner" {
		t.Fatalf("rol = %q, owner olmalı", role)
	}
	if _, err := Create(context.Background(), st, "example.com", 1); err != ErrDomainTaken {
		t.Fatalf("aynı site: %v", err)
	}
	// Olmayan kullanıcıya site açılamaz; yarım kalan site satırı da kalmamalı.
	if _, err := Create(context.Background(), st, "other.com", 99); err == nil {
		t.Fatal("olmayan kullanıcıya site açıldı")
	}
	var n int
	st.R.QueryRow(`SELECT count(*) FROM sites WHERE domain = 'other.com'`).Scan(&n)
	if n != 0 {
		t.Fatal("başarısız işlemden site satırı kaldı")
	}
}
