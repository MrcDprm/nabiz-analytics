package stats

// Performans ölçümü: go test -run x -bench . ./internal/stats
// Tek sitede 30 güne yayılmış 1 milyon olay (~330 bin ziyaret) üretir.

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

var (
	benchOnce  sync.Once
	benchStore *store.Store
	benchNow   = time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
)

func benchData(b *testing.B) *store.Store {
	benchOnce.Do(func() {
		dir, _ := os.MkdirTemp("", "nabiz-bench")
		st, err := store.Open(filepath.Join(dir, "nabiz.db"))
		if err != nil {
			b.Fatal(err)
		}
		st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)
		rng := rand.New(rand.NewPCG(1, 2))
		pages := make([]string, 200)
		for i := range pages {
			pages[i] = fmt.Sprintf("/sayfa-%d", i)
		}
		refs := []string{"", "", "google.com", "linkedin.com", "t.co", "github.com", "bing.com"}
		countries := []string{"TR", "TR", "TR", "DE", "US", "NL", "GB"}
		devices := []string{"mobile", "mobile", "desktop", "laptop", "tablet"}
		browsers := []string{"Chrome", "Chrome", "Safari", "Firefox", "Edge"}

		tx, _ := st.W.Begin()
		insS, _ := tx.Prepare(`INSERT INTO sessions (id, site_id, visitor, started_at, ended_at, pageviews, entry_page, exit_page,
			referrer, country, device, browser, os) VALUES (?, 's1', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'Windows')`)
		insE, _ := tx.Prepare(`INSERT INTO events (site_id, ts, session_id, visitor, name, pathname) VALUES ('s1', ?, ?, ?, ?, ?)`)
		start := benchNow.AddDate(0, 0, -30).Unix()
		span := benchNow.Unix() - start
		events := 0
		check := func(_ sql.Result, err error) {
			if err != nil {
				b.Fatal(err)
			}
		}
		for sid := 1; events < 1_000_000; sid++ {
			ts := start + rng.Int64N(span)
			n := 1 + rng.IntN(5)
			path := make([]string, n)
			for j := range path {
				path[j] = pages[rng.IntN(len(pages))]
			}
			visitor := rng.Int64N(200_000)
			// Önce ziyaret, sonra olaylar: events.session_id yabancı anahtar.
			check(insS.Exec(sid, visitor, ts, ts+int64((n-1)*40), n, path[0], path[n-1],
				refs[rng.IntN(len(refs))], countries[rng.IntN(len(countries))],
				devices[rng.IntN(len(devices))], browsers[rng.IntN(len(browsers))]))
			for j, p := range path {
				check(insE.Exec(ts+int64(j*40), sid, visitor, "pageview", p))
				events++
			}
			if rng.IntN(10) == 0 {
				check(insE.Exec(ts+int64(n*40), sid, visitor, "İndir", path[n-1]))
				events++
			}
		}
		if err := tx.Commit(); err != nil {
			b.Fatal(err)
		}
		st.W.Exec(`ANALYZE`)
		benchStore = st
	})
	return benchStore
}

func benchQuery(b *testing.B, filters ...Filter) Query {
	r, _ := ParsePeriod("30d", "", "", "UTC", benchNow)
	return Query{SiteID: "s1", Range: r, Filters: filters}
}

func BenchmarkSummary30d(b *testing.B) {
	st, q := benchData(b), benchQuery(b)
	for b.Loop() {
		if _, err := GetSummary(context.Background(), st.R, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSummary30dCountry(b *testing.B) {
	st, q := benchData(b), benchQuery(b, Filter{"country", "DE"})
	for b.Loop() {
		GetSummary(context.Background(), st.R, q)
	}
}

func BenchmarkSummary30dPage(b *testing.B) {
	st, q := benchData(b), benchQuery(b, Filter{"page", "/sayfa-7"})
	for b.Loop() {
		GetSummary(context.Background(), st.R, q)
	}
}

func BenchmarkTimeseries30d(b *testing.B) {
	st, q := benchData(b), benchQuery(b)
	for b.Loop() {
		GetTimeseries(context.Background(), st.R, q)
	}
}

func BenchmarkBreakdownPages30d(b *testing.B) {
	st, q := benchData(b), benchQuery(b)
	for b.Loop() {
		GetBreakdown(context.Background(), st.R, q, "page", 10, 0)
	}
}

func BenchmarkBreakdownSource30d(b *testing.B) {
	st, q := benchData(b), benchQuery(b)
	for b.Loop() {
		GetBreakdown(context.Background(), st.R, q, "source", 10, 0)
	}
}
