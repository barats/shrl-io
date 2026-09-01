package store

import (
	"context"
	"errors"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestBaseURLStoreCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	s := NewBaseURLStore(db)
	if err := s.Create(ctx, &domain.BaseURL{BaseURL: "https://example.com", RegisteredBy: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, &domain.BaseURL{BaseURL: "https://example.com", RegisteredBy: 2}); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate create err = %v, want ErrDuplicatedKey", err)
	}
	if got, err := s.Get(ctx, "https://example.com"); err != nil || got.BaseURL != "https://example.com" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := s.Get(ctx, "https://nope.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, "https://example.com"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "https://example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

func TestBaseURLStoreListSorted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	s := NewBaseURLStore(db)
	_ = s.Create(ctx, &domain.BaseURL{BaseURL: "https://b.com"})
	_ = s.Create(ctx, &domain.BaseURL{BaseURL: "https://a.com"})
	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].BaseURL != "https://a.com" || got[1].BaseURL != "https://b.com" {
		t.Fatalf("list = %+v, want sorted [a.com b.com]", got)
	}
}
