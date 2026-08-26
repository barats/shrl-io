// Package ratelimit provides a Redis-backed sliding-window rate limiter for
// the Auth API (ADR 0016). Counters are TTL'd; nothing here is stored
// forever, unlike the redirect cache it shares Redis with.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// allowScript implements the sliding-window counter: the count is the current
// window's count plus the previous window's count weighted by how far the
// current window has progressed. Returns {allowed, retry_after_ms}.
var allowScript = redis.NewScript(`
local base = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local cur_start = math.floor(now / window) * window
local prev_start = cur_start - window
local cur_key = base .. ":" .. cur_start
local prev_key = base .. ":" .. prev_start
local cur = redis.call('INCR', cur_key)
redis.call('EXPIRE', cur_key, window / 1000 + 60)
local prev = tonumber(redis.call('GET', prev_key) or '0')
local weight = (now - cur_start) / window
local count = prev * (1 - weight) + cur
local reset = math.floor(window - (now - cur_start))
if count > limit then
  return {0, reset}
end
return {1, reset}
`)

// Limiter is a sliding-window rate limiter keyed by arbitrary buckets.
type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// Allow checks a bucket against a limit over a window. allowed is false when
// the request must be rejected; retryAfter is how long to wait until the
// window rolls. The caller composes bucket names (e.g. "ip:1.2.3.4").
func (l *Limiter) Allow(ctx context.Context, bucket string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration) {
	if limit <= 0 {
		return true, 0 // unlimited
	}
	now := time.Now().UnixMilli()
	windowMS := window.Milliseconds()
	if windowMS <= 0 {
		return true, 0
	}
	res, err := allowScript.Run(ctx, l.rdb, []string{"rl:" + bucket}, now, windowMS, limit).Int64Slice()
	if err != nil {
		// Fail open on Redis errors: rate limiting must not take the Auth API
		// down; the limit counters are a protective measure, not a guard.
		return true, 0
	}
	if len(res) != 2 {
		return true, 0
	}
	if res[0] == 1 {
		return true, 0
	}
	return false, time.Duration(res[1]) * time.Millisecond
}
