package redisutil

import (
	"context"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config controls the Redis connection pool. Zero values fall back to
// go-redis defaults (PoolSize = 10 * GOMAXPROCS, no minimum idle connections).
type Config struct {
	Addr         string
	PoolSize     int
	MinIdleConns int
}

// ConfigFromEnv builds a pool config from SHRL_REDIS_POOL_SIZE and
// SHRL_REDIS_MIN_IDLE_CONNS, defaulting to defPoolSize/defMinIdle when unset.
func ConfigFromEnv(addr string, defPoolSize, defMinIdle int) Config {
	return Config{
		Addr:         addr,
		PoolSize:     envInt("SHRL_REDIS_POOL_SIZE", defPoolSize),
		MinIdleConns: envInt("SHRL_REDIS_MIN_IDLE_CONNS", defMinIdle),
	}
}

// Connect creates a Redis client with the given pool config and waits for it
// to answer, so services can start before Redis is ready (e.g. under compose).
func Connect(ctx context.Context, cfg Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
	for attempt := 0; attempt < 30; attempt++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			log.Printf("redis connected (%s): pool_size=%d min_idle=%d",
				cfg.Addr, effectivePoolSize(cfg.PoolSize), cfg.MinIdleConns)
			return rdb
		}
		log.Printf("waiting for redis at %s", cfg.Addr)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("redis at %s never became ready", cfg.Addr)
	return nil
}

func effectivePoolSize(poolSize int) int {
	if poolSize <= 0 {
		return 10 * runtime.GOMAXPROCS(0)
	}
	return poolSize
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
