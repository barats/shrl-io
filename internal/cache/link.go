package cache

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/domain"
)

const keyPrefix = "link:"

// Key is the Redis key for a link's redirect mapping.
func Key(hostname, code string) string { return keyPrefix + hostname + ":" + code }

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
		return c.rdb.Del(ctx, Key(l.Hostname, l.Code)).Err()
	}
	return c.rdb.Set(ctx, Key(l.Hostname, l.Code), l.Destination, 0).Err()
}

// Get returns the destination for a link. ok is false when the key is absent
// (unknown or disabled link).
func (c *LinkCache) Get(ctx context.Context, hostname, code string) (dest string, ok bool, err error) {
	dest, err = c.rdb.Get(ctx, Key(hostname, code)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return dest, true, nil
}

func (c *LinkCache) Delete(ctx context.Context, hostname, code string) error {
	return c.rdb.Del(ctx, Key(hostname, code)).Err()
}
