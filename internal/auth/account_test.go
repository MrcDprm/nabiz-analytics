package auth

import (
	"context"
	"testing"
	"time"
)

// outbox, gönderilen e-postaları yakalayan sahte postacı.
type outbox struct {
	sent []struct{ to, lang, kind, token string }
}

func (o *outbox) send(_ context.Context, to, lang, kind, token string) error {
	o.sent = append(o.sent, struct{ to, lang, kind, token string }{to, lang, kind, token})
	return nil
}

func (o *outbox) last() struct{ to, lang, kind, token string } { return o.sent[len(o.sent)-1] }

func TestSignupAndVerify(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	box := &outbox{}

	if err := s.Signup(ctx, "Yeni@Example.com", "kahve-fincanı-7", "en", box.send); err != nil {
		t.Fatal(err)
	}
	m := box.last()
	if m.to != "yeni@example.com" || m.kind != "verify" || m.lang != "en" || m.token == "" {
		t.Fatalf("e-posta = %+v", m)
	}
	u, _ := s.UserByEmail(ctx, "yeni@example.com")
	if u.Verified || u.Locale != "en" {
		t.Fatalf("kullanıcı = %+v", u)
	}
	if err := s.Verify(ctx, "uydurma"); err != ErrBadToken {
		t.Fatalf("uydurma token: %v", err)
	}
	if err := s.Verify(ctx, m.token); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.UserByEmail(ctx, "yeni@example.com"); !u.Verified {
		t.Fatal("doğrulanmadı")
	}
	if err := s.Verify(ctx, m.token); err != ErrBadToken {
		t.Fatalf("token iki kez kullanıldı: %v", err)
	}

	// Aynı adresle tekrar kayıt: hata yok, ama "zaten var" e-postası gider ve şifre değişmez.
	if err := s.Signup(ctx, "yeni@example.com", "baska-sifre-77", "tr", box.send); err != nil {
		t.Fatal(err)
	}
	if box.last().kind != "already_taken" {
		t.Fatalf("e-posta = %+v", box.last())
	}
	if _, _, err := s.Login(ctx, "yeni@example.com", "kahve-fincanı-7"); err != nil {
		t.Fatal("ikinci kayıt denemesi şifreyi değiştirdi")
	}
}

func TestVerifyTokenExpires(t *testing.T) {
	s, now := newService(t)
	ctx := context.Background()
	box := &outbox{}
	s.Signup(ctx, "a@example.com", "kahve-fincanı-7", "tr", box.send)
	*now = now.Add(VerifyLifetime + time.Minute)
	if err := s.Verify(ctx, box.last().token); err != ErrBadToken {
		t.Fatalf("süresi dolan token: %v", err)
	}
}

func TestPasswordReset(t *testing.T) {
	s, now := newService(t)
	ctx := context.Background()
	box := &outbox{}
	s.CreateUser(ctx, "ali@example.com", "kahve-fincanı-7", true)
	_, oldSession, _ := s.Login(ctx, "ali@example.com", "kahve-fincanı-7")

	// Olmayan adres: hata yok, e-posta yok.
	if err := s.ForgotPassword(ctx, "yok@example.com", box.send); err != nil || len(box.sent) != 0 {
		t.Fatalf("olmayan adres: %v %d", err, len(box.sent))
	}
	s.ForgotPassword(ctx, "ali@example.com", box.send)
	first := box.last().token
	s.ForgotPassword(ctx, "ali@example.com", box.send)
	second := box.last().token
	if err := s.ResetPassword(ctx, first, "yeni-sifre-2026"); err != ErrBadToken {
		t.Fatalf("eski bağlantı çalıştı: %v", err)
	}
	if err := s.ResetPassword(ctx, second, "kisa"); err != ErrPasswordShort {
		t.Fatalf("zayıf şifre: %v", err)
	}
	if err := s.ResetPassword(ctx, second, "yeni-sifre-2026"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, oldSession); err != ErrNoSession {
		t.Fatal("sıfırlamadan sonra eski oturum açık kaldı")
	}
	if _, _, err := s.Login(ctx, "ali@example.com", "yeni-sifre-2026"); err != nil {
		t.Fatal("yeni şifreyle giriş olmadı")
	}

	s.ForgotPassword(ctx, "ali@example.com", box.send)
	*now = now.Add(ResetLifetime + time.Minute)
	if err := s.ResetPassword(ctx, box.last().token, "baska-sifre-77"); err != ErrBadToken {
		t.Fatalf("1 saatten eski bağlantı: %v", err)
	}
}

func TestChangePasswordKeepsCurrentSession(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	s.CreateUser(ctx, "ali@example.com", "kahve-fincanı-7", true)
	u, here, _ := s.Login(ctx, "ali@example.com", "kahve-fincanı-7")
	_, there, _ := s.Login(ctx, "ali@example.com", "kahve-fincanı-7")

	if err := s.ChangePassword(ctx, u, here, "yanlis-sifre-1", "yeni-sifre-2026"); err != ErrWrongPassword {
		t.Fatalf("yanlış mevcut şifre: %v", err)
	}
	if err := s.ChangePassword(ctx, u, here, "kahve-fincanı-7", "yeni-sifre-2026"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, here); err != nil {
		t.Fatal("bu oturum kapandı")
	}
	if _, err := s.UserBySession(ctx, there); err != ErrNoSession {
		t.Fatal("diğer cihazdaki oturum açık kaldı")
	}
}

func TestEmailChange(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	box := &outbox{}
	u, _ := s.CreateUser(ctx, "ali@example.com", "kahve-fincanı-7", true)
	s.CreateUser(ctx, "veli@example.com", "baska-sifre-77", true)

	if err := s.RequestEmailChange(ctx, u, "kahve-fincanı-7", "veli@example.com", box.send); err != nil || len(box.sent) != 0 {
		t.Fatalf("başkasının adresi: %v, %d e-posta", err, len(box.sent))
	}
	if err := s.RequestEmailChange(ctx, u, "kahve-fincanı-7", "ali@example.com", box.send); err != ErrSameEmail {
		t.Fatalf("aynı adres: %v", err)
	}
	if err := s.RequestEmailChange(ctx, u, "yanlis-sifre-1", "yeni@example.com", box.send); err != ErrWrongPassword {
		t.Fatalf("yanlış şifre: %v", err)
	}
	if err := s.RequestEmailChange(ctx, u, "kahve-fincanı-7", "yeni@example.com", box.send); err != nil {
		t.Fatal(err)
	}
	if m := box.last(); m.to != "yeni@example.com" || m.kind != "email_change" {
		t.Fatalf("e-posta = %+v", m)
	}
	// Bağlantı açılana kadar adres değişmez.
	if _, err := s.UserByEmail(ctx, "ali@example.com"); err != nil {
		t.Fatal("adres bağlantıdan önce değişti")
	}
	if err := s.Verify(ctx, box.last().token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Login(ctx, "yeni@example.com", "kahve-fincanı-7"); err != nil {
		t.Fatal("yeni adresle giriş olmadı")
	}
}

func TestDeleteAccount(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "ali@example.com", "kahve-fincanı-7", true)
	v, _ := s.CreateUser(ctx, "veli@example.com", "baska-sifre-77", true)
	exec := func(q string, args ...any) {
		if _, err := s.st.W.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO sites (id, domain, created_at) VALUES ('mine', 'a.com', 0), ('theirs', 'b.com', 0)`)
	exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('mine', ?, 'owner'), ('theirs', ?, 'owner'), ('theirs', ?, 'viewer')`, u.ID, v.ID, u.ID)
	exec(`INSERT INTO sessions (id, site_id, visitor, started_at, ended_at) VALUES (1, 'mine', 1, 0, 0)`)

	if err := s.DeleteAccount(ctx, u, "yanlis-sifre-1", nil); err != ErrWrongPassword {
		t.Fatalf("yanlış şifre: %v", err)
	}
	called := false
	if err := s.DeleteAccount(ctx, u, "kahve-fincanı-7", func(context.Context, int64) error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("abonelik iptal kancası çağrılmadı")
	}
	count := func(q string) int {
		var n int
		s.st.R.QueryRow(q).Scan(&n)
		return n
	}
	if count(`SELECT count(*) FROM users WHERE email = 'ali@example.com'`) != 0 ||
		count(`SELECT count(*) FROM sites WHERE id = 'mine'`) != 0 ||
		count(`SELECT count(*) FROM sessions`) != 0 {
		t.Fatal("hesap ya da verisi kaldı")
	}
	// Başkasının sitesi (sadece izleyiciydi) silinmez.
	if count(`SELECT count(*) FROM sites WHERE id = 'theirs'`) != 1 {
		t.Fatal("başkasının sitesi silindi")
	}
}
