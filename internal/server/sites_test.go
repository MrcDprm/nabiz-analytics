package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

func siteID(t *testing.T, s *Server, domain string) string {
	t.Helper()
	st, err := site.ByDomain(context.Background(), s.store.R, domain)
	if err != nil {
		t.Fatal(err)
	}
	return st.ID
}

func TestCreateSite(t *testing.T) {
	s, c := statsSetup(t)
	rec := do(s, "POST", "/api/sites", `{"domain":"https://www.Yeni-Site.com/"}`, c, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ekle = %d %s", rec.Code, rec.Body)
	}
	var got siteJSON
	json.NewDecoder(rec.Body).Decode(&got)
	if got.Domain != "yeni-site.com" || got.Role != "owner" {
		t.Fatalf("site = %+v", got)
	}
	for _, body := range []string{`{"domain":"yeni-site.com"}`, `{"domain":"localhost"}`} {
		if rec := do(s, "POST", "/api/sites", body, c, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", body, rec.Code)
		}
	}
}

func TestUnverifiedCannotCreateSite(t *testing.T) {
	s, _ := newTestServer(t)
	s.auth.CreateUser(context.Background(), "yeni@example.com", "kahve-fincanı-7", false)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"yeni@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	if rec := do(s, "POST", "/api/sites", `{"domain":"x.com"}`, c, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("doğrulanmamış = %d", rec.Code)
	}
}

func TestSiteSettingsAndRoles(t *testing.T) {
	s, owner := statsSetup(t)
	id := siteID(t, s, "a.com")

	// Bir izleyici ekle.
	s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true)
	u, _ := s.auth.UserByEmail(context.Background(), "veli@example.com")
	s.store.W.Exec(`INSERT INTO memberships (site_id, user_id, role) VALUES (?, ?, 'viewer')`, id, u.ID)
	viewer := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil))

	if rec := do(s, "PATCH", "/api/sites/"+id, `{"public":true}`, viewer, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("izleyici ayar değiştirdi: %d", rec.Code)
	}
	if rec := do(s, "PATCH", "/api/sites/"+id, `{"public":true}`, owner, nil); rec.Code != http.StatusOK {
		t.Fatalf("sahip ayar = %d", rec.Code)
	}
	// Herkese açık oldu: girişsiz görülebilir.
	if rec := do(s, "GET", "/api/stats/a.com/summary", "", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("herkese açık = %d", rec.Code)
	}

	// Hedefler: izleyici görür ama ekleyemez.
	if rec := do(s, "POST", "/api/sites/"+id+"/goals", `{"kind":"page","value":"/iletisim"}`, viewer, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("izleyici hedef ekledi: %d", rec.Code)
	}
	rec := do(s, "POST", "/api/sites/"+id+"/goals", `{"kind":"page","value":"/iletisim"}`, owner, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("hedef ekle = %d %s", rec.Code, rec.Body)
	}
	var g site.Goal
	json.NewDecoder(rec.Body).Decode(&g)
	if rec := do(s, "GET", "/api/sites/"+id+"/goals", "", viewer, nil); rec.Code != http.StatusOK {
		t.Fatalf("izleyici hedef listesi = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/stats/a.com/goals", "", owner, nil); rec.Code != http.StatusOK {
		t.Fatalf("hedef istatistiği = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id+"/goals/"+g.ID, "", owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hedef sil = %d", rec.Code)
	}

	// Silme sadece sahibin.
	if rec := do(s, "DELETE", "/api/sites/"+id, "", viewer, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("izleyici sildi: %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id, "", owner, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("sahip sil = %d", rec.Code)
	}
	var n int
	s.store.R.QueryRow(`SELECT count(*) FROM events`).Scan(&n)
	if n != 0 {
		t.Fatalf("site silindi ama %d olay kaldı", n)
	}
}

func TestOtherUsersSiteIs404(t *testing.T) {
	s, _ := statsSetup(t)
	id := siteID(t, s, "a.com")
	s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil))
	for _, r := range [][2]string{{"PATCH", "/api/sites/" + id}, {"DELETE", "/api/sites/" + id}, {"GET", "/api/sites/" + id + "/goals"}} {
		body := ""
		if r[0] == "PATCH" {
			body = `{"public":true}`
		}
		if rec := do(s, r[0], r[1], body, c, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, 404 olmalı", r[0], r[1], rec.Code)
		}
	}
}

func TestFunnelEndpoints(t *testing.T) {
	s, c := statsSetup(t)
	id := siteID(t, s, "a.com")
	rec := do(s, "POST", "/api/sites/"+id+"/funnels", `{"name":"Kayıt","steps":[{"kind":"page","value":"/"},{"kind":"event","value":"Kayıt"}]}`, c, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("huni ekle = %d %s", rec.Code, rec.Body)
	}
	rec = do(s, "GET", "/api/stats/a.com/funnels?period=day", "", c, nil)
	var body struct {
		Funnels  []site.Funnel `json:"funnels"`
		Selected string        `json:"selected"`
		Steps    []struct{ Visitors int64 }
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if rec.Code != 200 || len(body.Funnels) != 1 || body.Selected != body.Funnels[0].ID || len(body.Steps) != 2 || body.Steps[0].Visitors != 1 {
		t.Fatalf("huni istatistiği = %d %+v", rec.Code, body)
	}
	if rec := do(s, "GET", "/api/stats/a.com/props?event=Kayıt", "", c, nil); rec.Code != 200 {
		t.Fatalf("özellikler = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/stats/a.com/props", "", c, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("olaysız özellik = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/sites/"+id+"/funnels/"+body.Funnels[0].ID, "", c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("huni sil = %d", rec.Code)
	}
}
