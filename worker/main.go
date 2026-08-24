package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/analytics"
	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/redisutil"
	"github.com/barats/shrl-io/internal/store"
)

type config struct {
	redisAddr     string
	databaseURL   string
	retentionDays int
	consumer      string
}

func loadConfig() config {
	hostname, _ := os.Hostname()
	return config{
		redisAddr:     envOr("SHRL_REDIS_ADDR", "localhost:6379"),
		databaseURL:   envOr("SHRL_DATABASE_URL", "postgres://shrl:shrl@localhost:5432/shrl"),
		retentionDays: envInt("SHRL_RETENTION_DAYS", 365),
		// Stable per host: a fresh PID would orphan this consumer's unacked
		// messages in the group after a restart. One worker per host at MVP.
		consumer: hostname,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	rdb := redisutil.Connect(ctx, cfg.redisAddr)
	defer rdb.Close()

	db := openPostgres(ctx, cfg.databaseURL)
	st := store.New(db)
	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ca := cache.New(rdb)
	if err := ca.EnsureVisitGroup(ctx); err != nil {
		log.Fatalf("create visit group: %v", err)
	}

	proc := &analytics.Processor{Cache: ca, Store: st}

	// Retention prune: at boot, then nightly.
	go func() {
		prune(ctx, st, cfg.retentionDays)
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for range t.C {
			prune(ctx, st, cfg.retentionDays)
		}
	}()

	// Consume the visits stream in batches. Unacked batches are redelivered
	// after a crash (at-least-once), the accepted double-count window.
	go func() {
		for {
			// Recover this consumer's in-flight batch from a previous crash.
			msgs, err := ca.ReadVisitsPending(ctx, cfg.consumer, 1000)
			if err != nil {
				log.Printf("read pending visits: %v", err)
			} else if len(msgs) > 0 {
				if err := processBatch(ctx, proc, ca, msgs); err != nil {
					log.Printf("process pending batch: %v", err)
				}
				continue
			}

			msgs, err = ca.ReadVisitsNew(ctx, cfg.consumer, 1000, 2*time.Second)
			if err != nil {
				log.Printf("read visits: %v", err)
				time.Sleep(time.Second)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			if err := processBatch(ctx, proc, ca, msgs); err != nil {
				log.Printf("process batch: %v", err)
			}
		}
	}()

	log.Printf("worker started: consumer=%s retention=%dd", cfg.consumer, cfg.retentionDays)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func processBatch(ctx context.Context, proc *analytics.Processor, ca *cache.Cache, msgs []redis.XMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]string, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	if err := proc.ProcessMessages(ctx, msgs); err != nil {
		return err
	}
	return ca.AckVisits(ctx, ids)
}

func prune(ctx context.Context, st *store.Store, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)
	if err := st.PruneAnalytics(ctx, cutoff); err != nil {
		log.Printf("prune analytics: %v", err)
		return
	}
	log.Printf("pruned analytics older than %s", cutoff.Format("2006-01-02"))
}

func openPostgres(ctx context.Context, dsn string) *gorm.DB {
	var db *gorm.DB
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:         gormlogger.Default.LogMode(gormlogger.Warn),
			TranslateError: true,
		})
		if err == nil {
			var sqlDB *sql.DB
			if sqlDB, err = db.DB(); err == nil {
				err = sqlDB.PingContext(ctx)
			}
		}
		if err == nil {
			return db
		}
		log.Printf("waiting for postgres: %v", err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("postgres never became ready: %v", err)
	return nil
}
