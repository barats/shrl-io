package store

import (
	"context"
	"errors"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestTeamStoreCRUD(t *testing.T) {
	db := newTestDB(t)
	s := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := s.Create(ctx, tm); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tm.ID == 0 {
		t.Fatal("id not assigned")
	}
	if err := s.Create(ctx, &domain.Team{Name: "growth", CreatedBy: 1}); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate err = %v, want ErrDuplicatedKey", err)
	}
	got, err := s.Get(ctx, tm.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "growth" || got.CreatedBy != 1 {
		t.Fatalf("unexpected team: %+v", got)
	}
	if _, err := s.Get(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing team err = %v, want ErrNotFound", err)
	}
}

func TestTeamStoreMembership(t *testing.T) {
	db := newTestDB(t)
	s := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := s.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := s.AddMember(ctx, tm.ID, 1, domain.RoleOwner); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if err := s.AddMember(ctx, tm.ID, 2, domain.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.AddMember(ctx, tm.ID, 2, domain.RoleMember); !errors.Is(err, ErrDuplicatedKey) {
		t.Fatalf("duplicate member err = %v, want ErrDuplicatedKey", err)
	}

	if role, err := s.MemberRole(ctx, tm.ID, 1); err != nil || role != domain.RoleOwner {
		t.Fatalf("role(user 1) = %q, %v", role, err)
	}
	if role, err := s.MemberRole(ctx, tm.ID, 2); err != nil || role != domain.RoleMember {
		t.Fatalf("role(user 2) = %q, %v", role, err)
	}
	if _, err := s.MemberRole(ctx, tm.ID, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("role(user 3) err = %v, want ErrNotFound", err)
	}

	// the member's team is listed for them; an outsider sees nothing
	teams, err := s.ListForUser(ctx, 2)
	if err != nil {
		t.Fatalf("list for user: %v", err)
	}
	if len(teams) != 1 || teams[0].ID != tm.ID {
		t.Fatalf("list for user 2 = %+v, want team %d", teams, tm.ID)
	}
	outsider, err := s.ListForUser(ctx, 99)
	if err != nil {
		t.Fatalf("list for outsider: %v", err)
	}
	if len(outsider) != 0 {
		t.Fatalf("outsider teams = %+v, want none", outsider)
	}

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("list all = %+v, want 1", all)
	}

	members, err := s.ListMembers(ctx, tm.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %+v, want 2", members)
	}
}

func TestTeamStoreRolesAndOwnerCount(t *testing.T) {
	db := newTestDB(t)
	s := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := s.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	_ = s.AddMember(ctx, tm.ID, 1, domain.RoleOwner)
	_ = s.AddMember(ctx, tm.ID, 2, domain.RoleMember)

	if err := s.SetRole(ctx, tm.ID, 2, domain.RoleOwner); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if n, err := s.CountOwners(ctx, tm.ID); err != nil || n != 2 {
		t.Fatalf("owners = %d, %v (want 2)", n, err)
	}
	if err := s.SetRole(ctx, tm.ID, 2, domain.RoleMember); err != nil {
		t.Fatalf("demote: %v", err)
	}
	if n, err := s.CountOwners(ctx, tm.ID); err != nil || n != 1 {
		t.Fatalf("owners = %d, %v (want 1)", n, err)
	}
}

func TestTeamStoreRemoveMember(t *testing.T) {
	db := newTestDB(t)
	s := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := s.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	_ = s.AddMember(ctx, tm.ID, 2, domain.RoleMember)

	ok, err := s.RemoveMember(ctx, tm.ID, 2)
	if err != nil || !ok {
		t.Fatalf("remove member = %v, %v (want true, nil)", ok, err)
	}
	if _, err := s.MemberRole(ctx, tm.ID, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("member after remove err = %v, want ErrNotFound", err)
	}
	ok, err = s.RemoveMember(ctx, tm.ID, 2)
	if err != nil || ok {
		t.Fatalf("remove missing member = %v, %v (want false, nil)", ok, err)
	}
}

func TestTeamStoreDeleteRemovesMemberships(t *testing.T) {
	db := newTestDB(t)
	s := NewTeamStore(db)
	ctx := context.Background()

	tm := &domain.Team{Name: "growth", CreatedBy: 1}
	if err := s.Create(ctx, tm); err != nil {
		t.Fatalf("create team: %v", err)
	}
	_ = s.AddMember(ctx, tm.ID, 1, domain.RoleOwner)
	_ = s.AddMember(ctx, tm.ID, 2, domain.RoleMember)

	if err := s.Delete(ctx, tm.ID); err != nil {
		t.Fatalf("delete team: %v", err)
	}
	if _, err := s.Get(ctx, tm.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("team after delete err = %v, want ErrNotFound", err)
	}
	if members, _ := s.ListMembers(ctx, tm.ID); len(members) != 0 {
		t.Fatalf("members after delete = %+v, want none", members)
	}
}
