package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

// statsSetup: ali'nin a.com sitesi var, içine bir ziyaret yazılmış; b.com herkese açık.
func statsSetup(t *testing.T) (*Server, *http.Cookie) {
	t.Helper()
	s, st := newTestServer(t)
	addUser(t, s)
	u, _ := s.auth.UserByEmail(context.Background(), "ali@example.com")
	if _, err := site.Create(context.Background(), st, "a.com", u.ID); err != nil {
		t.Fatal(err)
	}
	if rec := post(s, validBody, "https://a.com", chromeUA); rec.Code != http.StatusAccepted {
		t.Fatalf("olay = %d", rec.Code)
	}
	st.W.Exec(`INSERT INTO sites (id, domain, public, created_at) VALUES ('pub', 'b.com', 1, 0)`)
	rec := do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil)
	return s, sessionFrom(t, rec)
}

func TestStatsSummaryForOwner(t *testing.T) {
	s, c := statsSetup(t)
	rec := do(s, "GET", "/api/stats/a.com/summary?period=day&tz=Europe/Istanbul", "", c, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("durum = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Summary struct {
			Current struct{ Visitors, Pageviews int64 }
		}
		Range struct{ Interval string }
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if body.Summary.Current.Visitors != 1 || body.Summary.Current.Pageviews != 1 || body.Range.Interval != "hour" {
		t.Fatalf("gövde = %+v", body)
	}
}

func TestStatsAccess(t *testing.T) {
	s, c := statsSetup(t)
	cases := []struct {
		name, path string
		cookie     *http.Cookie
		want       int
	}{
		{"girişsiz özel site", "/api/stats/a.com/summary", nil, http.StatusNotFound},
		{"olmayan site", "/api/stats/yok.com/summary", c, http.StatusNotFound},
		{"herkese açık site", "/api/stats/b.com/summary", nil, http.StatusOK},
		{"canlı", "/api/stats/a.com/live", c, http.StatusOK},
		{"seri", "/api/stats/a.com/timeseries?period=7d", c, http.StatusOK},
		{"kırılım", "/api/stats/a.com/breakdown?property=page", c, http.StatusOK},
		{"geçersiz kırılım", "/api/stats/a.com/breakdown?property=password_hash", c, http.StatusBadRequest},
		{"geçersiz dönem", "/api/stats/a.com/summary?period=yil", c, http.StatusBadRequest},
		{"geçersiz filtre", "/api/stats/a.com/summary?filters=" + url.QueryEscape(`[["s.site_id","x"]]`), c, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if rec := do(s, "GET", tc.path, "", tc.cookie, nil); rec.Code != tc.want {
			t.Errorf("%s: durum = %d, beklenen %d (%s)", tc.name, rec.Code, tc.want, rec.Body)
		}
	}
}

func TestStatsOtherUserCannotSee(t *testing.T) {
	s, _ := statsSetup(t)
	if _, err := s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true); err != nil {
		t.Fatal(err)
	}
	rec := do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil)
	c := sessionFrom(t, rec)
	if rec := do(s, "GET", "/api/stats/a.com/summary", "", c, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("başkasının sitesi = %d, 404 olmalı", rec.Code)
	}
}
