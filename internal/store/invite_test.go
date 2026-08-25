package store

import (
	"context"
	"errors"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestInviteStoreLifecycle(t *testing.T) {
	db := newTestDB(t)
	s := NewInviteStore(db)
	ts := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := ts.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	inv := &domain.InviteCode{TeamID: tm.ID, Code: "ABC12345", CreatedBy: 1}
	if err := s.Create(ctx, inv); err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if inv.ID == 0 {
		t.Fatal("id not assigned")
	}
	// duplicate code is rejected
	if err := s.Create(ctx, &domain.InviteCode{TeamID: tm.ID, Code: "ABC12345", CreatedBy: 1}); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate code err = %v, want ErrDuplicatedKey", err)
	}

	invites, err := s.ListOutstanding(ctx, tm.ID)
	if err != nil || len(invites) != 1 || invites[0].Code != "ABC12345" {
		t.Fatalf("outstanding = %+v, %v (want 1)", invites, err)
	}
	// revoke an unknown code is a no-op
	if ok, _ := s.Revoke(ctx, tm.ID, "ZZZ99999"); ok {
		t.Fatal("revoke unknown code = true, want false")
	}
	if ok, err := s.Revoke(ctx, tm.ID, "ABC12345"); err != nil || !ok {
		t.Fatalf("revoke = %v, %v (want true, nil)", ok, err)
	}
	if invites, _ := s.ListOutstanding(ctx, tm.ID); len(invites) != 0 {
		t.Fatalf("outstanding after revoke = %+v, want none", invites)
	}
}

func TestInviteStoreJoinByCode(t *testing.T) {
	db := newTestDB(t)
	s := NewInviteStore(db)
	ts := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := ts.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := ts.AddMember(ctx, tm.ID, 1, domain.RoleOwner); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	inv := &domain.InviteCode{TeamID: tm.ID, Code: "ABC12345", CreatedBy: 1}
	if err := s.Create(ctx, inv); err != nil {
		t.Fatalf("create invite: %v", err)
	}

	// user 2 joins via the code
	teamID, err := s.JoinByCode(ctx, "ABC12345", 2)
	if err != nil || teamID != tm.ID {
		t.Fatalf("join = %d, %v (want %d)", teamID, err, tm.ID)
	}
	if role, err := ts.MemberRole(ctx, tm.ID, 2); err != nil || role != domain.RoleMember {
		t.Fatalf("joined role = %q, %v", role, err)
	}

	// the code is single-use: user 3 gets ErrInvalidInvite
	if _, err := s.JoinByCode(ctx, "ABC12345", 3); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("second join err = %v, want ErrInvalidInvite", err)
	}
	// an unknown code is rejected
	if _, err := s.JoinByCode(ctx, "NOPE0000", 3); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("unknown code err = %v, want ErrInvalidInvite", err)
	}

	// an existing member joining leaves the code unconsumed
	inv2 := &domain.InviteCode{TeamID: tm.ID, Code: "DEF67890", CreatedBy: 1}
	if err := s.Create(ctx, inv2); err != nil {
		t.Fatalf("create invite2: %v", err)
	}
	if _, err := s.JoinByCode(ctx, "DEF67890", 2); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("member join err = %v, want ErrDuplicatedKey", err)
	}
	if _, err := s.JoinByCode(ctx, "DEF67890", 3); err != nil {
		t.Fatalf("join after member-reject err = %v", err)
	}
}
