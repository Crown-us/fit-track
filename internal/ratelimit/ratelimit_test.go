package ratelimit

import (
	"sync"
	"testing"
	"time"
)

const (
	testLimit  = 3
	testWindow = time.Minute
	testKey    = "203.0.113.7"
)

// fakeClock is a manually advanced clock.
type fakeClock struct {
	current time.Time
}

func (c *fakeClock) now() time.Time { return c.current }

func (c *fakeClock) advance(d time.Duration) { c.current = c.current.Add(d) }

func newTestLimiter() (*Limiter, *fakeClock) {
	clock := &fakeClock{current: time.Date(2026, time.November, 16, 9, 0, 0, 0, time.UTC)}
	return New(testLimit, testWindow, clock.now), clock
}

// exhaust uses up the whole budget of testKey.
func exhaust(t *testing.T, limiter *Limiter) {
	t.Helper()
	for attempt := range testLimit {
		if isAllowed, _ := limiter.Allow(testKey); !isAllowed {
			t.Fatalf("attempt %d was rejected within the limit", attempt+1)
		}
	}
}

func TestAllowWithinLimitThenRejects(t *testing.T) {
	limiter, clock := newTestLimiter()
	exhaust(t, limiter)

	clock.advance(20 * time.Second)
	isAllowed, retryAfter := limiter.Allow(testKey)
	if isAllowed {
		t.Fatal("Allow() accepted an event over the limit")
	}
	if retryAfter != 40*time.Second {
		t.Errorf("retryAfter = %v, want 40s (the rest of the window)", retryAfter)
	}
}

func TestRejectedEventsDoNotExtendTheWindow(t *testing.T) {
	limiter, clock := newTestLimiter()
	exhaust(t, limiter)
	for range 10 {
		clock.advance(5 * time.Second)
		limiter.Allow(testKey)
	}
	// 50 s have passed; hammering must not have pushed the reset further out.
	clock.advance(10 * time.Second)
	if isAllowed, _ := limiter.Allow(testKey); !isAllowed {
		t.Error("Allow() still rejects after the original window ended")
	}
}

func TestWindowResets(t *testing.T) {
	tests := []struct {
		name        string
		wait        time.Duration
		wantAllowed bool
	}{
		{name: "just before the window ends", wait: testWindow - time.Nanosecond, wantAllowed: false},
		{name: "exactly at the window end", wait: testWindow, wantAllowed: true},
		{name: "long after", wait: 10 * testWindow, wantAllowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, clock := newTestLimiter()
			exhaust(t, limiter)
			clock.advance(tt.wait)
			if isAllowed, _ := limiter.Allow(testKey); isAllowed != tt.wantAllowed {
				t.Errorf("Allow() = %v, want %v", isAllowed, tt.wantAllowed)
			}
		})
	}
}

func TestKeysAreIndependent(t *testing.T) {
	limiter, _ := newTestLimiter()
	exhaust(t, limiter)
	if isAllowed, _ := limiter.Allow("b"); !isAllowed {
		t.Error("exhausting one key rejected another key")
	}
}

func TestSweepDropsExpiredWindows(t *testing.T) {
	limiter, clock := newTestLimiter()
	limiter.Allow("old")
	clock.advance(testWindow)
	limiter.Allow("fresh")

	if _, isKept := limiter.windows["old"]; isKept {
		t.Error("expired window was not swept")
	}
	if _, isKept := limiter.windows["fresh"]; !isKept {
		t.Error("active window was swept")
	}
}

func TestAllowIsSafeForConcurrentUse(t *testing.T) {
	limiter, _ := newTestLimiter()
	const callers = 50
	results := make(chan bool, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			isAllowed, _ := limiter.Allow("shared")
			results <- isAllowed
		}()
	}
	group.Wait()
	close(results)

	allowed := 0
	for isAllowed := range results {
		if isAllowed {
			allowed++
		}
	}
	if allowed != testLimit {
		t.Errorf("%d concurrent events were allowed, want exactly %d", allowed, testLimit)
	}
}
