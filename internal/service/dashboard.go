package service

import (
	"context"
	"sort"
	"time"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

// dashboardBreakdownLimit is the number of top values returned per dimension
// and the number of top links in each ranking.
const dashboardBreakdownLimit = 10

// DashboardTopLink is one link's window totals for the dashboard ranking.
type DashboardTopLink struct {
	Code           string `json:"code"`
	BaseURL        string `json:"base_url"`
	Visits         int64  `json:"visits"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

// DashboardBreakdownItem is one dimension value's window totals.
type DashboardBreakdownItem struct {
	Value          string `json:"value"`
	Visits         int64  `json:"visits"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

// Dashboard is the read model for the personal analytics dashboard: all rows
// react to the same from/to window. LifetimeVisits is the all-time total, so
// the "all time" preset can show a number that outlives the retention window.
type Dashboard struct {
	TotalLinks     int64                               `json:"total_links"`
	ActiveLinks    int64                               `json:"active_links"`
	DisabledLinks  int64                               `json:"disabled_links"`
	LifetimeVisits int64                               `json:"lifetime_visits"`
	WindowVisits   int64                               `json:"window_visits"`
	WindowUniques  int64                               `json:"window_uniques"`
	Timeseries     []domain.DailyStats                 `json:"timeseries"`
	TopByVisits    []DashboardTopLink                  `json:"top_by_visits"`
	TopByVisitors  []DashboardTopLink                  `json:"top_by_visitors"`
	Sources        []DashboardBreakdownItem            `json:"sources"`
	Environment    map[string][]DashboardBreakdownItem `json:"environment"`
	Location       map[string][]DashboardBreakdownItem `json:"location"`
}

// GetDashboard returns the full dashboard model across a user's Personal
// links for a window: cards, timeseries, per-link rankings, and the
// environment/location breakdowns ordered by unique visitors.
func (s *LinkService) GetDashboard(ctx context.Context, userID int64, from, to time.Time) (Dashboard, error) {
	links, err := s.links.List(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	return s.dashboardForLinks(ctx, links, from, to)
}

// GetTeamDashboard returns the full dashboard model across a team's links,
// after enforcing read access (member or admin; outsiders get ErrNotFound so
// team existence is not leaked).
func (s *LinkService) GetTeamDashboard(ctx context.Context, u *domain.User, teamID int64, from, to time.Time) (Dashboard, error) {
	if _, err := s.GetTeam(ctx, u, teamID); err != nil {
		return Dashboard{}, err
	}
	links, err := s.links.ListByTeam(ctx, teamID)
	if err != nil {
		return Dashboard{}, err
	}
	return s.dashboardForLinks(ctx, links, from, to)
}

// dashboardForLinks builds the dashboard model across a fixed set of links.
func (s *LinkService) dashboardForLinks(ctx context.Context, links []domain.Link, from, to time.Time) (Dashboard, error) {
	d := Dashboard{
		TotalLinks:  int64(len(links)),
		Environment: map[string][]DashboardBreakdownItem{},
		Location:    map[string][]DashboardBreakdownItem{},
	}
	baseURLOf := make(map[string]string, len(links))
	codes := make([]string, 0, len(links))
	for _, l := range links {
		if l.Disabled {
			d.DisabledLinks++
		} else {
			d.ActiveLinks++
		}
		baseURLOf[l.Code] = l.BaseURL
		codes = append(codes, l.Code)
	}
	if len(codes) == 0 {
		d.Timeseries = []domain.DailyStats{}
		return d, nil
	}
	if lt, err := s.analytics.LifetimeTotal(ctx, codes); err == nil {
		d.LifetimeVisits = lt
	}
	visits, uniques, err := s.analytics.SumDailyStatsForCodes(ctx, codes, from, to)
	if err != nil {
		return Dashboard{}, err
	}
	d.WindowVisits, d.WindowUniques = visits, uniques
	rows, err := s.analytics.GetTimeseriesForCodes(ctx, codes, from, to)
	if err != nil {
		return Dashboard{}, err
	}
	if rows == nil {
		rows = []domain.DailyStats{}
	}
	d.Timeseries = rows

	totals, err := s.analytics.SumDailyStatsByCode(ctx, codes, from, to)
	if err != nil {
		return Dashboard{}, err
	}
	d.TopByVisits = topLinks(totals, baseURLOf, true, dashboardBreakdownLimit)
	d.TopByVisitors = topLinks(totals, baseURLOf, false, dashboardBreakdownLimit)

	sourceItems, err := s.analytics.GetBreakdownsForCodes(ctx, codes, "referrer", from, to, dashboardBreakdownLimit)
	if err != nil {
		return Dashboard{}, err
	}
	d.Sources = breakdownItems(sourceItems)

	for _, dim := range []string{"device", "os", "browser"} {
		items, err := s.analytics.GetBreakdownsForCodes(ctx, codes, dim, from, to, dashboardBreakdownLimit)
		if err != nil {
			return Dashboard{}, err
		}
		d.Environment[dim] = breakdownItems(items)
	}
	for _, dim := range []string{"country", "region", "city"} {
		items, err := s.analytics.GetBreakdownsForCodes(ctx, codes, dim, from, to, dashboardBreakdownLimit)
		if err != nil {
			return Dashboard{}, err
		}
		d.Location[dim] = breakdownItems(items)
	}
	return d, nil
}

func topLinks(totals []store.CodeTotals, baseURLOf map[string]string, byVisits bool, limit int) []DashboardTopLink {
	sorted := make([]store.CodeTotals, len(totals))
	copy(sorted, totals)
	if byVisits {
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Visits > sorted[j].Visits })
	} else {
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Uniques > sorted[j].Uniques })
	}
	if limit <= 0 || limit > len(sorted) {
		limit = len(sorted)
	}
	out := make([]DashboardTopLink, 0, limit)
	for i := 0; i < limit; i++ {
		t := sorted[i]
		out = append(out, DashboardTopLink{
			Code:           t.Code,
			BaseURL:        baseURLOf[t.Code],
			Visits:         t.Visits,
			UniqueVisitors: t.Uniques,
		})
	}
	return out
}

// DashboardBreakdown is an aggregate dimension breakdown across a user's
// Personal links, the dialog's data source for the "More" action.
type DashboardBreakdown struct {
	Dimension string                   `json:"dimension"`
	Total     int64                    `json:"total"`
	Items     []DashboardBreakdownItem `json:"items"`
	Other     int64                    `json:"other"`
}

// GetStatsBreakdowns returns a dimension's values summed across a user's
// Personal links within a window, ordered by unique visitors then visits. A
// limit <= 0 returns every distinct value.
func (s *LinkService) GetStatsBreakdowns(ctx context.Context, userID int64, dimension string, from, to time.Time, limit int) (DashboardBreakdown, error) {
	if !validDimensions[dimension] {
		return DashboardBreakdown{}, &ValidationError{Msg: "dimension must be referrer, device, os, browser, country, region, city, or a utm_* parameter"}
	}
	links, err := s.links.List(ctx, userID)
	if err != nil {
		return DashboardBreakdown{}, err
	}
	return s.statsBreakdownsForLinks(ctx, links, dimension, from, to, limit)
}

// GetTeamStatsBreakdowns is the team-scoped equivalent of GetStatsBreakdowns,
// after enforcing read access.
func (s *LinkService) GetTeamStatsBreakdowns(ctx context.Context, u *domain.User, teamID int64, dimension string, from, to time.Time, limit int) (DashboardBreakdown, error) {
	if !validDimensions[dimension] {
		return DashboardBreakdown{}, &ValidationError{Msg: "dimension must be referrer, device, os, browser, country, region, city, or a utm_* parameter"}
	}
	if _, err := s.GetTeam(ctx, u, teamID); err != nil {
		return DashboardBreakdown{}, err
	}
	links, err := s.links.ListByTeam(ctx, teamID)
	if err != nil {
		return DashboardBreakdown{}, err
	}
	return s.statsBreakdownsForLinks(ctx, links, dimension, from, to, limit)
}

// statsBreakdownsForLinks builds a dimension breakdown across a fixed set of
// links.
func (s *LinkService) statsBreakdownsForLinks(ctx context.Context, links []domain.Link, dimension string, from, to time.Time, limit int) (DashboardBreakdown, error) {
	codes := make([]string, 0, len(links))
	for _, l := range links {
		codes = append(codes, l.Code)
	}
	if len(codes) == 0 {
		return DashboardBreakdown{Dimension: dimension, Items: []DashboardBreakdownItem{}}, nil
	}
	visits, _, err := s.analytics.SumDailyStatsForCodes(ctx, codes, from, to)
	if err != nil {
		return DashboardBreakdown{}, err
	}
	rows, err := s.analytics.GetBreakdownsForCodes(ctx, codes, dimension, from, to, limit)
	if err != nil {
		return DashboardBreakdown{}, err
	}
	items := breakdownItems(rows)
	var sum int64
	for _, it := range items {
		sum += it.Visits
	}
	other := visits - sum
	if other < 0 {
		other = 0
	}
	return DashboardBreakdown{Dimension: dimension, Total: visits, Items: items, Other: other}, nil
}

// GetTopLinks returns a user's links ranked within a window, the dialog's
// data source for the Top Links "More" action. A limit <= 0 returns every
// link with activity in the window.
func (s *LinkService) GetTopLinks(ctx context.Context, userID int64, from, to time.Time, byVisits bool, limit int) ([]DashboardTopLink, error) {
	links, err := s.links.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.topLinksForLinks(ctx, links, from, to, byVisits, limit)
}

// GetTeamTopLinks is the team-scoped equivalent of GetTopLinks, after
// enforcing read access.
func (s *LinkService) GetTeamTopLinks(ctx context.Context, u *domain.User, teamID int64, from, to time.Time, byVisits bool, limit int) ([]DashboardTopLink, error) {
	if _, err := s.GetTeam(ctx, u, teamID); err != nil {
		return nil, err
	}
	links, err := s.links.ListByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return s.topLinksForLinks(ctx, links, from, to, byVisits, limit)
}

// topLinksForLinks ranks a fixed set of links within a window.
func (s *LinkService) topLinksForLinks(ctx context.Context, links []domain.Link, from, to time.Time, byVisits bool, limit int) ([]DashboardTopLink, error) {
	baseURLOf := make(map[string]string, len(links))
	codes := make([]string, 0, len(links))
	for _, l := range links {
		baseURLOf[l.Code] = l.BaseURL
		codes = append(codes, l.Code)
	}
	if len(codes) == 0 {
		return []DashboardTopLink{}, nil
	}
	totals, err := s.analytics.SumDailyStatsByCode(ctx, codes, from, to)
	if err != nil {
		return nil, err
	}
	return topLinks(totals, baseURLOf, byVisits, limit), nil
}

func breakdownItems(items []store.BreakdownValues) []DashboardBreakdownItem {
	out := make([]DashboardBreakdownItem, 0, len(items))
	for _, it := range items {
		out = append(out, DashboardBreakdownItem{Value: it.Value, Visits: it.Visits, UniqueVisitors: it.Uniques})
	}
	return out
}
