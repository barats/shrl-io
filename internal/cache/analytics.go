package cache

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// visitGroup is the single consumer group for the visits stream. Worker
	// replicas are consumers within it, so events are partitioned across them.
	visitGroup   = "visits-group"
	uvKeyPrefix  = "uv:"
	uvSetTTL     = 48 * time.Hour
)

func uvKey(hostname, code string, day time.Time) string {
	return uvKeyPrefix + hostname + ":" + code + ":" + day.Format("2006-01-02")
}

// AddUniqueVisitor records a hashed visitor identity for a link on a day and
// reports whether it is new (the SADD returned 1). The per-day set persists
// for 48h so dedup works across worker batches.
func (c *Cache) AddUniqueVisitor(ctx context.Context, hostname, code string, day time.Time, hash string) (bool, error) {
	key := uvKey(hostname, code, day)
	added, err := c.rdb.SAdd(ctx, key, hash).Result()
	if err != nil {
		return false, err
	}
	if added == 1 {
		c.rdb.Expire(ctx, key, uvSetTTL)
	}
	return added == 1, nil
}

// RemoveUniqueVisitor undoes a dedup addition after a failed DB apply, so a
// redelivered batch counts the visitor as new again.
func (c *Cache) RemoveUniqueVisitor(ctx context.Context, hostname, code string, day time.Time, hash string) error {
	return c.rdb.SRem(ctx, uvKey(hostname, code, day), hash).Err()
}

// EnsureVisitGroup creates the visits consumer group from the stream start so
// existing backlog is processed. BUSYGROUP means it already exists.
func (c *Cache) EnsureVisitGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, visitStream, visitGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// ReadVisitsNew returns a batch of new messages for a consumer. An empty
// result means the stream had nothing new.
func (c *Cache) ReadVisitsNew(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	return c.readVisits(ctx, consumer, count, block, ">")
}

// ReadVisitsPending returns this consumer's delivered-but-unacked messages,
// recovering a crashed worker's in-flight batch on restart.
func (c *Cache) ReadVisitsPending(ctx context.Context, consumer string, count int64) ([]redis.XMessage, error) {
	return c.readVisits(ctx, consumer, count, 0, "0")
}

func (c *Cache) readVisits(ctx context.Context, consumer string, count int64, block time.Duration, id string) ([]redis.XMessage, error) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    visitGroup,
		Consumer: consumer,
		Count:    count,
		Block:    block,
		Streams:  []string{visitStream, id},
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// AckVisits acknowledges processed message IDs for the group.
func (c *Cache) AckVisits(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return c.rdb.XAck(ctx, visitStream, visitGroup, ids...).Err()
}
