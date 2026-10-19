package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
	"github.com/MrcDprm/nabiz-analytics/internal/team"
)

// teamSetup: ali, a.com'un sahibi; e-postalar kutuya düşer.
func teamSetup(t *testing.T) (*Server, chanMail, *http.Cookie, string) {
	t.Helper()
	s, box := mailServer(t, false)
	addUser(t, s)
	u, _ := s.auth.UserByEmail(context.Background(), "ali@example.com")
	st, err := site.Create(context.Background(), s.store, "a.com", u.ID)
	if err != nil {
		t.Fatal(err)
	}
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	return s, box, c, st.ID
}

func inviteToken(t *testing.T, s *Server, box chanMail, c *http.Cookie, id, email, role string) string {
	t.Helper()
	rec := do(s, "POST", "/api/sites/"+id+"/invites", `{"email":"`+email+`","role":"`+role+`"}`, c, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("davet = %d %s", rec.Code, rec.Body)
	}
	m := waitMail(t, box)
	if m.to != email || !strings.Contains(m.body, "a.com") {
		t.Fatalf("davet e-postası = %+v", m)
	}
	return tokenRe.FindStringSubmatch(m.body)[1]
}

func TestInviteNewUserCreatesAccount(t *testing.T) {
	s, box, owner, id := teamSetup(t)
	token := inviteToken(t, s, box, owner, id, "yeni@example.com", "viewer")

	rec := do(s, "POST", "/api/invites/info", `{"token":"`+token+`"}`, nil, nil)
	var info team.InviteInfo
	json.NewDecoder(rec.Body).Decode(&info)
	if info.Domain != "a.com" || info.Role != "viewer" || info.HasAccount {
		t.Fatalf("bilgi = %+v", info)
	}
	// Kayıt kapalı olsa da davetle hesap açılır; zayıf şifre yine reddedilir.
	if rec := do(s, "POST", "/api/invites/accept", `{"token":"`+token+`","password":"kisa"}`, nil, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("zayıf şifre = %d", rec.Code)
	}
	rec = do(s, "POST", "/api/invites/accept", `{"token":"`+token+`","password":"deniz-feneri-42"}`, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("kabul = %d %s", rec.Code, rec.Body)
	}
	c := sessionFrom(t, rec)
	if rec := do(s, "GET", "/api/stats/a.com/summary", "", c, nil); rec.Code != http.StatusOK {
		t.Fatalf("izleyici paneli göremiyor: %d", rec.Code)
	}
	// İzleyici davet gönderemez, ayar değiştiremez.
	if rec := do(s, "POST", "/api/sites/"+id+"/invites", `{"email":"x@example.com","role":"viewer"}`, c, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("izleyici davet etti: %d", rec.Code)
	}
	// Hesap doğrulanmış açıldı.
	if rec := do(s, "GET", "/api/me", "", c, nil); !strings.Contains(rec.Body.String(), `"verified":true`) {
		t.Fatalf("me = %s", rec.Body)
	}
}

func TestInviteExistingUser(t *testing.T) {
	s, box, owner, id := teamSetup(t)
	s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true)
	token := inviteToken(t, s, box, owner, id, "veli@example.com", "admin")

	// Hesabı olan biri girişsiz kabul edemez (şifre vererek başkasının hesabını ele geçiremez).
	if rec := do(s, "POST", "/api/invites/accept", `{"token":"`+token+`","password":"herhangi-1234"}`, nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("girişsiz = %d", rec.Code)
	}
	// Başka hesapla kabul edilemez.
	if rec := do(s, "POST", "/api/invites/accept", `{"token":"`+token+`"}`, owner, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("yanlış hesap = %d", rec.Code)
	}
	veli := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil))
	if rec := do(s, "POST", "/api/invites/accept", `{"token":"`+token+`"}`, veli, nil); rec.Code != http.StatusOK {
		t.Fatalf("kabul = %d %s", rec.Code, rec.Body)
	}

	// Yönetici davet edebilir ama sahipliği devredemez, sahibi çıkaramaz.
	if rec := do(s, "POST", "/api/sites/"+id+"/invites", `{"email":"x@example.com","role":"viewer"}`, veli, nil); rec.Code != http.StatusCreated {
		t.Fatalf("yönetici davet = %d", rec.Code)
	}
	waitMail(t, box)
	if rec := do(s, "POST", "/api/sites/"+id+"/transfer", `{"email":"veli@example.com","password":"baska-sifre-77"}`, veli, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("yönetici devir = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id+"/members/ali@example.com", "", veli, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("sahibi çıkar = %d", rec.Code)
	}

	// Liste: 2 üye, 1 bekleyen davet; davet iptal edilebilir.
	rec := do(s, "GET", "/api/sites/"+id+"/members", "", owner, nil)
	var body struct {
		Members []team.Member
		Invites []team.Invite
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body.Members) != 2 || len(body.Invites) != 1 {
		t.Fatalf("liste = %+v", body)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id+"/invites/x@example.com", "", owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("davet iptal = %d", rec.Code)
	}

	// Sahip devreder (şifreyle); sonra eski sahip ayrılabilir.
	if rec := do(s, "POST", "/api/sites/"+id+"/transfer", `{"email":"veli@example.com","password":"yanlis-sifre-1"}`, owner, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("yanlış şifreyle devir = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/sites/"+id+"/transfer", `{"email":"veli@example.com","password":"kahve-fincanı-7"}`, owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("devir = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id+"/members/ali@example.com", "", owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("ayrıl = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "GET", "/api/stats/a.com/summary", "", owner, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("ayrıldıktan sonra panel = %d", rec.Code)
	}
}

func TestRoleChange(t *testing.T) {
	s, box, owner, id := teamSetup(t)
	s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true)
	token := inviteToken(t, s, box, owner, id, "veli@example.com", "viewer")
	veli := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil))
	do(s, "POST", "/api/invites/accept", `{"token":"`+token+`"}`, veli, nil)

	if rec := do(s, "PATCH", "/api/sites/"+id+"/members/veli@example.com", `{"role":"owner"}`, owner, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("rol=owner = %d", rec.Code)
	}
	if rec := do(s, "PATCH", "/api/sites/"+id+"/members/veli@example.com", `{"role":"admin"}`, owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("rol = %d", rec.Code)
	}
	if rec := do(s, "PATCH", "/api/sites/"+id+"/members/yok@example.com", `{"role":"admin"}`, owner, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("üye olmayan = %d", rec.Code)
	}
}
