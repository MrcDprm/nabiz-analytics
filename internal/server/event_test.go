package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

const validBody = `{"n":"pageview","u":"https://a.com/","s":"a.com","w":1280}`

func post(s *Server, body, origin, ua string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/event", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.Header.Set("User-Agent", ua)
	req.RemoteAddr = "81.214.1.1:5555"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func eventCount(t *testing.T, s *Server) int {
	t.Helper()
	var n int
	if err := s.store.R.QueryRow(`SELECT count(*) FROM events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEventAccepted(t *testing.T) {
	s, st := newTestServer(t)
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)

	rec := post(s, validBody, "https://a.com", chromeUA)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("durum = %d, 202 olmalı", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://a.com" {
		t.Fatalf("CORS başlığı = %q", got)
	}
	if eventCount(t, s) != 1 {
		t.Fatal("olay yazılmadı")
	}
}

func TestEventRejections(t *testing.T) {
	s, st := newTestServer(t)
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)

	cases := []struct {
		name, body, origin string
		want               int
	}{
		{"bozuk gövde", `{`, "https://a.com", http.StatusBadRequest},
		{"Origin yok", validBody, "", http.StatusForbidden},
		{"başka Origin", validBody, "https://kotu.com", http.StatusForbidden},
		{"benzer Origin", validBody, "https://evila.com", http.StatusForbidden},
		{"çok büyük", `{"n":"` + strings.Repeat("x", 3000) + `"}`, "https://a.com", http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		if rec := post(s, c.body, c.origin, chromeUA); rec.Code != c.want {
			t.Errorf("%s: durum = %d, beklenen %d", c.name, rec.Code, c.want)
		}
	}
	if eventCount(t, s) != 0 {
		t.Fatal("reddedilen istek olay yazdı")
	}
}

func TestEventBotAndUnknownSiteSilent(t *testing.T) {
	s, st := newTestServer(t)
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)

	if rec := post(s, validBody, "https://a.com", "curl/8.5.0"); rec.Code != http.StatusAccepted {
		t.Fatalf("bot: durum = %d, 202 olmalı", rec.Code)
	}
	unknown := `{"n":"pageview","u":"https://b.com/","s":"b.com"}`
	if rec := post(s, unknown, "https://b.com", chromeUA); rec.Code != http.StatusAccepted {
		t.Fatalf("kayıtsız site: durum = %d, 202 olmalı", rec.Code)
	}
	if eventCount(t, s) != 0 {
		t.Fatal("bot ya da kayıtsız site olay yazdı")
	}
}

func TestEventRateLimit(t *testing.T) {
	s, _ := newTestServer(t)
	clock := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	s.eventLimit = newLimiter(1, 3, func() time.Time { return clock })

	for i := range 3 {
		if rec := post(s, validBody, "https://a.com", chromeUA); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("%d. istek erken sınırlandı", i+1)
		}
	}
	if rec := post(s, validBody, "https://a.com", chromeUA); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4. istek: durum = %d, 429 olmalı", rec.Code)
	}
	clock = clock.Add(time.Second) // bir jeton doldu
	if rec := post(s, validBody, "https://a.com", chromeUA); rec.Code == http.StatusTooManyRequests {
		t.Fatal("jeton dolduktan sonra hâlâ sınırlı")
	}
}

func TestVisitorIP(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/event", nil)
	req.RemoteAddr = "127.0.0.1:4000"
	// Ziyaretçi → Vercel (76.76.21.21) → Caddy: Caddy son değeri (Vercel) ekler.
	req.Header.Set("X-Forwarded-For", "81.214.1.1, 76.76.21.21")
	if ip := s.visitorIP(req); ip != "127.0.0.1" {
		t.Fatalf("proxy'ye güvenilmezken = %s", ip)
	}
	s.cfg.TrustProxy = true
	if ip := s.visitorIP(req); ip != "81.214.1.1" {
		t.Fatalf("ziyaretçi = %s, ilk değer olmalı", ip)
	}
	if ip := s.clientIP(req); ip != "76.76.21.21" {
		t.Fatalf("bağlantı = %s, son değer olmalı (hız sınırı bununla)", ip)
	}
	req.Header.Set("X-Forwarded-For", "bozuk, 76.76.21.21")
	if ip := s.visitorIP(req); ip != "76.76.21.21" {
		t.Fatalf("bozuk ilk değer = %s", ip)
	}
}

func TestClientIP(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/event", nil)
	req.RemoteAddr = "127.0.0.1:4000"
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 81.214.1.1")

	if ip := s.clientIP(req); ip != "127.0.0.1" {
		t.Fatalf("proxy'ye güvenilmezken IP = %s", ip)
	}
	s.cfg.TrustProxy = true
	if ip := s.clientIP(req); ip != "81.214.1.1" {
		t.Fatalf("proxy arkasında IP = %s, son değer (81.214.1.1) olmalı", ip)
	}
}
