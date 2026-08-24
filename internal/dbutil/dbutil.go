package dbutil

import (
	"context"
	"database/sql"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/barats/shrl-io/internal/env"
)

// Config controls the Postgres connection pool via GORM. Zero values fall back
// to database/sql defaults (unlimited open connections, 2 idle).
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// ConfigFromEnv builds a pool config from SHRL_DB_* env vars with defaults
// sized for a self-hosted instance: 20 open, 5 idle, 30m lifetime, 5m idle.
func ConfigFromEnv(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxOpenConns:    env.Int("SHRL_DB_MAX_OPEN_CONNS", 20),
		MaxIdleConns:    env.Int("SHRL_DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: env.Duration("SHRL_DB_CONN_MAX_LIFETIME", 30*time.Minute),
		ConnMaxIdleTime: env.Duration("SHRL_DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
	}
}

// Open connects to Postgres via GORM, waits for it to be ready, and applies
// the connection pool settings.
func Open(ctx context.Context, cfg Config) *gorm.DB {
	var db *gorm.DB
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		db, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
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
			applyPool(db, cfg)
			log.Printf("postgres connected: max_open=%d max_idle=%d lifetime=%s idle_time=%s",
				cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)
			return db
		}
		log.Printf("waiting for postgres: %v", err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("postgres never became ready: %v", err)
	return nil
}

func applyPool(db *gorm.DB, cfg Config) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}
