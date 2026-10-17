// Package ratelimit counts requests in fixed windows, in memory. Each API machine counts on its
// own, and a restart starts the counts again; that is enough for one or two machines.
package ratelimit

import (
	"sync"
	"time"
)

// sweepEvery is how often windows that have ended are dropped.
const sweepEvery = time.Minute

// Limiter counts requests per key.
type Limiter struct {
	mu        sync.Mutex
	windows   map[string]window
	lastSweep time.Time
	now       func() time.Time
}

type window struct {
	ends  time.Time
	count int
}

// New returns a Limiter on the given clock, or the real one when now is nil.
func New(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{windows: map[string]window{}, now: now}
}

// Allow counts one request for key against limit requests per window. When the limit is
// reached it returns false and how long until the window ends.
func (l *Limiter) Allow(key string, limit int, per time.Duration) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= sweepEvery {
		for k, w := range l.windows {
			if !now.Before(w.ends) {
				delete(l.windows, k)
			}
		}
		l.lastSweep = now
	}
	w, ok := l.windows[key]
	if !ok || !now.Before(w.ends) {
		w = window{ends: now.Add(per)}
	}
	if w.count >= limit {
		return false, w.ends.Sub(now)
	}
	w.count++
	l.windows[key] = w
	return true, 0
}
