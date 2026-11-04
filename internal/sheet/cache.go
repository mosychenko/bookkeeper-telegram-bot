package sheet

import (
	"context"
	"sync"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

// Source loads a budget input. Implementations must not write to the sheet.
type Source interface {
	Load(ctx context.Context, now time.Time) (budget.Input, error)
}

type memCache struct {
	inner Source
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	until time.Time
	in    budget.Input
	ok    bool
}

// Cache keeps a successful read for ttl. Errors are not cached.
func Cache(inner Source, ttl time.Duration) Source {
	return newCache(inner, ttl, time.Now)
}

func newCache(inner Source, ttl time.Duration, now func() time.Time) Source {
	if ttl <= 0 {
		return inner
	}
	return &memCache{inner: inner, ttl: ttl, now: now}
}

func (c *memCache) Load(ctx context.Context, now time.Time) (budget.Input, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && c.now().Before(c.until) {
		return c.in, nil
	}
	in, err := c.inner.Load(ctx, now)
	if err != nil {
		return budget.Input{}, err
	}
	c.in = in
	c.ok = true
	c.until = c.now().Add(c.ttl)
	return in, nil
}
