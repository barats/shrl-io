package redisutil

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect creates a Redis client and waits for it to answer, so services can
// start before Redis is ready (e.g. under compose).
func Connect(ctx context.Context, addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	for attempt := 0; attempt < 30; attempt++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			return rdb
		}
		log.Printf("waiting for redis at %s", addr)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("redis at %s never became ready", addr)
	return nil
}
