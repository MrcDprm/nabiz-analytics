package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWebRoutes(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Web = fstest.MapFS{
		"index.html":       {Data: []byte("<!doctype html>panel")},
		"assets/app-1.js":  {Data: []byte("console.log(1)")},
		"assets/.gizli.js": {Data: []byte("x")},
	}
	cases := []struct {
		path string
		want int
		body string
	}{
		{"/", 200, "panel"},
		{"/login", 200, "panel"},
		{"/miracdeprem.com", 200, "panel"},
		{"/wp-admin", 404, ""},
		{"/assets/app-1.js", 200, "console.log"},
		{"/assets/.gizli.js", 404, ""},
		{"/assets/yok.js", 404, ""},
		{"/healthz", 200, "ok"},
	}
	for _, c := range cases {
		rec := do(s, "GET", c.path, "", nil, nil)
		if rec.Code != c.want || !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("%s: %d %q", c.path, rec.Code, rec.Body.String())
		}
	}
	rec := do(s, "GET", "/miracdeprem.com", "", nil, nil)
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "require-trusted-types-for 'script'") || !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("CSP eksik: %q", csp)
	}
	if cc := do(s, "GET", "/assets/app-1.js", "", nil, nil).Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("assets önbelleği: %q", cc)
	}
}

func TestListSites(t *testing.T) {
	s, c := statsSetup(t)
	rec := do(s, "GET", "/api/sites", "", c, nil)
	var body struct {
		Sites []siteJSON `json:"sites"`
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if rec.Code != 200 || len(body.Sites) != 1 || body.Sites[0].Domain != "a.com" || body.Sites[0].Role != "owner" {
		t.Fatalf("siteler = %d %+v", rec.Code, body)
	}
	if rec := do(s, "GET", "/api/sites", "", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("girişsiz = %d", rec.Code)
	}
}

func TestTimeseriesCompare(t *testing.T) {
	s, c := statsSetup(t)
	rec := do(s, "GET", "/api/stats/a.com/timeseries?period=7d&compare=1", "", c, nil)
	var body struct {
		Points   []json.RawMessage `json:"points"`
		Previous []json.RawMessage `json:"previous"`
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body.Points) != 7 || len(body.Previous) != 7 {
		t.Fatalf("çubuklar = %d / %d", len(body.Points), len(body.Previous))
	}
}
