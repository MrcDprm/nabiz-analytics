package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(Config{Mode: ModeSelfhost, Version: "test"}, st), st
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
