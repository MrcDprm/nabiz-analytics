package ingest

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/geo"
	"github.com/MrcDprm/nabiz-analytics/internal/salt"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newRecorder(t *testing.T) (*Recorder, *store.Store, *testClock) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`); err != nil {
		t.Fatal(err)
	}
	c := &testClock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	k, err := salt.New(context.Background(), st.W, c.now)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := geo.Open(filepath.Join(dir, "yok.mmdb"))
	return NewRecorder(st, k, g, c.now), st, c
}

func pv(path string) *Event {
	return &Event{Name: Pageview, Site: "a.com", Path: path, Referrer: "google.com", Device: "laptop"}
}

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

func record(t *testing.T, r *Recorder, e *Event, ip string) {
	t.Helper()
	if err := r.Record(context.Background(), e, ip, ua); err != nil {
		t.Fatal(err)
	}
}

type sessionRow struct {
	pageviews         int
	entry, exit       string
	started, ended    int64
	referrer, browser string
	os                string
}

func sessions(t *testing.T, st *store.Store) []sessionRow {
	t.Helper()
	rows, err := st.R.Query(`SELECT pageviews, entry_page, exit_page, started_at, ended_at, referrer, browser, os FROM sessions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		var s sessionRow
		rows.Scan(&s.pageviews, &s.entry, &s.exit, &s.started, &s.ended, &s.referrer, &s.browser, &s.os)
		out = append(out, s)
	}
	return out
}

func count(t *testing.T, st *store.Store, table string) int {
	t.Helper()
	var n int
	if err := st.R.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSameVisitOnePerSession(t *testing.T) {
	r, st, c := newRecorder(t)
	record(t, r, pv("/"), "1.1.1.1")
	c.add(10 * time.Minute)
	record(t, r, pv("/projects"), "1.1.1.1")
	c.add(5 * time.Minute)
	record(t, r, &Event{Name: "İndir", Site: "a.com", Path: "/projects"}, "1.1.1.1")

	s := sessions(t, st)
	if len(s) != 1 {
		t.Fatalf("ziyaret sayısı = %d, 1 olmalı", len(s))
	}
	got := s[0]
	if got.pageviews != 2 || got.entry != "/" || got.exit != "/projects" || got.ended-got.started != 15*60 {
		t.Fatalf("ziyaret = %+v", got)
	}
	if got.browser != "Chrome" || got.os != "Windows" || got.referrer != "google.com" {
		t.Fatalf("tarayıcı/kaynak yanlış: %+v", got)
	}
	if n := count(t, st, "events"); n != 3 {
		t.Fatalf("olay sayısı = %d, 3 olmalı", n)
	}
}

func TestLongPauseStartsNewVisit(t *testing.T) {
	r, st, c := newRecorder(t)
	record(t, r, pv("/"), "1.1.1.1")
	c.add(SessionTimeout + time.Second)
	record(t, r, pv("/"), "1.1.1.1")
	if n := len(sessions(t, st)); n != 2 {
		t.Fatalf("ziyaret sayısı = %d, 2 olmalı", n)
	}
}

func TestDifferentVisitorsSeparate(t *testing.T) {
	r, st, _ := newRecorder(t)
	record(t, r, pv("/"), "1.1.1.1")
	record(t, r, pv("/"), "2.2.2.2")
	if n := len(sessions(t, st)); n != 2 {
		t.Fatalf("ziyaret sayısı = %d, 2 olmalı", n)
	}
}

func TestPropsStored(t *testing.T) {
	r, st, _ := newRecorder(t)
	e := &Event{Name: "Kayıt", Site: "a.com", Path: "/", Props: map[string]string{"paket": "pro", "kaynak": "menü"}}
	record(t, r, e, "1.1.1.1")
	if n := count(t, st, "event_props"); n != 2 {
		t.Fatalf("özellik sayısı = %d, 2 olmalı", n)
	}
	// Özel olay görüntüleme sayılmaz.
	if s := sessions(t, st); s[0].pageviews != 0 {
		t.Fatalf("özel olay görüntüleme sayıldı: %d", s[0].pageviews)
	}
}

func TestUnknownSite(t *testing.T) {
	r, st, _ := newRecorder(t)
	e := pv("/")
	e.Site = "b.com"
	if err := r.Record(context.Background(), e, "1.1.1.1", ua); err != ErrUnknownSite {
		t.Fatalf("hata = %v, ErrUnknownSite olmalı", err)
	}
	if n := count(t, st, "events"); n != 0 {
		t.Fatal("kayıtlı olmayan site için olay yazıldı")
	}
}

func TestNewSiteCountedAfterCache(t *testing.T) {
	r, st, c := newRecorder(t)
	e := pv("/")
	e.Site = "b.com"
	r.Record(context.Background(), e, "1.1.1.1", ua) // önbelleğe "yok" yazıldı
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s2', 'b.com', 0)`)
	c.add(siteTTL + time.Second)
	record(t, r, e, "1.1.1.1")
}

func TestDeletedSiteSessionRecovers(t *testing.T) {
	r, st, _ := newRecorder(t)
	record(t, r, pv("/"), "1.1.1.1")
	// Ziyaret satırı silinirse (ör. veri temizliği) aynı ziyaretçi yeni ziyaret açmalı.
	st.W.Exec(`DELETE FROM sessions`)
	record(t, r, pv("/x"), "1.1.1.1")
	if s := sessions(t, st); len(s) != 1 || s[0].entry != "/x" {
		t.Fatalf("ziyaretler = %+v", s)
	}
}

func TestSweep(t *testing.T) {
	r, _, c := newRecorder(t)
	record(t, r, pv("/"), "1.1.1.1")
	c.add(SessionTimeout)
	r.Sweep()
	if len(r.active) != 0 || len(r.sites) != 0 {
		t.Fatalf("temizlik sonrası kalan: %d ziyaret, %d site", len(r.active), len(r.sites))
	}
}

func TestConcurrentSameVisitor(t *testing.T) {
	r, st, _ := newRecorder(t)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Record(context.Background(), pv("/"), "1.1.1.1", ua); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if s := sessions(t, st); len(s) != 1 || s[0].pageviews != 10 {
		t.Fatalf("eşzamanlı istekler: %+v", s)
	}
}
