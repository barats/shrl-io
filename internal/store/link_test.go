package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/barats/shrl-io/internal/domain"
)

func TestLinkStoreCreateGet(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()

	l := &domain.Link{
		BaseURL: "http://localhost:8080", Code: "abc123", Destination: "https://example.com",
		Remark: "promo", CreatedBy: 7,
	}
	if err := s.Create(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.BaseURL != "http://localhost:8080" || got.Destination != "https://example.com" || got.Remark != "promo" || got.CreatedBy != 7 {
		t.Fatalf("unexpected link: %+v", got)
	}
}

func TestLinkStoreGetNotFound(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	_, err := s.Get(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLinkStoreDuplicateKey(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	// Codes are globally unique: the same Code on a different Base URL still
	// collides.
	if err := s.Create(ctx, &domain.Link{BaseURL: "http://localhost:8080", Code: "abc123", Destination: "https://a.example"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	err := s.Create(ctx, &domain.Link{BaseURL: "https://other.com", Code: "abc123", Destination: "https://b.example"})
	if !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("err = %v, want ErrDuplicatedKey", err)
	}
}

func TestLinkStoreListNewestFirst(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for i, code := range []string{"aaa", "bbb", "ccc"} {
		l := &domain.Link{
			BaseURL: "http://localhost:8080", Code: code, Destination: "https://x.example",
			CreatedBy: 1, CreatedAt: base.Add(time.Duration(i) * time.Hour),
		}
		if err := s.Create(ctx, l); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	links, err := s.List(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("len = %d, want 3", len(links))
	}
	if links[0].Code != "ccc" || links[2].Code != "aaa" {
		t.Fatalf("order = %s,%s,%s, want ccc,bbb,aaa", links[0].Code, links[1].Code, links[2].Code)
	}
}

func TestLinkStoreListScopedByCreatorAcrossBaseURLs(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	links := []*domain.Link{
		{BaseURL: "http://localhost:8080", Code: "one", Destination: "https://a.example", CreatedBy: 1},
		{BaseURL: "http://localhost:8080", Code: "two", Destination: "https://b.example", CreatedBy: 2},
		{BaseURL: "https://other.com", Code: "three", Destination: "https://c.example", CreatedBy: 1},
	}
	for _, l := range links {
		if err := s.Create(ctx, l); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, err := s.List(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("list = %+v, want creator 1's two links across base URLs", got)
	}
	for _, l := range got {
		if l.CreatedBy != 1 {
			t.Fatalf("list leaked another creator's link: %+v", l)
		}
	}
}

func TestLinkStoreSavePersistsRemark(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	l := &domain.Link{BaseURL: "http://localhost:8080", Code: "abc123", Destination: "https://a.example", Remark: "first"}
	if err := s.Create(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}
	l.Destination = "https://b.example"
	l.Remark = "second"
	if err := s.Save(ctx, l); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Destination != "https://b.example" || got.Remark != "second" {
		t.Fatalf("save did not persist: %+v", got)
	}
}

func TestLinkStoreListActiveExcludesDisabled(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	_ = s.Create(ctx, &domain.Link{BaseURL: "http://localhost:8080", Code: "one", Destination: "https://a.example"})
	_ = s.Create(ctx, &domain.Link{BaseURL: "http://localhost:8080", Code: "two", Destination: "https://b.example", Disabled: true})
	active, err := s.ListActive(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].Code != "one" {
		t.Fatalf("active = %+v, want just 'one'", active)
	}
}

func TestLinkStoreDelete(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	_ = s.Create(ctx, &domain.Link{BaseURL: "http://localhost:8080", Code: "abc123", Destination: "https://a.example"})
	if err := s.Delete(ctx, "abc123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "abc123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

// TestLinkStoreMigrateRebuildsLegacyCompositeKey verifies the ADR 0019
// migration: a table still using the (Hostname, Code) composite key is dropped
// and recreated with Code as the sole primary key, clearing legacy data.
func TestLinkStoreMigrateRebuildsLegacyCompositeKey(t *testing.T) {
	// Raw sqlite DB with no migrations, so a genuine legacy table can be built.
	db, err := gorm.Open(sqlite.Open("file:legacy?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Legacy model: composite (Hostname, Code) primary key, table "links".
	type legacyLink struct {
		Hostname    string `gorm:"primaryKey"`
		Code        string `gorm:"primaryKey"`
		Destination string
	}
	if err := db.AutoMigrate(&legacyLink{}); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Create(&legacyLink{
		Hostname: "localhost", Code: "abc", Destination: "https://example.com",
	}).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	s := NewLinkStore(db)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The legacy row is gone (data cleared by decision).
	if _, err := s.Get(context.Background(), "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy row should be cleared, got err=%v", err)
	}
	// The new schema has a single primary-key column.
	cols, err := db.Migrator().ColumnTypes(&domain.Link{})
	if err != nil {
		t.Fatalf("column types: %v", err)
	}
	keys := 0
	for _, c := range cols {
		if pk, ok := c.PrimaryKey(); ok && pk {
			keys++
		}
	}
	if keys != 1 {
		t.Fatalf("primary key columns = %d, want 1 (Code)", keys)
	}
}

func TestLinkStoreMigrateIdempotentOnFreshTable(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// A second migrate (e.g. every boot) must not drop data.
	if err := s.Create(ctx, &domain.Link{BaseURL: "http://localhost:8080", Code: "abc123", Destination: "https://a.example"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if _, err := s.Get(ctx, "abc123"); err != nil {
		t.Fatalf("data lost on second migrate: %v", err)
	}
}
