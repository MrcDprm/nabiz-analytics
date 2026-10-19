package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/geo"
	"github.com/MrcDprm/nabiz-analytics/internal/ingest"
	"github.com/MrcDprm/nabiz-analytics/internal/salt"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	k, err := salt.New(context.Background(), st.W, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := geo.Open(filepath.Join(dir, "yok.mmdb"))
	rec := ingest.NewRecorder(st, k, g, nil)
	a, err := auth.NewService(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{Mode: ModeSelfhost, Version: "test"}, st, rec, a), st
}

func TestHealthz(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("durum = %d, 200 olmalı", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Fatalf("gövde = %v", body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Permissions-Policy") == "" {
		t.Error("güvenlik başlıkları eksik")
	}
}

func TestHealthzDatabaseDown(t *testing.T) {
	s, st := newTestServer(t)
	st.Close()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("durum = %d, 503 olmalı", rec.Code)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/yok", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("durum = %d, 404 olmalı", rec.Code)
	}
}

func TestTrackerServed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/js/nabiz.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("derlenmemiş betik: durum = %d, 404 olmalı", rec.Code)
	}

	s.cfg.Tracker = []byte("(function(){})();")
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/js/nabiz.js", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "(function(){})();" {
		t.Fatalf("durum = %d, gövde = %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/javascript; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}
