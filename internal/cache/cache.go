package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/domain"
)

const (
	keyPrefix    = "link:"
	visitStream  = "visits"
	maxStreamLen = 100000
)

// Key is the Redis key for a link's redirect mapping.
func Key(hostname, code string) string { return keyPrefix + hostname + ":" + code }

// Cache is the Redis layer. The redirector reads only here.
type Cache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// Put writes an active link into Redis, or removes the key if the link is
// disabled. Write-through from the API; also used by the cache warmer.
func (c *Cache) Put(ctx context.Context, l *domain.Link) error {
	if l.Disabled {
		return c.rdb.Del(ctx, Key(l.Hostname, l.Code)).Err()
	}
	return c.rdb.Set(ctx, Key(l.Hostname, l.Code), l.Destination, 0).Err()
}

// Get returns the destination for a link. ok is false when the key is absent
// (unknown or disabled link).
func (c *Cache) Get(ctx context.Context, hostname, code string) (dest string, ok bool, err error) {
	dest, err = c.rdb.Get(ctx, Key(hostname, code)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return dest, true, nil
}

func (c *Cache) Delete(ctx context.Context, hostname, code string) error {
	return c.rdb.Del(ctx, Key(hostname, code)).Err()
}

// RecordVisit appends one event per redirect to a capped Redis stream. The
// worker (analytics) is a later slice; data accumulates from day one.
func (c *Cache) RecordVisit(ctx context.Context, hostname, code, ip, userAgent, referrer string) error {
	_, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: visitStream,
		MaxLen: maxStreamLen,
		Approx: true,
		Values: map[string]interface{}{
			"hostname":   hostname,
			"code":       code,
			"ip":         ip,
			"user_agent": userAgent,
			"referrer":   referrer,
			"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	return err
}
