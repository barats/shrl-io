// Package service holds the Link operations shared by the Internal and Auth
// APIs, so both binaries write the redirect cache identically (ADR 0016).
package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

// Errors returned by service operations; handlers map them to HTTP statuses.
var (
	// ErrNotFound means the target link does not exist or the caller lacks
	// read access. Used instead of revealing a link's existence.
	ErrNotFound = errors.New("not found")
	// ErrForbidden means the caller can read the link but cannot manage it.
	ErrForbidden = errors.New("forbidden")
)

// ValidationError is a request-validation failure whose message is safe to
// return to the caller.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// LinkService performs Link operations shared by the Internal and Auth APIs.
type LinkService struct {
	links           *store.LinkStore
	analytics       *store.AnalyticsStore
	hostnames       *store.HostnameStore
	teams           *store.TeamStore
	settings        *store.SettingStore
	linkCache       *cache.LinkCache
	defaultHostname string
	retentionDays   int
}

func NewLinkService(
	links *store.LinkStore,
	analytics *store.AnalyticsStore,
	hostnames *store.HostnameStore,
	teams *store.TeamStore,
	settings *store.SettingStore,
	linkCache *cache.LinkCache,
	defaultHostname string,
	retentionDays int,
) *LinkService {
	return &LinkService{
		links: links, analytics: analytics, hostnames: hostnames,
		teams: teams, settings: settings, linkCache: linkCache,
		defaultHostname: defaultHostname, retentionDays: retentionDays,
	}
}

// CanReadLink reports whether the user may see a link. Personal links are
// visible only to their creator; Team links to any member and to admins (as
// instance oversight); a creator who left the team is an outsider.
func (s *LinkService) CanReadLink(ctx context.Context, u *domain.User, l *domain.Link) bool {
	if u == nil {
		return false
	}
	if l.TeamID == nil {
		return l.CreatedBy == u.ID
	}
	if u.IsAdmin {
		return true
	}
	_, err := s.teams.MemberRole(ctx, *l.TeamID, u.ID)
	return err == nil
}

// CanManageLink reports whether the user may edit, disable, or delete a link:
// its creator (while a member of its team), or a Team Owner of its team.
func (s *LinkService) CanManageLink(ctx context.Context, u *domain.User, l *domain.Link) bool {
	if u == nil {
		return false
	}
	if l.TeamID == nil {
		return l.CreatedBy == u.ID
	}
	role, err := s.teams.MemberRole(ctx, *l.TeamID, u.ID)
	if err != nil {
		return false
	}
	return role == domain.RoleOwner || l.CreatedBy == u.ID
}

// CreateLinkInput carries a validated-in-service create request.
type CreateLinkInput struct {
	Hostname    string
	Destination string
	Remark      string
	ForwardUTM  bool
}

// CreateLink validates and persists a Link in Personal (teamID nil) or Team
// scope, retrying code generation on collision and writing the redirect cache
// on success.
func (s *LinkService) CreateLink(ctx context.Context, teamID *int64, creatorID int64, in CreateLinkInput) (*domain.Link, error) {
	hostname := in.Hostname
	if hostname == "" {
		hostname = s.defaultHostname
	}
	hostname, err := domain.NormalizeAndValidateHostname(hostname)
	if err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}
	if _, err := s.hostnames.Get(ctx, hostname); err != nil {
		return nil, &ValidationError{Msg: "hostname is not registered"}
	}
	dest, err := domain.NormalizeAndValidateDestination(in.Destination)
	if err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}
	remark, err := domain.NormalizeRemark(in.Remark)
	if err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}
	codeLength, err := s.settings.CodeLength(ctx)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		code, err := domain.GenerateCode(codeLength)
		if err != nil {
			return nil, err
		}
		l := &domain.Link{Hostname: hostname, Code: code, Destination: dest, Remark: remark, ForwardUTM: in.ForwardUTM, CreatedBy: creatorID, TeamID: teamID}
		if err := s.links.Create(ctx, l); err == nil {
			if err := s.linkCache.Put(ctx, l); err != nil {
				log.Printf("cache put: %v", err)
			}
			return l, nil
		} else if errors.Is(err, store.ErrDuplicatedKey) {
			continue // auto codes never reuse an existing code
		} else {
			return nil, err
		}
	}
	return nil, errors.New("could not allocate a unique code")
}

// GetLink loads a link the user may read, or ErrNotFound.
func (s *LinkService) GetLink(ctx context.Context, u *domain.User, hostname, code string) (*domain.Link, error) {
	l, err := s.links.Get(ctx, hostname, code)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !s.CanReadLink(ctx, u, l) {
		return nil, ErrNotFound
	}
	return l, nil
}

// ManageableLink loads a link the user may manage. A user who can read but
// not manage gets ErrForbidden; a user with no access gets ErrNotFound so
// link existence is not leaked.
func (s *LinkService) ManageableLink(ctx context.Context, u *domain.User, hostname, code string) (*domain.Link, error) {
	l, err := s.links.Get(ctx, hostname, code)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if s.CanManageLink(ctx, u, l) {
		return l, nil
	}
	if s.CanReadLink(ctx, u, l) {
		return nil, ErrForbidden
	}
	return nil, ErrNotFound
}

// UpdateLinkInput carries an update request; a nil ForwardUTM keeps the
// current value.
type UpdateLinkInput struct {
	Destination string
	Remark      string
	ForwardUTM  *bool
}

// UpdateLink changes a link's destination, remark, and optional forward_utm,
// then refreshes the redirect cache.
func (s *LinkService) UpdateLink(ctx context.Context, u *domain.User, hostname, code string, in UpdateLinkInput) (*domain.Link, error) {
	dest, err := domain.NormalizeAndValidateDestination(in.Destination)
	if err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}
	remark, err := domain.NormalizeRemark(in.Remark)
	if err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}
	l, err := s.ManageableLink(ctx, u, hostname, code)
	if err != nil {
		return nil, err
	}
	l.Destination = dest
	l.Remark = remark
	if in.ForwardUTM != nil {
		l.ForwardUTM = *in.ForwardUTM
	}
	if err := s.links.Save(ctx, l); err != nil {
		return nil, err
	}
	if err := s.linkCache.Put(ctx, l); err != nil {
		log.Printf("cache put: %v", err)
	}
	return l, nil
}

// SetDisabled disables or enables a link, evicting or populating the redirect
// cache so the redirector 404s or serves it.
func (s *LinkService) SetDisabled(ctx context.Context, u *domain.User, hostname, code string, disabled bool) (*domain.Link, error) {
	l, err := s.ManageableLink(ctx, u, hostname, code)
	if err != nil {
		return nil, err
	}
	l.Disabled = disabled
	if err := s.links.Save(ctx, l); err != nil {
		return nil, err
	}
	if err := s.linkCache.Put(ctx, l); err != nil {
		log.Printf("cache put: %v", err)
	}
	return l, nil
}

// DeleteLink permanently removes a link and evicts it from the redirect
// cache. Internal API only; the Auth API never exposes deletion.
func (s *LinkService) DeleteLink(ctx context.Context, u *domain.User, hostname, code string) error {
	if _, err := s.ManageableLink(ctx, u, hostname, code); err != nil {
		return err
	}
	if err := s.links.Delete(ctx, hostname, code); err != nil {
		return err
	}
	return s.linkCache.Delete(ctx, hostname, code)
}

// ListLinks returns the user's Personal links on a hostname, newest first.
func (s *LinkService) ListLinks(ctx context.Context, hostname string, userID int64) ([]domain.Link, error) {
	return s.links.List(ctx, hostname, userID)
}

// ListTeamLinks returns a team's links on a hostname, newest first.
func (s *LinkService) ListTeamLinks(ctx context.Context, hostname string, teamID int64) ([]domain.Link, error) {
	return s.links.ListByTeam(ctx, hostname, teamID)
}

// ListHostnames returns the Hostname Registry for the create-link select.
func (s *LinkService) ListHostnames(ctx context.Context) ([]string, error) {
	hostnames, err := s.hostnames.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(hostnames))
	for _, h := range hostnames {
		names = append(names, h.Name)
	}
	return names, nil
}

// TeamSummary is a team with the caller's role.
type TeamSummary struct {
	Team domain.Team
	Role domain.TeamRole
}

// ListTeamSummaries returns the teams the user can see with their role;
// admins additionally see every team with an empty role when not a member.
func (s *LinkService) ListTeamSummaries(ctx context.Context, u *domain.User) ([]TeamSummary, error) {
	var teams []domain.Team
	var err error
	if u.IsAdmin {
		teams, err = s.teams.ListAll(ctx)
	} else {
		teams, err = s.teams.ListForUser(ctx, u.ID)
	}
	if err != nil {
		return nil, err
	}
	items := make([]TeamSummary, 0, len(teams))
	for _, t := range teams {
		role, err := s.teams.MemberRole(ctx, t.ID, u.ID)
		if err != nil {
			role = ""
		}
		items = append(items, TeamSummary{Team: t, Role: role})
	}
	return items, nil
}

// GetTeam returns a team the user may read (member or admin), or ErrNotFound.
func (s *LinkService) GetTeam(ctx context.Context, u *domain.User, teamID int64) (*domain.Team, error) {
	t, err := s.teams.Get(ctx, teamID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if u.IsAdmin {
		return t, nil
	}
	if _, err := s.teams.MemberRole(ctx, teamID, u.ID); err != nil {
		return nil, ErrNotFound
	}
	return t, nil
}

// TeamMember reports whether the user belongs to the team.
func (s *LinkService) TeamMember(ctx context.Context, u *domain.User, teamID int64) bool {
	_, err := s.teams.MemberRole(ctx, teamID, u.ID)
	return err == nil
}

// LinkAnalytics is the read model for a link's analytics summary.
type LinkAnalytics struct {
	Hostname       string
	Code           string
	RetentionDays  int
	LifetimeVisits int64
	WindowVisits   int64
	WindowUniques  int64
}

// GetAnalytics returns a link's lifetime and window totals, after enforcing
// read access.
func (s *LinkService) GetAnalytics(ctx context.Context, u *domain.User, hostname, code string, from, to time.Time) (LinkAnalytics, error) {
	if _, err := s.GetLink(ctx, u, hostname, code); err != nil {
		return LinkAnalytics{}, err
	}
	a := LinkAnalytics{Hostname: hostname, Code: code, RetentionDays: s.retentionDays}
	if lt, err := s.analytics.GetLifetime(ctx, hostname, code); err == nil {
		a.LifetimeVisits = lt.TotalVisits
	}
	visits, uniques, err := s.analytics.SumDailyStats(ctx, hostname, code, from, to)
	if err != nil {
		return LinkAnalytics{}, err
	}
	a.WindowVisits = visits
	a.WindowUniques = uniques
	return a, nil
}

// GetTimeseries returns a link's daily buckets within a window, ascending.
func (s *LinkService) GetTimeseries(ctx context.Context, u *domain.User, hostname, code string, from, to time.Time) ([]domain.DailyStats, error) {
	if _, err := s.GetLink(ctx, u, hostname, code); err != nil {
		return nil, err
	}
	rows, err := s.analytics.GetTimeseries(ctx, hostname, code, from, to)
	if rows == nil {
		rows = []domain.DailyStats{}
	}
	return rows, err
}

// BreakdownItem is one dimension value with its window total.
type BreakdownItem struct {
	Value string
	Total int64
}

// LinkBreakdown is a link's dimension breakdown with the window totals.
type LinkBreakdown struct {
	Dimension string
	Total     int64
	Items     []BreakdownItem
	Other     int64
}

// GetBreakdowns returns a link's top-N dimension values for a window, after
// enforcing read access. limit <= 0 returns every distinct value.
func (s *LinkService) GetBreakdowns(ctx context.Context, u *domain.User, hostname, code, dimension string, from, to time.Time, limit int) (LinkBreakdown, error) {
	if !validDimensions[dimension] {
		return LinkBreakdown{}, &ValidationError{Msg: "dimension must be referrer, device, os, browser, country, region, city, or a utm_* parameter"}
	}
	if _, err := s.GetLink(ctx, u, hostname, code); err != nil {
		return LinkBreakdown{}, err
	}
	totals, err := s.analytics.GetBreakdowns(ctx, hostname, code, dimension, from, to, limit)
	if err != nil {
		return LinkBreakdown{}, err
	}
	visits, _, err := s.analytics.SumDailyStats(ctx, hostname, code, from, to)
	if err != nil {
		return LinkBreakdown{}, err
	}
	var sum int64
	items := make([]BreakdownItem, 0, len(totals))
	for _, t := range totals {
		items = append(items, BreakdownItem{Value: t.Value, Total: t.Total})
		sum += t.Total
	}
	other := visits - sum
	if other < 0 {
		other = 0
	}
	return LinkBreakdown{Dimension: dimension, Total: visits, Items: items, Other: other}, nil
}

var validDimensions = map[string]bool{
	"referrer": true, "device": true, "os": true, "browser": true,
	"country": true, "region": true, "city": true,
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"utm_term": true, "utm_content": true, "utm_id": true,
}

// AnalyticsWindow returns the from/to date range for analytics reads,
// defaulting to the retention window.
func (s *LinkService) AnalyticsWindow(fromParam, toParam string, now time.Time) (time.Time, time.Time) {
	to := parseDayParam(toParam, now)
	from := parseDayParam(fromParam, now.AddDate(0, 0, -s.retentionDays))
	if from.After(to) {
		from, to = to, from
	}
	return from, to
}

func parseDayParam(v string, def time.Time) time.Time {
	if v == "" {
		return def
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return def
	}
	return t.UTC()
}
