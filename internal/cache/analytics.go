package cache

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	visitStream  = "visits"
	maxStreamLen = 1000000
	// visitGroup is the single consumer group for the visits stream. Worker
	// replicas are consumers within it, so events are partitioned across them.
	visitGroup  = "visits-group"
	uvKeyPrefix = "uv:"
	uvSetTTL    = 48 * time.Hour
)

// AnalyticsCache is the Redis layer for visit tracking: the capped visits
// stream and the per-day unique-visitor dedup sets.
type AnalyticsCache struct {
	rdb *redis.Client
}

func NewAnalyticsCache(rdb *redis.Client) *AnalyticsCache { return &AnalyticsCache{rdb: rdb} }

// RecordVisit appends one event per redirect to a capped Redis stream. The
// worker aggregates it into the analytics rollups. utm holds the recognized
// UTM parameter values from the short URL; empty values are omitted.
func (c *AnalyticsCache) RecordVisit(ctx context.Context, hostname, code, ip, userAgent, referrer string, utm map[string]string) error {
	values := map[string]interface{}{
		"hostname":   hostname,
		"code":       code,
		"ip":         ip,
		"user_agent": userAgent,
		"referrer":   referrer,
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
	}
	for k, v := range utm {
		if v != "" {
			values[k] = v
		}
	}
	_, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: visitStream,
		MaxLen: maxStreamLen,
		Approx: true,
		Values: values,
	}).Result()
	return err
}

func uvKey(code string, day time.Time) string {
	return uvKeyPrefix + code + ":" + day.Format("2006-01-02")
}

func uvDimKey(code string, day time.Time, dimension, value string) string {
	return uvKeyPrefix + "d:" + code + ":" + day.Format("2006-01-02") + ":" + dimension + ":" + value
}

// AddUniqueVisitor records a hashed visitor identity for a link on a day and
// reports whether it is new (the SADD returned 1). The per-day set persists
// for 48h so dedup works across worker batches.
func (c *AnalyticsCache) AddUniqueVisitor(ctx context.Context, code string, day time.Time, hash string) (bool, error) {
	key := uvKey(code, day)
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
func (c *AnalyticsCache) RemoveUniqueVisitor(ctx context.Context, code string, day time.Time, hash string) error {
	return c.rdb.SRem(ctx, uvKey(code, day), hash).Err()
}

// AddUniqueVisitorDim records a hashed visitor identity for a link on a day
// within one dimension value and reports whether it is new. Each (code, day,
// dimension, value) has its own set, so a visitor who appears under several
// values (e.g. multiple referrers) is counted once per value.
func (c *AnalyticsCache) AddUniqueVisitorDim(ctx context.Context, code string, day time.Time, dimension, value, hash string) (bool, error) {
	key := uvDimKey(code, day, dimension, value)
	added, err := c.rdb.SAdd(ctx, key, hash).Result()
	if err != nil {
		return false, err
	}
	if added == 1 {
		c.rdb.Expire(ctx, key, uvSetTTL)
	}
	return added == 1, nil
}

// RemoveUniqueVisitorDim undoes a dimension-scoped dedup addition after a
// failed DB apply.
func (c *AnalyticsCache) RemoveUniqueVisitorDim(ctx context.Context, code string, day time.Time, dimension, value, hash string) error {
	return c.rdb.SRem(ctx, uvDimKey(code, day, dimension, value), hash).Err()
}

// EnsureVisitGroup creates the visits consumer group from the stream start so
// existing backlog is processed. BUSYGROUP means it already exists.
func (c *AnalyticsCache) EnsureVisitGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, visitStream, visitGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// ReadVisitsNew returns a batch of new messages for a consumer. An empty
// result means the stream had nothing new.
func (c *AnalyticsCache) ReadVisitsNew(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	return c.readVisits(ctx, consumer, count, block, ">")
}

// ReadVisitsPending returns this consumer's delivered-but-unacked messages,
// recovering a crashed worker's in-flight batch on restart.
func (c *AnalyticsCache) ReadVisitsPending(ctx context.Context, consumer string, count int64) ([]redis.XMessage, error) {
	return c.readVisits(ctx, consumer, count, 0, "0")
}

func (c *AnalyticsCache) readVisits(ctx context.Context, consumer string, count int64, block time.Duration, id string) ([]redis.XMessage, error) {
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
func (c *AnalyticsCache) AckVisits(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return c.rdb.XAck(ctx, visitStream, visitGroup, ids...).Err()
}
