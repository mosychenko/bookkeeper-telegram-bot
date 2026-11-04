package sheet

import (
	"context"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

type counting struct {
	n  int
	in budget.Input
}

func (c *counting) Load(context.Context, time.Time) (budget.Input, error) {
	c.n++
	return c.in, nil
}

func TestCacheKeepsARead(t *testing.T) {
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	src := &counting{in: budget.Input{Planned: 10}}
	c := newCache(src, 3*time.Minute, func() time.Time { return at })
	if _, err := c.Load(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Load(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if src.n != 1 {
		t.Fatalf("кэш не сработал, чтений %d", src.n)
	}
	at = at.Add(3*time.Minute + time.Second)
	if _, err := c.Load(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	if src.n != 2 {
		t.Fatalf("после трёх минут ждали новое чтение, было %d", src.n)
	}
}
