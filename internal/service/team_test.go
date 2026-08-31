package service

import (
	"context"
	"errors"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestRenameTeam(t *testing.T) {
	svc, _, _ := newTestServiceFull(t)
	ctx := context.Background()
	admin := &domain.User{ID: 1, Username: "admin", IsAdmin: true}
	alice := &domain.User{ID: 2, Username: "alice"}
	bob := &domain.User{ID: 3, Username: "bob"}

	team := &domain.Team{Name: "growth"}
	if err := svc.teams.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := svc.teams.AddMember(ctx, team.ID, alice.ID, domain.RoleOwner); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if err := svc.teams.AddMember(ctx, team.ID, bob.ID, domain.RoleMember); err != nil {
		t.Fatalf("add bob: %v", err)
	}
	dupe := &domain.Team{Name: "dupe"}
	if err := svc.teams.Create(ctx, dupe); err != nil {
		t.Fatalf("create dupe team: %v", err)
	}

	// the owner renames
	if err := svc.RenameTeam(ctx, alice, team.ID, "Growth v2"); err != nil {
		t.Fatalf("owner rename: %v", err)
	}
	got, err := svc.teams.Get(ctx, team.ID)
	if err != nil || got.Name != "Growth v2" {
		t.Fatalf("renamed team = %+v, %v", got, err)
	}

	// a plain member cannot rename
	if err := svc.RenameTeam(ctx, bob, team.ID, "nope"); !errors.Is(err, ErrForbidden) {
		t.Errorf("member rename = %v, want ErrForbidden", err)
	}

	// an outsider gets ErrNotFound so team existence is not leaked
	if err := svc.RenameTeam(ctx, &domain.User{ID: 99, Username: "stranger"}, team.ID, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("outsider rename = %v, want ErrNotFound", err)
	}

	// an admin not in the team can rename (instance oversight)
	if err := svc.RenameTeam(ctx, admin, team.ID, "Growth Admin"); err != nil {
		t.Fatalf("admin rename: %v", err)
	}

	// a duplicate name conflicts
	if err := svc.RenameTeam(ctx, alice, team.ID, "dupe"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate rename = %v, want ErrConflict", err)
	}

	// a blank name is a validation error
	if err := svc.RenameTeam(ctx, alice, team.ID, "   "); err == nil {
		t.Error("blank name should be a validation error")
	}
}
