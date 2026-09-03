package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/barats/shrl-io/internal/domain"
)

// AnalyticsStore is the Postgres model for pre-aggregated analytics via GORM.
type AnalyticsStore struct {
	db *gorm.DB
}

func NewAnalyticsStore(db *gorm.DB) *AnalyticsStore { return &AnalyticsStore{db: db} }

func (s *AnalyticsStore) Migrate(ctx context.Context) error {
	// ADR 0019: rollups are keyed by Code alone; the legacy (Hostname, Code)
	// keys cannot be altered in place by GORM, and existing data was cleared
	// by decision, so tables still carrying the legacy hostname key are
	// rebuilt. Only the legacy key triggers a rebuild: the current rollups
	// key by Code (+ Day / dimension / value), which are legitimate composite
	// keys and must survive restarts.
	for _, dst := range []any{&domain.DailyStats{}, &domain.Breakdown{}, &domain.LifetimeStats{}} {
		if s.db.WithContext(ctx).Migrator().HasTable(dst) && hasLegacyAnalyticsKey(s.db, dst) {
			if err := s.db.WithContext(ctx).Migrator().DropTable(dst); err != nil {
				return err
			}
		}
	}
	return s.db.WithContext(ctx).AutoMigrate(
		&domain.DailyStats{},
		&domain.Breakdown{},
		&domain.LifetimeStats{},
	)
}

// hasLegacyAnalyticsKey reports whether an analytics table still uses the
// legacy (Hostname, Code, ...) primary key, i.e. a hostname column that is
// part of the primary key. The current composite keys (Code+Day,
// Code+Day+dimension+value) have no hostname and are not legacy.
func hasLegacyAnalyticsKey(db *gorm.DB, dst any) bool {
	cols, err := db.Migrator().ColumnTypes(dst)
	if err != nil {
		return false
	}
	for _, c := range cols {
		if c.Name() != "hostname" {
			continue
		}
		if pk, ok := c.PrimaryKey(); ok && pk {
			return true
		}
	}
	return false
}

// DailyIncrement, LifetimeIncrement, and BreakdownIncrement are the deltas a
// worker batch applies atomically in one transaction.
type DailyIncrement struct {
	Code    string
	Day     time.Time
	Visits  int64
	Uniques int64
}

type LifetimeIncrement struct {
	Code   string
	Visits int64
}

type BreakdownIncrement struct {
	Code      string
	Day       time.Time
	Dimension string
	Value     string
	Count     int64
	Uniques   int64
}

// BreakdownTotal is a dimension value summed across a window.
type BreakdownTotal struct {
	Value string
	Total int64
}

// BreakdownValues is a dimension value's window totals across a set of codes.
type BreakdownValues struct {
	Value   string
	Visits  int64
	Uniques int64
}

// CodeTotals is one link's window totals, used for ranking links.
type CodeTotals struct {
	Code    string
	Visits  int64
	Uniques int64
}

// ApplyAnalytics upserts a batch of increments in one transaction. Updates are
// additive, so a batch applied twice double-counts visits — the accepted
// at-least-once window between apply and ack. Unique-visitor increments are
// already deduplicated by the caller.
func (s *AnalyticsStore) ApplyAnalytics(ctx context.Context, dailies []DailyIncrement, lifetimes []LifetimeIncrement, breakdowns []BreakdownIncrement) error {
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
		Columns: []clause.Column{{Name: "code"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			// Table-qualified: an unqualified column here is ambiguous in
			// Postgres ON CONFLICT when the name is also in the INSERT list.
			"visits":          gorm.Expr(`"daily_stats"."visits" + ?`, d.Visits),
			"unique_visitors": gorm.Expr(`"daily_stats"."unique_visitors" + ?`, d.Uniques),
			"updated_at":      time.Now(),
		}),
	}).Create(&domain.DailyStats{
		Code: d.Code, Day: d.Day,
		Visits: d.Visits, UniqueVisitors: d.Uniques,
	}).Error
}

func upsertLifetime(tx *gorm.DB, l LifetimeIncrement) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"total_visits": gorm.Expr(`"lifetime_stats"."total_visits" + ?`, l.Visits),
			"updated_at":   time.Now(),
		}),
	}).Create(&domain.LifetimeStats{
		Code: l.Code, TotalVisits: l.Visits,
	}).Error
}

func upsertBreakdown(tx *gorm.DB, b BreakdownIncrement) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}, {Name: "day"}, {Name: "dimension"}, {Name: "value"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":           gorm.Expr(`"breakdowns"."count" + ?`, b.Count),
			"unique_visitors": gorm.Expr(`"breakdowns"."unique_visitors" + ?`, b.Uniques),
		}),
	}).Create(&domain.Breakdown{
		Code: b.Code, Day: b.Day,
		Dimension: b.Dimension, Value: b.Value, Count: b.Count, UniqueVisitors: b.Uniques,
	}).Error
}

// GetLifetime returns the permanent per-link total.
func (s *AnalyticsStore) GetLifetime(ctx context.Context, code string) (*domain.LifetimeStats, error) {
	var l domain.LifetimeStats
	err := s.db.WithContext(ctx).Where("code = ?", code).First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// SumDailyStats returns visits and unique visitors for a link within a window.
func (s *AnalyticsStore) SumDailyStats(ctx context.Context, code string, from, to time.Time) (visits, uniques int64, err error) {
	var row struct {
		Visits  int64
		Uniques int64
	}
	err = s.db.WithContext(ctx).Model(&domain.DailyStats{}).
		Select("COALESCE(SUM(visits), 0) AS visits, COALESCE(SUM(unique_visitors), 0) AS uniques").
		Where("code = ? AND day >= ? AND day <= ?", code, from, to).
		Scan(&row).Error
	return row.Visits, row.Uniques, err
}

// GetTimeseries returns daily buckets for a link within a window, ascending.
func (s *AnalyticsStore) GetTimeseries(ctx context.Context, code string, from, to time.Time) ([]domain.DailyStats, error) {
	var rows []domain.DailyStats
	err := s.db.WithContext(ctx).
		Where("code = ? AND day >= ? AND day <= ?", code, from, to).
		Order("day ASC").Find(&rows).Error
	return rows, err
}

// GetBreakdowns returns the top-N dimension values by count within a window.
// A limit <= 0 returns every distinct value.
func (s *AnalyticsStore) GetBreakdowns(ctx context.Context, code, dimension string, from, to time.Time, limit int) ([]BreakdownTotal, error) {
	var totals []BreakdownTotal
	q := s.db.WithContext(ctx).Model(&domain.Breakdown{}).
		Select("value, SUM(count) AS total").
		Where("code = ? AND dimension = ? AND day >= ? AND day <= ?", code, dimension, from, to).
		Group("value").Order("total DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Scan(&totals).Error
	return totals, err
}

// LifetimeTotals returns the all-time visit total per code for a set of
// codes; a code with no recorded visits is absent from the map.
func (s *AnalyticsStore) LifetimeTotals(ctx context.Context, codes []string) (map[string]int64, error) {
	if len(codes) == 0 {
		return map[string]int64{}, nil
	}
	var rows []struct {
		Code  string
		Total int64
	}
	err := s.db.WithContext(ctx).Model(&domain.LifetimeStats{}).
		Select("code, total_visits AS total").
		Where("code IN ?", codes).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Code] = r.Total
	}
	return out, nil
}

// LifetimeTotal returns the summed all-time visit total for a set of codes.
func (s *AnalyticsStore) LifetimeTotal(ctx context.Context, codes []string) (int64, error) {
	if len(codes) == 0 {
		return 0, nil
	}
	var row struct{ Total int64 }
	err := s.db.WithContext(ctx).Model(&domain.LifetimeStats{}).
		Select("COALESCE(SUM(total_visits), 0) AS total").
		Where("code IN ?", codes).
		Scan(&row).Error
	return row.Total, err
}

// SumDailyStatsForCodes returns total visits and unique visitors within a
// window, summed across the given codes.
func (s *AnalyticsStore) SumDailyStatsForCodes(ctx context.Context, codes []string, from, to time.Time) (visits, uniques int64, err error) {
	if len(codes) == 0 {
		return 0, 0, nil
	}
	var row struct {
		Visits  int64
		Uniques int64
	}
	err = s.db.WithContext(ctx).Model(&domain.DailyStats{}).
		Select("COALESCE(SUM(visits), 0) AS visits, COALESCE(SUM(unique_visitors), 0) AS uniques").
		Where("code IN ? AND day >= ? AND day <= ?", codes, from, to).
		Scan(&row).Error
	return row.Visits, row.Uniques, err
}

// GetTimeseriesForCodes returns per-day totals within a window, ascending,
// summed across the given codes.
func (s *AnalyticsStore) GetTimeseriesForCodes(ctx context.Context, codes []string, from, to time.Time) ([]domain.DailyStats, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var rows []domain.DailyStats
	err := s.db.WithContext(ctx).Model(&domain.DailyStats{}).
		Select("day, SUM(visits) AS visits, SUM(unique_visitors) AS unique_visitors").
		Where("code IN ? AND day >= ? AND day <= ?", codes, from, to).
		Group("day").Order("day ASC").
		Scan(&rows).Error
	return rows, err
}

// SumDailyStatsByCode returns per-link window totals (visits and unique
// visitors) for ranking links within a window.
func (s *AnalyticsStore) SumDailyStatsByCode(ctx context.Context, codes []string, from, to time.Time) ([]CodeTotals, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var rows []CodeTotals
	err := s.db.WithContext(ctx).Model(&domain.DailyStats{}).
		Select("code, SUM(visits) AS visits, SUM(unique_visitors) AS uniques").
		Where("code IN ? AND day >= ? AND day <= ?", codes, from, to).
		Group("code").
		Scan(&rows).Error
	return rows, err
}

// GetBreakdownsForCodes returns the top-N dimension values by unique visitors
// (then visits) within a window, summed across the given codes. A limit <= 0
// returns every distinct value.
func (s *AnalyticsStore) GetBreakdownsForCodes(ctx context.Context, codes []string, dimension string, from, to time.Time, limit int) ([]BreakdownValues, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var rows []BreakdownValues
	q := s.db.WithContext(ctx).Model(&domain.Breakdown{}).
		Select("value, SUM(count) AS visits, SUM(unique_visitors) AS uniques").
		Where("code IN ? AND dimension = ? AND day >= ? AND day <= ?", codes, dimension, from, to).
		Group("value").Order("uniques DESC, visits DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Scan(&rows).Error
	return rows, err
}

// PruneAnalytics deletes daily rollups and breakdowns older than the
// retention window. Lifetime totals are never pruned.
func (s *AnalyticsStore) PruneAnalytics(ctx context.Context, before time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("day < ?", before).Delete(&domain.DailyStats{}).Error; err != nil {
			return err
		}
		return tx.Where("day < ?", before).Delete(&domain.Breakdown{}).Error
	})
}
