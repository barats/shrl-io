package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

func TestAPIKeyStore(t *testing.T) {
	db := newTestDB(t)
	s := NewUserStore(db)
	ctx := context.Background()

	u1 := &domain.User{Username: "a"}
	u2 := &domain.User{Username: "b"}
	if err := s.Create(ctx, u1); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if err := s.Create(ctx, u2); err != nil {
		t.Fatalf("create b: %v", err)
	}

	k1 := &domain.APIKey{UserID: u1.ID, Name: "ci", Hash: "h1"}
	if err := s.CreateKey(ctx, k1); err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := s.CreateKey(ctx, &domain.APIKey{UserID: u1.ID, Name: "old", Hash: "h0"}); err != nil {
		t.Fatalf("create old key: %v", err)
	}

	if got, err := s.KeyByHash(ctx, "h1"); err != nil || got.Name != "ci" || got.UserID != u1.ID {
		t.Fatalf("key by hash = %+v, %v", got, err)
	}
	if _, err := s.KeyByHash(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key err = %v, want ErrNotFound", err)
	}

	// list is newest first ("old" was created after "ci")
	keys, err := s.ListKeys(ctx, u1.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 2 || keys[0].Name != "old" || keys[1].Name != "ci" {
		t.Fatalf("listed keys = %+v, want [old ci] newest-first", keys)
	}

	// a user cannot revoke another user's key
	if err := s.DeleteKey(ctx, k1.ID, u2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete other's key err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteKey(ctx, k1.ID, u1.ID); err != nil {
		t.Fatalf("delete own key: %v", err)
	}
	if _, err := s.KeyByHash(ctx, "h1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted key still resolvable: %v", err)
	}

	// deleting for a user removes all of theirs
	_ = s.CreateKey(ctx, &domain.APIKey{UserID: u2.ID, Name: "x", Hash: "h2"})
	if err := s.DeleteKeysForUser(ctx, u2.ID); err != nil {
		t.Fatalf("delete keys for user: %v", err)
	}
	if _, err := s.KeyByHash(ctx, "h2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("key after delete-for-user still resolvable: %v", err)
	}
}

func TestUserStorePasswordState(t *testing.T) {
	db := newTestDB(t)
	s := NewUserStore(db)
	ctx := context.Background()

	u := &domain.User{Username: "u"}
	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.SetPassword(ctx, u.ID, "newhash"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	got, err := s.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.PasswordHash != "newhash" || got.MustChangePassword {
		t.Fatalf("after SetPassword = %+v, want newhash and no forced change", got)
	}

	if err := s.RequirePasswordChange(ctx, u.ID); err != nil {
		t.Fatalf("require change: %v", err)
	}
	got, _ = s.GetByID(ctx, u.ID)
	if !got.MustChangePassword {
		t.Fatal("MustChangePassword not set after RequirePasswordChange")
	}

	// DeleteTokensForUserExcept keeps only the named token
	_ = s.CreateToken(ctx, &domain.Token{UserID: u.ID, Hash: "t1", ExpiresAt: time.Now().Add(time.Hour)})
	_ = s.CreateToken(ctx, &domain.Token{UserID: u.ID, Hash: "t2", ExpiresAt: time.Now().Add(time.Hour)})
	if err := s.DeleteTokensForUserExcept(ctx, u.ID, "t1"); err != nil {
		t.Fatalf("delete tokens except: %v", err)
	}
	if _, err := s.TokenByHash(ctx, "t1"); err != nil {
		t.Fatalf("kept token lost: %v", err)
	}
	if _, err := s.TokenByHash(ctx, "t2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-kept token survived: %v", err)
	}

	// DeleteTokensForUser removes everything
	if err := s.DeleteTokensForUser(ctx, u.ID); err != nil {
		t.Fatalf("delete tokens: %v", err)
	}
	if _, err := s.TokenByHash(ctx, "t1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token after delete-all survived: %v", err)
	}
}
