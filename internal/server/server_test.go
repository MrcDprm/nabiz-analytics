package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
	return New(Config{Mode: ModeSelfhost, Version: "test"}, st, rec), st
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
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
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
