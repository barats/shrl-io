package domain

import "time"

// DailyStats is the pre-aggregated per-link, per-day rollup written by the
// worker and read by the analytics API.
type DailyStats struct {
	Code           string    `json:"code" gorm:"primaryKey"`
	Day            time.Time `json:"day" gorm:"primaryKey;type:date"`
	Visits         int64     `json:"visits"`
	UniqueVisitors int64     `json:"unique_visitors"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Breakdown is one dimension value per link per day (referrer, device, os,
// browser, location, utm_*). Count is visit totals; UniqueVisitors is the
// number of distinct visitor hashes seen for this dimension value. Rows are
// pruned with the retention window.
type Breakdown struct {
	Code           string    `json:"code" gorm:"primaryKey"`
	Day            time.Time `json:"day" gorm:"primaryKey;type:date"`
	Dimension      string    `json:"dimension" gorm:"primaryKey"`
	Value          string    `json:"value" gorm:"primaryKey"`
	Count          int64     `json:"count"`
	UniqueVisitors int64     `json:"unique_visitors"`
}

// LifetimeStats is the per-link total that survives pruning, so the "total
// visits" headline stays accurate for the life of a link.
type LifetimeStats struct {
	Code        string    `json:"code" gorm:"primaryKey"`
	TotalVisits int64     `json:"total_visits"`
	UpdatedAt   time.Time `json:"updated_at"`
}
