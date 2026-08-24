package store

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB opens a unique in-memory sqlite database per test and migrates the
// same models the stores use in production. The store queries are
// dialect-neutral, so sqlite covers the CRUD and read paths; the Postgres-only
// upsert path (ApplyAnalytics) stays covered by the integration stack.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	for _, migrate := range []func(context.Context) error{
		NewLinkStore(db).Migrate,
		NewHostnameStore(db).Migrate,
		NewUserStore(db).Migrate,
		NewAnalyticsStore(db).Migrate,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}
