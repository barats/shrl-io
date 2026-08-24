package store

import (
	"context"
	"errors"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestHostnameStoreCRUD(t *testing.T) {
	db := newTestDB(t)
	s := NewHostnameStore(db)
	ctx := context.Background()

	if err := s.Create(ctx, &domain.Hostname{Name: "example.com", RegisteredBy: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// duplicate registration is rejected
	if err := s.Create(ctx, &domain.Hostname{Name: "example.com", RegisteredBy: 2}); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate err = %v, want ErrDuplicatedKey", err)
	}
	// get by name
	h, err := s.Get(ctx, "example.com")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if h.RegisteredBy != 1 {
		t.Fatalf("registered_by = %d, want 1", h.RegisteredBy)
	}
	// missing hostname
	if _, err := s.Get(ctx, "nope.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}
	// list is sorted
	_ = s.Create(ctx, &domain.Hostname{Name: "b.com"})
	_ = s.Create(ctx, &domain.Hostname{Name: "a.com"})
	hs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(hs) != 3 || hs[0].Name != "a.com" || hs[1].Name != "b.com" || hs[2].Name != "example.com" {
		t.Fatalf("list = %+v", hs)
	}
	// delete removes it from the registry
	if err := s.Delete(ctx, "example.com"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}
