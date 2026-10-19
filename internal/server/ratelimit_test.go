package server

import (
	"testing"
	"time"
)

func TestLimiterPerKeyAndSweep(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	l := newLimiter(1, 2, func() time.Time { return now })

	if !l.allow("a") || !l.allow("a") || l.allow("a") {
		t.Fatal("a için kova sınırı yanlış")
	}
	if !l.allow("b") {
		t.Fatal("b, a'nın kovasından etkilendi")
	}
	now = now.Add(5 * time.Second)
	l.sweep()
	if len(l.buckets) != 0 {
		t.Fatalf("dolmuş kovalar silinmedi: %d", len(l.buckets))
	}
}
