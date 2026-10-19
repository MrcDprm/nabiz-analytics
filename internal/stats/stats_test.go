package stats

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

var ist, _ = time.LoadLocation("Europe/Istanbul")

// at: 19 Ekim 2026, İstanbul saatiyle hh:mm.
func at(h, m int) time.Time { return time.Date(2026, 10, 19, h, m, 0, 0, ist) }

// fixture, elle hesaplanabilecek küçük bir veri kümesi kurar:
//
//	z1  ziyaretçi 1  10:00-10:05  / → /projects  google.com   TR mobil Chrome Android
//	z2  ziyaretçi 2  11:00        /  + "İndir"   (doğrudan)   DE masaüstü Firefox Windows
//	z3  ziyaretçi 1  13:00-13:10  /blog + "Kayıt" linkedin.com TR mobil Chrome Android
//	z4  ziyaretçi 3  dün 12:00    /              (doğrudan)   TR
//	z5  başka site, bugün                                      (hiçbir sonuçta görünmemeli)
func fixture(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := st.W.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0), ('s2', 'b.com', 0)`)
	type sess struct {
		id                     int
		site                   string
		visitor                int
		start, end             time.Time
		pv                     int
		entry, exit, ref, ctry string
		device, browser, os    string
	}
	yesterday := at(12, 0).AddDate(0, 0, -1)
	for _, s := range []sess{
		{1, "s1", 1, at(10, 0), at(10, 5), 2, "/", "/projects", "google.com", "TR", "mobile", "Chrome", "Android"},
		{2, "s1", 2, at(11, 0), at(11, 0), 1, "/", "/", "", "DE", "desktop", "Firefox", "Windows"},
		{3, "s1", 1, at(13, 0), at(13, 10), 1, "/blog", "/blog", "linkedin.com", "TR", "mobile", "Chrome", "Android"},
		{4, "s1", 3, yesterday, yesterday, 1, "/", "/", "", "TR", "laptop", "Safari", "macOS"},
		{5, "s2", 9, at(10, 0), at(10, 0), 1, "/", "/", "", "TR", "mobile", "Chrome", "Android"},
	} {
		exec(`INSERT INTO sessions (id, site_id, visitor, started_at, ended_at, pageviews, entry_page, exit_page,
			referrer, country, device, browser, os) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.id, s.site, s.visitor, s.start.Unix(), s.end.Unix(), s.pv, s.entry, s.exit, s.ref, s.ctry, s.device, s.browser, s.os)
	}
	type ev struct {
		session int
		site    string
		ts      time.Time
		name    string
		path    string
	}
	for _, e := range []ev{
		{1, "s1", at(10, 0), "pageview", "/"},
		{1, "s1", at(10, 5), "pageview", "/projects"},
		{2, "s1", at(11, 0), "pageview", "/"},
		{2, "s1", at(11, 0), "İndir", "/"},
		{3, "s1", at(13, 0), "pageview", "/blog"},
		{3, "s1", at(13, 10), "Kayıt", "/blog"},
		{4, "s1", yesterday, "pageview", "/"},
		{5, "s2", at(10, 0), "pageview", "/"},
	} {
		exec(`INSERT INTO events (site_id, ts, session_id, visitor, name, pathname)
			VALUES (?, ?, ?, (SELECT visitor FROM sessions WHERE id = ?), ?, ?)`,
			e.site, e.ts.Unix(), e.session, e.session, e.name, e.path)
	}
	return st
}

func today(t *testing.T, filters ...Filter) Query {
	t.Helper()
	r, err := ParsePeriod("day", "", "", "Europe/Istanbul", at(15, 0))
	if err != nil {
		t.Fatal(err)
	}
	return Query{SiteID: "s1", Range: r, Filters: filters}
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestSummary(t *testing.T) {
	st := fixture(t)
	s, err := GetSummary(context.Background(), st.R, today(t))
	if err != nil {
		t.Fatal(err)
	}
	c := s.Current
	if c.Visitors != 2 || c.Visits != 3 || c.Pageviews != 4 || !near(c.BounceRate, 66.67) || !near(c.VisitLength, 300) {
		t.Fatalf("bugün = %+v", c)
	}
	p := s.Previous
	if p.Visitors != 1 || p.Visits != 1 || p.Pageviews != 1 || !near(p.BounceRate, 100) {
		t.Fatalf("dün = %+v", p)
	}
}

func TestSummaryFilters(t *testing.T) {
	st := fixture(t)
	cases := []struct {
		f                        Filter
		visitors, visits, pviews int64
	}{
		{Filter{"country", "TR"}, 1, 2, 3},
		{Filter{"source", ""}, 1, 1, 1}, // doğrudan gelenler
		{Filter{"page", "/projects"}, 1, 1, 1},
		{Filter{"event", "İndir"}, 1, 1, 1},
		{Filter{"browser", "Safari"}, 0, 0, 0},
	}
	for _, c := range cases {
		s, err := GetSummary(context.Background(), st.R, today(t, c.f))
		if err != nil {
			t.Fatal(err)
		}
		m := s.Current
		if m.Visitors != c.visitors || m.Visits != c.visits || m.Pageviews != c.pviews {
			t.Errorf("%v: %+v", c.f, m)
		}
	}
}

func TestBreakdown(t *testing.T) {
	st := fixture(t)
	ctx := context.Background()
	cases := map[string][]Row{
		"page":    {{"/", 2, 2}, {"/blog", 1, 1}, {"/projects", 1, 1}},
		"source":  {{"", 1, 1}, {"google.com", 1, 1}, {"linkedin.com", 1, 1}},
		"country": {{"TR", 1, 2}, {"DE", 1, 1}},
		"event":   {{"Kayıt", 1, 1}, {"İndir", 1, 1}},
	}
	for prop, want := range cases {
		got, err := GetBreakdown(ctx, st.R, today(t), prop, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %+v", prop, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s[%d] = %+v, beklenen %+v", prop, i, got[i], want[i])
			}
		}
	}
	if _, err := GetBreakdown(ctx, st.R, today(t), "password_hash", 10, 0); err == nil {
		t.Fatal("bilinmeyen kırılım kabul edildi")
	}
}

func TestBreakdownPaging(t *testing.T) {
	st := fixture(t)
	got, _ := GetBreakdown(context.Background(), st.R, today(t), "page", 1, 1)
	if len(got) != 1 || got[0].Value != "/blog" {
		t.Fatalf("2. sayfa = %+v", got)
	}
}

func TestTimeseries(t *testing.T) {
	st := fixture(t)
	points, err := GetTimeseries(context.Background(), st.R, today(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 24 {
		t.Fatalf("saat sayısı = %d", len(points))
	}
	for h, want := range map[int][2]int64{10: {1, 2}, 11: {1, 1}, 13: {1, 1}, 12: {0, 0}} {
		p := points[h]
		if p.Start.Hour() != h || p.Visitors != want[0] || p.Pageviews != want[1] {
			t.Errorf("%02d:00 = %+v, beklenen ziyaretçi %d görüntüleme %d", h, p, want[0], want[1])
		}
	}
}

func TestLive(t *testing.T) {
	st := fixture(t)
	l, err := GetLive(context.Background(), st.R, "s1", at(10, 7))
	if err != nil {
		t.Fatal(err)
	}
	// 10:02'den sonra: z1 hâlâ açık (10:05'te görüldü); sadece 10:05 görüntülemesi şeritte.
	if l.Visitors != 1 || len(l.Beats) != 1 || l.Beats[0] != at(10, 5).Unix() {
		t.Fatalf("canlı = %+v", l)
	}
}

func TestParseFilters(t *testing.T) {
	f, err := ParseFilters(`[["country","TR"],["page","/a:b;c"]]`)
	if err != nil || len(f) != 2 || f[1].Value != "/a:b;c" {
		t.Fatalf("%+v %v", f, err)
	}
	for _, bad := range []string{`{`, `[["password_hash","x"]]`, `[["page",""]]`, `[["s.site_id","x"]]`} {
		if _, err := ParseFilters(bad); err == nil {
			t.Errorf("%s kabul edildi", bad)
		}
	}
}

func TestParsePeriod(t *testing.T) {
	now := at(15, 0)
	r, _ := ParsePeriod("7d", "", "", "Europe/Istanbul", now)
	if !r.From.Equal(time.Date(2026, 10, 13, 0, 0, 0, 0, ist)) || !r.To.Equal(time.Date(2026, 10, 20, 0, 0, 0, 0, ist)) ||
		!r.PrevFrom.Equal(time.Date(2026, 10, 6, 0, 0, 0, 0, ist)) || len(r.Buckets()) != 7 {
		t.Fatalf("7d = %+v", r)
	}
	r, _ = ParsePeriod("month", "", "", "Europe/Istanbul", now)
	if !r.PrevFrom.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, ist)) || len(r.Buckets()) != 31 {
		t.Fatalf("month = %+v", r)
	}
	r, _ = ParsePeriod("12m", "", "", "Europe/Istanbul", now)
	if len(r.Buckets()) != 12 || r.Buckets()[0].Month() != time.November {
		t.Fatalf("12m = %+v", r.Buckets())
	}
	r, err := ParsePeriod("custom", "2026-10-01", "2026-10-03", "Europe/Istanbul", now)
	if err != nil || len(r.Buckets()) != 3 {
		t.Fatalf("custom = %+v %v", r, err)
	}
	for _, bad := range [][3]string{{"yil", "", ""}, {"custom", "2026-10-05", "2026-10-01"}, {"custom", "2024-01-01", "2026-01-01"}, {"custom", "dün", "bugün"}} {
		if _, err := ParsePeriod(bad[0], bad[1], bad[2], "Europe/Istanbul", now); err == nil {
			t.Errorf("%v kabul edildi", bad)
		}
	}
	// Bilinmeyen saat dilimi hata değil, UTC'ye düşer.
	if r, err := ParsePeriod("day", "", "", "Mars/Olympus", now); err != nil || r.Loc != time.UTC {
		t.Fatalf("bilinmeyen tz: %v %v", r.Loc, err)
	}
}

func TestDSTDayHas25Hours(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	// 25 Ekim 2026: Almanya'da yaz saati biter, gün 25 saat sürer.
	r, _ := ParsePeriod("day", "", "", "Europe/Berlin", time.Date(2026, 10, 25, 12, 0, 0, 0, berlin))
	if n := len(r.Buckets()); n != 25 {
		t.Fatalf("saat sayısı = %d, 25 olmalı", n)
	}
}
