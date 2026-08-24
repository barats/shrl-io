package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

func TestUserStoreCRUD(t *testing.T) {
	db := newTestDB(t)
	s := NewUserStore(db)
	ctx := context.Background()

	if n, err := s.Count(ctx); err != nil || n != 0 {
		t.Fatalf("count = %d, %v (want 0)", n, err)
	}
	u := &domain.User{Username: "admin", PasswordHash: "hash", IsAdmin: true}
	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("id not assigned")
	}
	// duplicate username is rejected
	if err := s.Create(ctx, &domain.User{Username: "admin", PasswordHash: "x"}); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate err = %v, want ErrDuplicatedKey", err)
	}
	if got, err := s.GetByUsername(ctx, "admin"); err != nil || got.ID != u.ID || !got.IsAdmin {
		t.Fatalf("get by username = %+v, %v", got, err)
	}
	if got, err := s.GetByID(ctx, u.ID); err != nil || got.Username != "admin" {
		t.Fatalf("get by id = %+v, %v", got, err)
	}
}

func TestUserStoreTokens(t *testing.T) {
	db := newTestDB(t)
	s := NewUserStore(db)
	ctx := context.Background()

	u := &domain.User{Username: "u"}
	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	tk := &domain.Token{UserID: u.ID, Hash: "deadbeef", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateToken(ctx, tk); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if got, err := s.TokenByHash(ctx, "deadbeef"); err != nil || got.ID != tk.ID {
		t.Fatalf("token by hash = %+v, %v", got, err)
	}
	if _, err := s.TokenByHash(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing token err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteToken(ctx, tk.ID); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	if _, err := s.TokenByHash(ctx, "deadbeef"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token after delete err = %v, want ErrNotFound", err)
	}
}

func TestUserStoreAssignLinksToCreator(t *testing.T) {
	db := newTestDB(t)
	ls := NewLinkStore(db)
	us := NewUserStore(db)
	ctx := context.Background()

	// created_by is 0 for both of these (pre-ownership backfill targets)
	_ = ls.Create(ctx, &domain.Link{Hostname: "localhost", Code: "one", Destination: "https://a.example"})
	_ = ls.Create(ctx, &domain.Link{Hostname: "localhost", Code: "two", Destination: "https://b.example", CreatedBy: 5})

	u := &domain.User{Username: "admin"}
	if err := us.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := us.AssignLinksToCreator(ctx, u.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	links, err := ls.List(ctx, "localhost", u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 1 || links[0].Code != "one" {
		t.Fatalf("backfilled links = %+v, want only 'one'", links)
	}
}
