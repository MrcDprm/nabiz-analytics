package salt

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

// clock testlerde zamanı elle ilerletmek için.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func setup(t *testing.T) (*store.Store, *clock, *Keeper) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c := &clock{t: time.Date(2026, 10, 19, 23, 50, 0, 0, time.UTC)}
	k, err := New(context.Background(), st.W, c.now)
	if err != nil {
		t.Fatal(err)
	}
	return st, c, k
}

func visitor(t *testing.T, k *Keeper, site, ip, ua string) int64 {
	t.Helper()
	v, err := k.Visitor(context.Background(), site, ip, ua)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSameDaySameVisitor(t *testing.T) {
	_, c, k := setup(t)
	a := visitor(t, k, "site", "1.2.3.4", "Firefox")
	c.add(5 * time.Minute)
	if b := visitor(t, k, "site", "1.2.3.4", "Firefox"); a != b {
		t.Fatal("aynı gün aynı kişi farklı kimlik aldı")
	}
}

func TestNextDayNewVisitor(t *testing.T) {
	_, c, k := setup(t)
	a := visitor(t, k, "site", "1.2.3.4", "Firefox")
	c.add(15 * time.Minute) // 23:50 → ertesi gün 00:05 (UTC)
	if b := visitor(t, k, "site", "1.2.3.4", "Firefox"); a == b {
		t.Fatal("gün değişti ama kimlik aynı kaldı")
	}
}

func TestDifferentInputsDiffer(t *testing.T) {
	_, _, k := setup(t)
	base := visitor(t, k, "site", "1.2.3.4", "Firefox")
	for name, v := range map[string]int64{
		"site": visitor(t, k, "other", "1.2.3.4", "Firefox"),
		"ip":   visitor(t, k, "site", "1.2.3.5", "Firefox"),
		"ua":   visitor(t, k, "site", "1.2.3.4", "Chrome"),
	} {
		if v == base {
			t.Errorf("%s değişti ama kimlik aynı", name)
		}
	}
}

func TestFieldBoundaries(t *testing.T) {
	_, _, k := setup(t)
	if visitor(t, k, "ab", "c", "") == visitor(t, k, "a", "bc", "") {
		t.Fatal("alan sınırları karışıyor")
	}
}

func TestOldSaltDeleted(t *testing.T) {
	st, c, k := setup(t)
	c.add(24 * time.Hour)
	visitor(t, k, "site", "1.2.3.4", "Firefox")

	var days []string
	rows, err := st.R.Query(`SELECT day FROM salts`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		rows.Scan(&d)
		days = append(days, d)
	}
	if len(days) != 1 || days[0] != "2026-10-20" {
		t.Fatalf("veritabanındaki tuzlar = %v, sadece 2026-10-20 olmalı", days)
	}
}

func TestRestartSameDayKeepsSalt(t *testing.T) {
	st, c, k := setup(t)
	a := visitor(t, k, "site", "1.2.3.4", "Firefox")

	k2, err := New(context.Background(), st.W, c.now)
	if err != nil {
		t.Fatal(err)
	}
	if b := visitor(t, k2, "site", "1.2.3.4", "Firefox"); a != b {
		t.Fatal("yeniden başlatma aynı gün kimliği değiştirdi")
	}
}

func TestConcurrentRotation(t *testing.T) {
	_, c, k := setup(t)
	c.add(time.Hour) // gün değişti; aynı anda gelen istekler tek tuz üretmeli
	var wg sync.WaitGroup
	results := make([]int64, 20)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := k.Visitor(context.Background(), "site", "1.2.3.4", "Firefox")
			if err != nil {
				t.Error(err)
			}
			results[i] = v
		}()
	}
	wg.Wait()
	for _, v := range results[1:] {
		if v != results[0] {
			t.Fatal("eşzamanlı istekler farklı tuz kullandı")
		}
	}
}
