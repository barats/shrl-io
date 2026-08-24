package store

import (
	"context"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func teamIDPtr(id int64) *int64 { return &id }

func TestLinkStorePersonalListExcludesTeamLinks(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()

	tid := int64(10)
	links := []*domain.Link{
		{Hostname: "localhost", Code: "one", Destination: "https://a.example", CreatedBy: 1},
		{Hostname: "localhost", Code: "two", Destination: "https://b.example", CreatedBy: 1, TeamID: teamIDPtr(tid)},
		{Hostname: "localhost", Code: "three", Destination: "https://c.example", CreatedBy: 2},
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
		t.Fatalf("personal list = %+v, want only 'one'", got)
	}
}

func TestLinkStoreListByTeam(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()

	tid := int64(10)
	other := int64(11)
	links := []*domain.Link{
		{Hostname: "localhost", Code: "one", Destination: "https://a.example", CreatedBy: 1, TeamID: teamIDPtr(tid)},
		{Hostname: "localhost", Code: "two", Destination: "https://b.example", CreatedBy: 2, TeamID: teamIDPtr(tid)},
		{Hostname: "localhost", Code: "three", Destination: "https://c.example", CreatedBy: 1, TeamID: teamIDPtr(other)},
		{Hostname: "localhost", Code: "four", Destination: "https://d.example", CreatedBy: 1},
	}
	for _, l := range links {
		if err := s.Create(ctx, l); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, err := s.ListByTeam(ctx, "localhost", tid)
	if err != nil {
		t.Fatalf("list by team: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("team list = %+v, want 2", got)
	}
	for _, l := range got {
		if l.TeamID == nil || *l.TeamID != tid {
			t.Fatalf("team list leaked wrong scope: %+v", l)
		}
	}
}

func TestLinkStoreTransferTeamLinksToPersonal(t *testing.T) {
	db := newTestDB(t)
	s := NewLinkStore(db)
	ctx := context.Background()

	tid := int64(10)
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "one", Destination: "https://a.example", CreatedBy: 1, TeamID: teamIDPtr(tid)})
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "two", Destination: "https://b.example", CreatedBy: 2, TeamID: teamIDPtr(tid)})
	_ = s.Create(ctx, &domain.Link{Hostname: "localhost", Code: "three", Destination: "https://c.example", CreatedBy: 3})

	if err := s.TransferTeamLinksToPersonal(ctx, tid); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	// team links became personal to their creators
	for _, c := range []string{"one", "two"} {
		l, err := s.Get(ctx, "localhost", c)
		if err != nil {
			t.Fatalf("get %s: %v", c, err)
		}
		if l.TeamID != nil {
			t.Fatalf("%s still has team %d", c, *l.TeamID)
		}
	}
	// the unrelated personal link is untouched
	l, err := s.Get(ctx, "localhost", "three")
	if err != nil {
		t.Fatalf("get three: %v", err)
	}
	if l.TeamID != nil {
		t.Fatalf("three changed scope")
	}
}
