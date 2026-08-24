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
	"github.com/barats/shrl-io/internal/geo"
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
	analyticsStore := store.NewAnalyticsStore(db)
	if err := analyticsStore.Migrate(ctx); err != nil {
		log.Fatalf("migrate analytics: %v", err)
	}

	analyticsCache := cache.NewAnalyticsCache(rdb)
	if err := analyticsCache.EnsureVisitGroup(ctx); err != nil {
		log.Fatalf("create visit group: %v", err)
	}

	proc := &analytics.Processor{Cache: analyticsCache, Store: analyticsStore}
	if resolver := setupGeo(ctx); resolver != nil {
		proc.Geo = resolver
		defer resolver.Close()
	}

	// Retention prune: at boot, then nightly.
	go func() {
		prune(ctx, analyticsStore, cfg.retentionDays)
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for range t.C {
			prune(ctx, analyticsStore, cfg.retentionDays)
		}
	}()

	// Consume the visits stream in batches. Unacked batches are redelivered
	// after a crash (at-least-once), the accepted double-count window.
	go func() {
		for {
			// Recover this consumer's in-flight batch from a previous crash.
			msgs, err := analyticsCache.ReadVisitsPending(ctx, cfg.consumer, 1000)
			if err != nil {
				log.Printf("read pending visits: %v", err)
			} else if len(msgs) > 0 {
				if err := processBatch(ctx, proc, analyticsCache, msgs); err != nil {
					log.Printf("process pending batch: %v", err)
				}
				continue
			}

			msgs, err = analyticsCache.ReadVisitsNew(ctx, cfg.consumer, 1000, 2*time.Second)
			if err != nil {
				log.Printf("read visits: %v", err)
				time.Sleep(time.Second)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			if err := processBatch(ctx, proc, analyticsCache, msgs); err != nil {
				log.Printf("process batch: %v", err)
			}
		}
	}()

	log.Printf("worker started: consumer=%s retention=%dd", cfg.consumer, cfg.retentionDays)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func processBatch(ctx context.Context, proc *analytics.Processor, ca *cache.AnalyticsCache, msgs []redis.XMessage) error {
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

// setupGeo enables GeoIP attribution if a database file is mounted or a
// MaxMind license key is configured; otherwise geo is disabled and locations
// are "unknown". When enabled it starts a periodic refresh.
func setupGeo(ctx context.Context) *geo.Resolver {
	dbPath := envOr("SHRL_GEOLITE_DB_PATH", "/data/GeoLite2-City.mmdb")
	licenseKey := os.Getenv("SHRL_GEOLITE_LICENSE")

	var r *geo.Resolver
	switch {
	case fileExists(dbPath):
		r, _ = geo.Open(dbPath)
	case licenseKey != "":
		if err := geo.Ensure(ctx, dbPath, licenseKey); err == nil {
			r, _ = geo.Open(dbPath)
		} else {
			log.Printf("geo: download failed: %v", err)
		}
	}
	if r == nil {
		log.Println("geo: disabled; locations will be 'unknown' (set SHRL_GEOLITE_LICENSE or mount a database)")
		return nil
	}
	log.Printf("geo: enabled (%s)", dbPath)
	go func() {
		t := time.NewTicker(geo.UpdateInterval)
		defer t.Stop()
		for range t.C {
			if err := geo.Update(ctx, dbPath, licenseKey); err != nil {
				log.Printf("geo: update failed: %v", err)
			}
		}
	}()
	return r
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func prune(ctx context.Context, st *store.AnalyticsStore, retentionDays int) {	cutoff := time.Now().AddDate(0, 0, -retentionDays)
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
