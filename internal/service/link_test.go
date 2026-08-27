package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

func newTestService(t *testing.T) (*LinkService, *cache.LinkCache) {
	svc, _, lc := newTestServiceFull(t)
	return svc, lc
}

// newTestServiceFull is the shared builder; it also returns the raw db so
// tests can seed analytics rollups directly.
func newTestServiceFull(t *testing.T) (*LinkService, *gorm.DB, *cache.LinkCache) {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	for _, migrate := range []func(context.Context) error{
		store.NewLinkStore(db).Migrate,
		store.NewHostnameStore(db).Migrate,
		store.NewUserStore(db).Migrate,
		store.NewTeamStore(db).Migrate,
		store.NewAnalyticsStore(db).Migrate,
		store.NewSettingStore(db).Migrate,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	hs := store.NewHostnameStore(db)
	if err := hs.Create(ctx, &domain.Hostname{Name: "shrl.io"}); err != nil {
		t.Fatalf("register hostname: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	links := store.NewLinkStore(db)
	analytics := store.NewAnalyticsStore(db)
	teams := store.NewTeamStore(db)
	settings := store.NewSettingStore(db)
	lc := cache.NewLinkCache(client)
	svc := NewLinkService(links, analytics, hs, teams, settings, lc, "shrl.io", 30)
	return svc, db, lc
}

func TestCreateLinkWritesCache(t *testing.T) {
	svc, lc := newTestService(t)
	ctx := context.Background()

	l, err := svc.CreateLink(ctx, nil, 1, CreateLinkInput{Destination: "https://example.com"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if l.Hostname != "shrl.io" {
		t.Errorf("hostname = %q, want default shrl.io", l.Hostname)
	}
	if l.Code == "" {
		t.Error("code should be auto-generated")
	}
	got, ok, err := lc.Get(ctx, l.Hostname, l.Code)
	if err != nil || !ok {
		t.Fatalf("cache Get: ok=%v err=%v", ok, err)
	}
	if got.Destination != "https://example.com" {
		t.Errorf("cached destination = %q", got.Destination)
	}
}

func TestCreateLinkValidation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateLink(ctx, nil, 1, CreateLinkInput{Hostname: "nope.io", Destination: "https://example.com"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Msg != "hostname is not registered" {
		t.Fatalf("unregistered hostname err = %v, want validation 'hostname is not registered'", err)
	}
	if _, err := svc.CreateLink(ctx, nil, 1, CreateLinkInput{Destination: ""}); !errors.As(err, &ve) {
		t.Fatalf("empty destination err = %v, want validation", err)
	}
}

func TestPersonalLinkAccessAndCache(t *testing.T) {
	svc, lc := newTestService(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}
	bob := &domain.User{ID: 2, Username: "bob"}

	l, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetLink(ctx, alice, l.Code); err != nil {
		t.Fatalf("alice get: %v", err)
	}
	upd, err := svc.UpdateLink(ctx, alice, l.Code, UpdateLinkInput{Destination: "https://new.example.com"})
	if err != nil {
		t.Fatalf("alice update: %v", err)
	}
	if upd.Destination != "https://new.example.com" {
		t.Errorf("updated destination = %q", upd.Destination)
	}
	if got, ok, _ := lc.Get(ctx, l.Hostname, l.Code); !ok || got.Destination != "https://new.example.com" {
		t.Errorf("cache not refreshed after update: ok=%v dest=%q", ok, got.Destination)
	}

	// bob is an outsider: existence is not leaked
	if _, err := svc.GetLink(ctx, bob, l.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob get err = %v, want ErrNotFound", err)
	}
	if _, err := svc.UpdateLink(ctx, bob, l.Code, UpdateLinkInput{Destination: "https://evil.example.com"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob update err = %v, want ErrNotFound", err)
	}

	// disabling evicts the redirect cache
	if _, err := svc.SetDisabled(ctx, alice, l.Code, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, ok, err := lc.Get(ctx, l.Hostname, l.Code); err != nil || ok {
		t.Fatalf("disabled link should be evicted: ok=%v err=%v", ok, err)
	}

	// deleting removes it from the store and cache
	if err := svc.DeleteLink(ctx, alice, l.Code); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetLink(ctx, alice, l.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

func TestTeamLinkPermissions(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}
	bob := &domain.User{ID: 2, Username: "bob"}
	carol := &domain.User{ID: 3, Username: "carol", IsAdmin: true}

	team := &domain.Team{Name: "growth", CreatedBy: alice.ID}
	if err := svc.teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	if err := svc.teams.AddMember(ctx, team.ID, alice.ID, domain.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if err := svc.teams.AddMember(ctx, team.ID, bob.ID, domain.RoleMember); err != nil {
		t.Fatal(err)
	}

	// the owner creates a team link
	l, err := svc.CreateLink(ctx, &team.ID, alice.ID, CreateLinkInput{Destination: "https://example.com"})
	if err != nil {
		t.Fatalf("create team link: %v", err)
	}
	if l.TeamID == nil || *l.TeamID != team.ID {
		t.Fatalf("team id = %v, want %d", l.TeamID, team.ID)
	}

	// bob (member) and carol (admin) can read
	if _, err := svc.GetLink(ctx, bob, l.Code); err != nil {
		t.Fatalf("member get team link: %v", err)
	}
	if _, err := svc.GetLink(ctx, carol, l.Code); err != nil {
		t.Fatalf("admin get team link: %v", err)
	}
	// a non-member gets ErrNotFound
	if _, err := svc.GetLink(ctx, &domain.User{ID: 99}, l.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider get err = %v, want ErrNotFound", err)
	}

	// bob is a member but neither owner nor creator: read-only
	if _, err := svc.UpdateLink(ctx, bob, l.Code, UpdateLinkInput{Destination: "https://x.example.com"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member update err = %v, want ErrForbidden", err)
	}
	// alice the owner can manage
	if _, err := svc.UpdateLink(ctx, alice, l.Code, UpdateLinkInput{Destination: "https://new.example.com"}); err != nil {
		t.Fatalf("owner update: %v", err)
	}

	// team summaries and membership
	if !svc.TeamMember(ctx, bob, team.ID) {
		t.Error("bob should be a member")
	}
	if svc.TeamMember(ctx, &domain.User{ID: 99}, team.ID) {
		t.Error("outsider should not be a member")
	}
	if _, err := svc.GetTeam(ctx, bob, team.ID); err != nil {
		t.Fatalf("member get team: %v", err)
	}
	if _, err := svc.GetTeam(ctx, &domain.User{ID: 99}, team.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider get team err = %v, want ErrNotFound", err)
	}
	summaries, err := svc.ListTeamSummaries(ctx, bob)
	if err != nil || len(summaries) != 1 {
		t.Fatalf("bob team summaries = %v, err %v", summaries, err)
	}
}

func TestAnalyticsRequiresReadAccess(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}
	bob := &domain.User{ID: 2, Username: "bob"}

	l, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	from, to := svc.AnalyticsWindow("", "", time.Now().UTC())
	if _, err := svc.GetAnalytics(ctx, alice, l.Code, from, to); err != nil {
		t.Fatalf("alice analytics: %v", err)
	}
	if _, err := svc.GetAnalytics(ctx, bob, l.Code, from, to); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob analytics err = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetBreakdowns(ctx, alice, l.Code, "bogus", from, to, 10); err == nil {
		t.Fatal("bad dimension should be a validation error")
	}
}

func TestAggregateStatsScope(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}
	bob := &domain.User{ID: 2, Username: "bob"}

	// alice owns two personal links; bob owns one.
	if _, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateLink(ctx, nil, bob.ID, CreateLinkInput{Destination: "https://example.com/c"}); err != nil {
		t.Fatal(err)
	}

	from, to := svc.StatsWindow("", "", time.Now().UTC())
	st, err := svc.GetStats(ctx, alice.ID, from, to)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.TotalLinks != 2 {
		t.Fatalf("total links = %d, want alice's 2", st.TotalLinks)
	}
	if st.TotalVisits != 0 || st.WindowVisits != 0 || st.WindowUniques != 0 {
		t.Fatalf("expected zero analytics, got %+v", st)
	}
	if len(st.Timeseries) != 0 {
		t.Fatalf("timeseries = %+v, want empty", st.Timeseries)
	}

	// Team stats enforce read access.
	team := &domain.Team{Name: "growth", CreatedBy: alice.ID}
	if err := svc.teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	if err := svc.teams.AddMember(ctx, team.ID, alice.ID, domain.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateLink(ctx, &team.ID, alice.ID, CreateLinkInput{Destination: "https://example.com/t"}); err != nil {
		t.Fatal(err)
	}
	tst, err := svc.GetTeamStats(ctx, alice, team.ID, from, to)
	if err != nil {
		t.Fatalf("team stats: %v", err)
	}
	if tst.TotalLinks != 1 {
		t.Fatalf("team total links = %d, want 1", tst.TotalLinks)
	}
	// an outsider cannot see team stats (team existence not leaked)
	if _, err := svc.GetTeamStats(ctx, bob, team.ID, from, to); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider team stats err = %v, want ErrNotFound", err)
	}
}
