package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/barats/shrl-io/internal/domain"
)

// DailyIncrement, LifetimeIncrement, and BreakdownIncrement are the deltas a
// worker batch applies atomically in one transaction.
type DailyIncrement struct {
	Hostname string
	Code     string
	Day      time.Time
	Visits   int64
	Uniques  int64
}

type LifetimeIncrement struct {
	Hostname string
	Code     string
	Visits   int64
}

type BreakdownIncrement struct {
	Hostname  string
	Code      string
	Day       time.Time
	Dimension string
	Value     string
	Count     int64
}

// BreakdownTotal is a dimension value summed across a window.
type BreakdownTotal struct {
	Value string
	Total int64
}

// ApplyAnalytics upserts a batch of increments in one transaction. Updates are
// additive, so a batch applied twice double-counts visits — the accepted
// at-least-once window between apply and ack. Unique-visitor increments are
// already deduplicated by the caller.
func (s *Store) ApplyAnalytics(ctx context.Context, dailies []DailyIncrement, lifetimes []LifetimeIncrement, breakdowns []BreakdownIncrement) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, d := range dailies {
			if err := upsertDaily(tx, d); err != nil {
				return err
			}
		}
		for _, l := range lifetimes {
			if err := upsertLifetime(tx, l); err != nil {
				return err
			}
		}
		for _, b := range breakdowns {
			if err := upsertBreakdown(tx, b); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertDaily(tx *gorm.DB, d DailyIncrement) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}, {Name: "code"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			// Table-qualified: an unqualified column here is ambiguous in
			// Postgres ON CONFLICT when the name is also in the INSERT list.
			"visits":          gorm.Expr(`"daily_stats"."visits" + ?`, d.Visits),
			"unique_visitors": gorm.Expr(`"daily_stats"."unique_visitors" + ?`, d.Uniques),
			"updated_at":      time.Now(),
		}),
	}).Create(&domain.DailyStats{
		Hostname: d.Hostname, Code: d.Code, Day: d.Day,
		Visits: d.Visits, UniqueVisitors: d.Uniques,
	}).Error
}

func upsertLifetime(tx *gorm.DB, l LifetimeIncrement) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}, {Name: "code"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"total_visits": gorm.Expr(`"lifetime_stats"."total_visits" + ?`, l.Visits),
			"updated_at":   time.Now(),
		}),
	}).Create(&domain.LifetimeStats{
		Hostname: l.Hostname, Code: l.Code, TotalVisits: l.Visits,
	}).Error
}

func upsertBreakdown(tx *gorm.DB, b BreakdownIncrement) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}, {Name: "code"}, {Name: "day"}, {Name: "dimension"}, {Name: "value"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count": gorm.Expr(`"breakdowns"."count" + ?`, b.Count),
		}),
	}).Create(&domain.Breakdown{
		Hostname: b.Hostname, Code: b.Code, Day: b.Day,
		Dimension: b.Dimension, Value: b.Value, Count: b.Count,
	}).Error
}

// GetLifetime returns the permanent per-link total.
func (s *Store) GetLifetime(ctx context.Context, hostname, code string) (*domain.LifetimeStats, error) {
	var l domain.LifetimeStats
	err := s.db.WithContext(ctx).Where("hostname = ? AND code = ?", hostname, code).First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// SumDailyStats returns visits and unique visitors for a link within a window.
func (s *Store) SumDailyStats(ctx context.Context, hostname, code string, from, to time.Time) (visits, uniques int64, err error) {
	var row struct {
		Visits  int64
		Uniques int64
	}
	err = s.db.WithContext(ctx).Model(&domain.DailyStats{}).
		Select("COALESCE(SUM(visits), 0) AS visits, COALESCE(SUM(unique_visitors), 0) AS uniques").
		Where("hostname = ? AND code = ? AND day >= ? AND day <= ?", hostname, code, from, to).
		Scan(&row).Error
	return row.Visits, row.Uniques, err
}

// GetTimeseries returns daily buckets for a link within a window, ascending.
func (s *Store) GetTimeseries(ctx context.Context, hostname, code string, from, to time.Time) ([]domain.DailyStats, error) {
	var rows []domain.DailyStats
	err := s.db.WithContext(ctx).
		Where("hostname = ? AND code = ? AND day >= ? AND day <= ?", hostname, code, from, to).
		Order("day ASC").Find(&rows).Error
	return rows, err
}

// GetBreakdowns returns the top-N dimension values by count within a window.
func (s *Store) GetBreakdowns(ctx context.Context, hostname, code, dimension string, from, to time.Time, limit int) ([]BreakdownTotal, error) {
	var totals []BreakdownTotal
	err := s.db.WithContext(ctx).Model(&domain.Breakdown{}).
		Select("value, SUM(count) AS total").
		Where("hostname = ? AND code = ? AND dimension = ? AND day >= ? AND day <= ?", hostname, code, dimension, from, to).
		Group("value").Order("total DESC").Limit(limit).Scan(&totals).Error
	return totals, err
}

// PruneAnalytics deletes daily rollups and breakdowns older than the
// retention window. Lifetime totals are never pruned.
func (s *Store) PruneAnalytics(ctx context.Context, before time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("day < ?", before).Delete(&domain.DailyStats{}).Error; err != nil {
			return err
		}
		return tx.Where("day < ?", before).Delete(&domain.Breakdown{}).Error
	})
}
