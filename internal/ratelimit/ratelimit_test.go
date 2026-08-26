package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testLimiter(t *testing.T) *Limiter {
	t.Helper()
	rdb := miniredis.RunT(t)
	return New(redis.NewClient(&redis.Options{Addr: rdb.Addr()}))
}

func TestAllowWithinLimit(t *testing.T) {
	l := testLimiter(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow(ctx, "test:1", 3, time.Minute); !ok {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	ok, retry := l.Allow(ctx, "test:1", 3, time.Minute)
	if ok {
		t.Fatal("4th request should be rejected")
	}
	if retry <= 0 {
		t.Errorf("retryAfter = %v, want > 0", retry)
	}
	// a different bucket is unaffected
	if ok, _ := l.Allow(ctx, "test:2", 3, time.Minute); !ok {
		t.Fatal("other bucket should be allowed")
	}
}

func TestUnlimited(t *testing.T) {
	l := testLimiter(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow(ctx, "test:1", 0, time.Minute); !ok {
			t.Fatal("limit 0 should be unlimited")
		}
	}
}
