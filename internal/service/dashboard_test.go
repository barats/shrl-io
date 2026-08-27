package service

import (
	"context"
	"testing"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

func TestGetDashboard(t *testing.T) {
	svc, db, _ := newTestServiceFull(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}

	la, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/a"})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	lb, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/b"})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	// bob's link must not leak into alice's dashboard
	if _, err := svc.CreateLink(ctx, nil, 2, CreateLinkInput{Destination: "https://example.com/c"}); err != nil {
		t.Fatalf("create c: %v", err)
	}
	if _, err := svc.SetDisabled(ctx, alice, lb.Code, true); err != nil {
		t.Fatalf("disable b: %v", err)
	}

	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	seed := []*domain.DailyStats{
		{Code: la.Code, Day: day, Visits: 10, UniqueVisitors: 6},
		{Code: lb.Code, Day: day, Visits: 2, UniqueVisitors: 2},
	}
	for _, d := range seed {
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("seed daily: %v", err)
		}
	}
	if err := db.Create(&domain.LifetimeStats{Code: la.Code, TotalVisits: 50}).Error; err != nil {
		t.Fatalf("seed lifetime: %v", err)
	}
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "browser", Value: "Chrome", Count: 8, UniqueVisitors: 5})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "browser", Value: "Safari", Count: 2, UniqueVisitors: 1})
	_ = db.Create(&domain.Breakdown{Code: lb.Code, Day: day, Dimension: "browser", Value: "Chrome", Count: 2, UniqueVisitors: 2})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "country", Value: "US", Count: 10, UniqueVisitors: 6})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "referrer", Value: "google.com", Count: 6, UniqueVisitors: 4})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "referrer", Value: "direct", Count: 4, UniqueVisitors: 2})

	from := day.AddDate(0, 0, -1)
	to := day.AddDate(0, 0, 1)
	d, err := svc.GetDashboard(ctx, alice.ID, from, to)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}

	if d.TotalLinks != 2 || d.ActiveLinks != 1 || d.DisabledLinks != 1 {
		t.Errorf("cards = total %d active %d disabled %d, want 2/1/1", d.TotalLinks, d.ActiveLinks, d.DisabledLinks)
	}
	if d.LifetimeVisits != 50 {
		t.Errorf("lifetime visits = %d, want 50", d.LifetimeVisits)
	}
	if d.WindowVisits != 12 || d.WindowUniques != 8 {
		t.Errorf("window = %d visits / %d uniques, want 12/8", d.WindowVisits, d.WindowUniques)
	}
	if len(d.Timeseries) != 1 || !d.Timeseries[0].Day.Equal(day) || d.Timeseries[0].Visits != 12 || d.Timeseries[0].UniqueVisitors != 8 {
		t.Errorf("timeseries = %+v, want one day 12/8", d.Timeseries)
	}

	if len(d.TopByVisits) != 2 || d.TopByVisits[0].Code != la.Code || d.TopByVisits[0].Visits != 10 {
		t.Errorf("top by visits = %+v, want a first with 10", d.TopByVisits)
	}
	if len(d.TopByVisitors) != 2 || d.TopByVisitors[0].Code != la.Code || d.TopByVisitors[0].UniqueVisitors != 6 {
		t.Errorf("top by visitors = %+v, want a first with 6", d.TopByVisitors)
	}
	if d.TopByVisits[0].Hostname != "shrl.io" {
		t.Errorf("top link hostname = %q, want shrl.io", d.TopByVisits[0].Hostname)
	}

	// environment browser breakdown ordered by unique visitors desc, summed
	// across both of alice's links.
	browser := d.Environment["browser"]
	if len(browser) != 2 || browser[0].Value != "Chrome" || browser[0].Visits != 10 || browser[0].UniqueVisitors != 7 {
		t.Errorf("browser = %+v, want Chrome 10/7 first", browser)
	}
	if browser[1].Value != "Safari" || browser[1].UniqueVisitors != 1 {
		t.Errorf("browser[1] = %+v, want Safari uniques 1", browser[1])
	}
	country := d.Location["country"]
	if len(country) != 1 || country[0].Value != "US" || country[0].Visits != 10 || country[0].UniqueVisitors != 6 {
		t.Errorf("country = %+v, want US 10/6", country)
	}
	// sources: referrers ordered by unique visitors desc, direct included.
	if len(d.Sources) != 2 || d.Sources[0].Value != "google.com" || d.Sources[0].Visits != 6 || d.Sources[0].UniqueVisitors != 4 {
		t.Errorf("sources = %+v, want google.com 6/4 first", d.Sources)
	}
	if d.Sources[1].Value != "direct" || d.Sources[1].UniqueVisitors != 2 {
		t.Errorf("sources[1] = %+v, want direct uniques 2", d.Sources[1])
	}
}

func TestGetStatsBreakdowns(t *testing.T) {
	svc, db, _ := newTestServiceFull(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}

	la, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	// bob's breakdowns must not leak into alice's aggregate.
	if _, err := svc.CreateLink(ctx, nil, 2, CreateLinkInput{Destination: "https://example.com/c"}); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	_ = db.Create(&domain.DailyStats{Code: la.Code, Day: day, Visits: 8, UniqueVisitors: 6})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "referrer", Value: "google.com", Count: 5, UniqueVisitors: 3})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "referrer", Value: "twitter.com", Count: 2, UniqueVisitors: 2})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "referrer", Value: "direct", Count: 1, UniqueVisitors: 1})
	_ = db.Create(&domain.Breakdown{Code: la.Code, Day: day, Dimension: "country", Value: "US", Count: 8, UniqueVisitors: 5})

	from := day.AddDate(0, 0, -1)
	to := day.AddDate(0, 0, 1)

	// limit 0 returns every distinct value, ordered by uniques desc.
	all, err := svc.GetStatsBreakdowns(ctx, alice.ID, "referrer", from, to, 0)
	if err != nil {
		t.Fatalf("breakdowns: %v", err)
	}
	if len(all.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(all.Items))
	}
	if all.Items[0].Value != "google.com" || all.Items[0].UniqueVisitors != 3 {
		t.Errorf("items[0] = %+v, want google.com uniques 3", all.Items[0])
	}
	if all.Total != 8 || all.Other != 0 {
		t.Errorf("total = %d other = %d, want 8/0", all.Total, all.Other)
	}

	// limit 1 trims and reports the rest as "other".
	top, err := svc.GetStatsBreakdowns(ctx, alice.ID, "referrer", from, to, 1)
	if err != nil || len(top.Items) != 1 {
		t.Fatalf("top = %+v, %v", top.Items, err)
	}
	if top.Items[0].Value != "google.com" || top.Other != 3 {
		t.Errorf("top[0] = %+v other = %d, want google.com other 3", top.Items[0], top.Other)
	}

	// invalid dimensions are rejected.
	if _, err := svc.GetStatsBreakdowns(ctx, alice.ID, "bogus", from, to, 0); err == nil {
		t.Error("bad dimension should be a validation error")
	}
}

func TestGetTopLinks(t *testing.T) {
	svc, db, _ := newTestServiceFull(t)
	ctx := context.Background()
	alice := &domain.User{ID: 1, Username: "alice"}

	la, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	lb, err := svc.CreateLink(ctx, nil, alice.ID, CreateLinkInput{Destination: "https://example.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	_ = db.Create(&domain.DailyStats{Code: la.Code, Day: day, Visits: 10, UniqueVisitors: 6})
	_ = db.Create(&domain.DailyStats{Code: lb.Code, Day: day, Visits: 20, UniqueVisitors: 2})

	from := day.AddDate(0, 0, -1)
	to := day.AddDate(0, 0, 1)

	byVisits, err := svc.GetTopLinks(ctx, alice.ID, from, to, true, 0)
	if err != nil {
		t.Fatalf("top links by visits: %v", err)
	}
	if len(byVisits) != 2 || byVisits[0].Code != lb.Code || byVisits[0].Visits != 20 {
		t.Errorf("by visits = %+v, want b first with 20", byVisits)
	}
	byVisitors, err := svc.GetTopLinks(ctx, alice.ID, from, to, false, 0)
	if err != nil {
		t.Fatalf("top links by visitors: %v", err)
	}
	if len(byVisitors) != 2 || byVisitors[0].Code != la.Code || byVisitors[0].UniqueVisitors != 6 {
		t.Errorf("by visitors = %+v, want a first with 6", byVisitors)
	}
	if byVisitors[0].Hostname != "shrl.io" {
		t.Errorf("hostname = %q, want shrl.io", byVisitors[0].Hostname)
	}
}

func TestGetDashboardEmptyUser(t *testing.T) {
	svc, _, _ := newTestServiceFull(t)
	ctx := context.Background()
	from, to := svc.StatsWindow("", "", time.Now().UTC())
	d, err := svc.GetDashboard(ctx, 99, from, to)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if d.TotalLinks != 0 || d.ActiveLinks != 0 || d.DisabledLinks != 0 || d.WindowVisits != 0 || d.LifetimeVisits != 0 {
		t.Errorf("expected zero dashboard, got %+v", d)
	}
	if len(d.Timeseries) != 0 {
		t.Errorf("timeseries = %+v, want empty", d.Timeseries)
	}
}
