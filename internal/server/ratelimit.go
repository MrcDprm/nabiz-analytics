package server

import (
	"sync"
	"time"
)

// limiter, anahtar (IP) başına basit bir jeton kovasıdır: kovada en fazla
// burst jeton olur, her saniye rate kadar dolar, her istek bir jeton harcar.
// IP'ler sadece bellekte ve kısa süre durur; hiçbir yere yazılmaz.
type limiter struct {
	rate  float64
	burst float64
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rate, burst float64, now func() time.Time) *limiter {
	if now == nil {
		now = time.Now
	}
	return &limiter{rate: rate, burst: burst, now: now, buckets: make(map[string]*bucket)}
}

func (l *limiter) allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep, dolmuş (yani uzun süredir istek atmayan) kovaları siler.
func (l *limiter) sweep() {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, k)
		}
	}
}
