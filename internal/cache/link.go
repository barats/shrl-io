package cache

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/domain"
)

const keyPrefix = "link:"

// Key is the Redis key for a link's redirect mapping. Codes are globally
// unique (ADR 0019), so the code alone identifies the mapping; the base URL
// is a display attribute and never part of the routing key.
func Key(code string) string { return keyPrefix + code }

// cachedLink is the value stored for an active link: everything the
// Redis-only redirector needs to serve a redirect. Stored as JSON.
type cachedLink struct {
	Destination string `json:"destination"`
	ForwardUTM  bool   `json:"forward_utm"`
}

// LinkCache is the Redis layer for link redirect mappings. The redirector
// reads only here.
type LinkCache struct {
	rdb *redis.Client
}

func NewLinkCache(rdb *redis.Client) *LinkCache { return &LinkCache{rdb: rdb} }

// Put writes an active link into Redis, or removes the key if the link is
// disabled. Write-through from the API; also used by the cache warmer.
func (c *LinkCache) Put(ctx context.Context, l *domain.Link) error {
	if l.Disabled {
		return c.rdb.Del(ctx, Key(l.Code)).Err()
	}
	b, err := json.Marshal(cachedLink{Destination: l.Destination, ForwardUTM: l.ForwardUTM})
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, Key(l.Code), b, 0).Err()
}

// Get returns the cached redirect mapping for a link. ok is false when the
// key is absent (unknown or disabled link).
func (c *LinkCache) Get(ctx context.Context, code string) (cachedLink, bool, error) {
	var cl cachedLink
	b, err := c.rdb.Get(ctx, Key(code)).Bytes()
	if errors.Is(err, redis.Nil) {
		return cl, false, nil
	}
	if err != nil {
		return cl, false, err
	}
	if err := json.Unmarshal(b, &cl); err != nil {
		return cl, false, err
	}
	return cl, true, nil
}

func (c *LinkCache) Delete(ctx context.Context, code string) error {
	return c.rdb.Del(ctx, Key(code)).Err()
}
