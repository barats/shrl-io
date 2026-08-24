package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

func TestLinkStoreCreateGet(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()

	l := &domain.Link{
		Hostname: "localhost", Code: "abc123", Destination: "https://example.com",
		Remark: "promo", CreatedBy: 7,
	}
	if err := s.Create(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Get(ctx, "localhost", "abc123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Destination != "https://example.com" || got.Remark != "promo" || got.CreatedBy != 7 {
		t.Fatalf("unexpected link: %+v", got)
	}
}

func TestLinkStoreGetNotFound(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	_, err := s.Get(context.Background(), "localhost", "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLinkStoreDuplicateKey(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	if err := s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "abc123", Destination: "https://a.example"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	err := s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "abc123", Destination: "https://b.example"})
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
			Hostname: "localhost", Code: code, Destination: "https://x.example",
			CreatedBy: 1, CreatedAt: base.Add(time.Duration(i) * time.Hour),
		}
		if err := s.Create(ctx, l); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	links, err := s.List(ctx, "localhost", 1)
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

func TestLinkStoreListScopedByCreatorAndHostname(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	links := []*domain.Link{
		{Hostname: "localhost", Code: "one", Destination: "https://a.example", CreatedBy: 1},
		{Hostname: "localhost", Code: "two", Destination: "https://b.example", CreatedBy: 2},
		{Hostname: "other.com", Code: "one", Destination: "https://c.example", CreatedBy: 1},
	}
	for _, l := range links {
		if err := s.Create(ctx, l); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, err := s.List(ctx, "localhost", 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Code != "one" {
		t.Fatalf("list = %+v, want only creator 1 on localhost", got)
	}
}

func TestLinkStoreSavePersistsRemark(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()
	l := &domain.Link{Hostname: "localhost", Code: "abc123", Destination: "https://a.example", Remark: "first"}
	if err := s.Create(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}
	l.Destination = "https://b.example"
	l.Remark = "second"
	if err := s.Save(ctx, l); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Get(ctx, "localhost", "abc123")
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
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "one", Destination: "https://a.example"})
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "two", Destination: "https://b.example", Disabled: true})
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
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "abc123", Destination: "https://a.example"})
	if err := s.Delete(ctx, "localhost", "abc123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "localhost", "abc123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}
