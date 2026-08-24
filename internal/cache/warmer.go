package cache

import (
	"context"

	"github.com/barats/shrl-io/internal/domain"
)

// LinkSource is what the warmer needs from the write side.
type LinkSource interface {
	ListActive(ctx context.Context) ([]domain.Link, error)
}

// Warm loads every active link into Redis. The redirector stays Redis-only:
// misses are genuine unknowns or disabled links, so a warm cache prevents
// false 404s after eviction or a cold start.
func (c *Cache) Warm(ctx context.Context, src LinkSource) (int, error) {
	links, err := src.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	for i := range links {
		if err := c.Put(ctx, &links[i]); err != nil {
			return 0, err
		}
	}
	return len(links), nil
}
