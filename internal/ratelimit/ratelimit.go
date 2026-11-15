// Package ratelimit provides a small in-memory, fixed-window rate limiter
// keyed by an arbitrary string such as a client IP address.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most limit events per key within each window. It is safe
// for concurrent use. State lives in memory, so it is per process: several
// instances of the API would each enforce their own budget.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	windows map[string]window
	sweptAt time.Time
}

type window struct {
	startedAt time.Time
	count     int
}

// New returns a limiter allowing limit events per key in each window of the
// given length. The clock is injected so tests can move time without sleeping.
func New(limit int, length time.Duration, now func() time.Time) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  length,
		now:     now,
		windows: make(map[string]window),
		sweptAt: now(),
	}
}

// Allow records one event for key. It reports whether the event is within
// the budget and, when it is not, how long until the key's window resets.
func (l *Limiter) Allow(key string) (isAllowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)
	current, ok := l.windows[key]
	if !ok || now.Sub(current.startedAt) >= l.window {
		current = window{startedAt: now}
	}
	if current.count >= l.limit {
		return false, current.startedAt.Add(l.window).Sub(now)
	}
	current.count++
	l.windows[key] = current
	return true, 0
}

// sweep drops expired windows at most once per window length. Without it the
// map would keep one entry for every key ever seen; with it, memory is
// bounded by the keys active within the last two windows.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.sweptAt) < l.window {
		return
	}
	for key, current := range l.windows {
		if now.Sub(current.startedAt) >= l.window {
			delete(l.windows, key)
		}
	}
	l.sweptAt = now
}
