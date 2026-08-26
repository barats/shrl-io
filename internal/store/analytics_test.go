package store

import (
	"context"
	"testing"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

// These tests exercise the AnalyticsStore read paths (GetLifetime,
// SumDailyStats, GetTimeseries, GetBreakdowns). Rows are seeded directly
// rather than through ApplyAnalytics: the worker's upsert path uses
// Postgres-flavored ON CONFLICT and stays covered by the integration stack.
func TestAnalyticsStoreReads(t *testing.T) {
	db := newTestDB(t)
	s := NewAnalyticsStore(db)
	ctx := context.Background()

	day1 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)

	seed := []*domain.DailyStats{
		{Code: "abc", Day: day1, Visits: 5, UniqueVisitors: 3},
		{Code: "abc", Day: day2, Visits: 7, UniqueVisitors: 4},
		{Code: "other", Day: day1, Visits: 99, UniqueVisitors: 99},
	}
	for _, d := range seed {
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("seed daily: %v", err)
		}
	}
	if err := db.Create(&domain.LifetimeStats{Code: "abc", TotalVisits: 12}).Error; err != nil {
		t.Fatalf("seed lifetime: %v", err)
	}

	lt, err := s.GetLifetime(ctx, "abc")
	if err != nil || lt.TotalVisits != 12 {
		t.Fatalf("lifetime = %+v, %v", lt, err)
	}

	visits, uniques, err := s.SumDailyStats(ctx, "abc", day1, day2)
	if err != nil || visits != 12 || uniques != 7 {
		t.Fatalf("sum = %d/%d, %v (want 12/7)", visits, uniques, err)
	}

	rows, err := s.GetTimeseries(ctx, "abc", day1, day2)
	if err != nil || len(rows) != 2 {
		t.Fatalf("timeseries = %+v, %v", rows, err)
	}
	if !rows[0].Day.Equal(day1) || !rows[1].Day.Equal(day2) || rows[1].Visits != 7 {
		t.Fatalf("timeseries order/content wrong: %+v", rows)
	}
}

func TestAnalyticsStoreBreakdownsLimit(t *testing.T) {
	db := newTestDB(t)
	s := NewAnalyticsStore(db)
	ctx := context.Background()

	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	// 12 distinct referrers with descending counts, 12..1
	for i := 0; i < 12; i++ {
		value := "ref" + string(rune('a'+i)) + ".example"
		if err := db.Create(&domain.Breakdown{
			Code: "abc", Day: day, Dimension: "referrer",
			Value: value, Count: int64(12 - i),
		}).Error; err != nil {
			t.Fatalf("seed breakdown: %v", err)
		}
	}

	// top 5 by count, descending
	tops, err := s.GetBreakdowns(ctx, "abc", "referrer", day, day, 5)
	if err != nil {
		t.Fatalf("get breakdowns: %v", err)
	}
	if len(tops) != 5 {
		t.Fatalf("top5 len = %d, want 5", len(tops))
	}
	if tops[0].Value != "refa.example" || tops[0].Total != 12 {
		t.Fatalf("top5[0] = %+v, want refa.example/12", tops[0])
	}
	if tops[4].Value != "refe.example" || tops[4].Total != 8 {
		t.Fatalf("top5[4] = %+v, want refe.example/8", tops[4])
	}

	// limit <= 0 returns every distinct value
	all, err := s.GetBreakdowns(ctx, "abc", "referrer", day, day, 0)
	if err != nil {
		t.Fatalf("get breakdowns all: %v", err)
	}
	if len(all) != 12 {
		t.Fatalf("all len = %d, want 12", len(all))
	}

	// scoped to the code: another link's breakdowns don't leak in
	_ = db.Create(&domain.Breakdown{
		Code: "other", Day: day, Dimension: "referrer",
		Value: "other.example", Count: 100,
	}).Error
	scoped, err := s.GetBreakdowns(ctx, "abc", "referrer", day, day, 0)
	if err != nil || len(scoped) != 12 {
		t.Fatalf("scoped len = %d, %v (want 12)", len(scoped), err)
	}
}

func TestAnalyticsStoreAggregatesAcrossCodes(t *testing.T) {
	db := newTestDB(t)
	s := NewAnalyticsStore(db)
	ctx := context.Background()

	day1 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)

	seed := []*domain.DailyStats{
		{Code: "abc", Day: day1, Visits: 5, UniqueVisitors: 3},
		{Code: "abc", Day: day2, Visits: 7, UniqueVisitors: 4},
		{Code: "other", Day: day2, Visits: 2, UniqueVisitors: 2},
	}
	for _, d := range seed {
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("seed daily: %v", err)
		}
	}
	for _, l := range []*domain.LifetimeStats{
		{Code: "abc", TotalVisits: 12},
		{Code: "other", TotalVisits: 99},
	} {
		if err := db.Create(l).Error; err != nil {
			t.Fatalf("seed lifetime: %v", err)
		}
	}

	// window across both codes
	visits, uniques, err := s.SumDailyStatsForCodes(ctx, []string{"abc", "other"}, day1, day2)
	if err != nil || visits != 14 || uniques != 9 {
		t.Fatalf("sum across codes = %d/%d, %v (want 14/9)", visits, uniques, err)
	}

	// all-time total across both codes
	total, err := s.LifetimeTotal(ctx, []string{"abc", "other"})
	if err != nil || total != 111 {
		t.Fatalf("lifetime total = %d, %v (want 111)", total, err)
	}

	// timeseries grouped by day across both codes, ascending
	rows, err := s.GetTimeseriesForCodes(ctx, []string{"abc", "other"}, day1, day2)
	if err != nil || len(rows) != 2 {
		t.Fatalf("timeseries = %+v, %v", rows, err)
	}
	if !rows[0].Day.Equal(day1) || rows[0].Visits != 5 || rows[0].UniqueVisitors != 3 {
		t.Fatalf("day1 row wrong: %+v", rows[0])
	}
	if !rows[1].Day.Equal(day2) || rows[1].Visits != 9 || rows[1].UniqueVisitors != 6 {
		t.Fatalf("day2 row wrong: %+v", rows[1])
	}

	// an empty code set returns zeros without querying
	v, u, err := s.SumDailyStatsForCodes(ctx, nil, day1, day2)
	if err != nil || v != 0 || u != 0 {
		t.Fatalf("empty codes sum = %d/%d, %v", v, u, err)
	}
}
