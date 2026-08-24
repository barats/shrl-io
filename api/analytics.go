package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/barats/shrl-io/internal/domain"
)

var validDimensions = map[string]bool{
	"referrer": true, "device": true, "os": true, "browser": true,
	"country": true, "region": true, "city": true,
}

// analyticsWindow returns the from/to date range for analytics reads,
// defaulting to the retention window.
func (s *server) analyticsWindow(r *http.Request) (time.Time, time.Time) {
	now := time.Now().UTC()
	to := parseDayParam(r.URL.Query().Get("to"), now)
	from := parseDayParam(r.URL.Query().Get("from"), now.AddDate(0, 0, -s.cfg.retentionDays))
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

func (s *server) getAnalytics(w http.ResponseWriter, r *http.Request) {
	hostname := s.hostname(r)
	code := r.PathValue("code")
	if _, ok := s.accessibleLink(w, r, code); !ok {
		return
	}
	from, to := s.analyticsWindow(r)

	lifetime := int64(0)
	if lt, err := s.analytics.GetLifetime(r.Context(), hostname, code); err == nil {
		lifetime = lt.TotalVisits
	}
	visits, uniques, err := s.analytics.SumDailyStats(r.Context(), hostname, code, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"hostname":    hostname,
		"code":        code,
		"window_days": s.cfg.retentionDays,
		"lifetime":    map[string]int64{"visits": lifetime},
		"window":      map[string]int64{"visits": visits, "unique_visitors": uniques},
	})
}

func (s *server) getAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	hostname := s.hostname(r)
	code := r.PathValue("code")
	if _, ok := s.accessibleLink(w, r, code); !ok {
		return
	}
	from, to := s.analyticsWindow(r)
	rows, err := s.analytics.GetTimeseries(r.Context(), hostname, code, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}
	if rows == nil {
		rows = []domain.DailyStats{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *server) getAnalyticsBreakdowns(w http.ResponseWriter, r *http.Request) {
	hostname := s.hostname(r)
	code := r.PathValue("code")
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "referrer"
	}
	if !validDimensions[dimension] {
		writeError(w, http.StatusBadRequest, "dimension must be referrer, device, os, browser, country, region, or city")
		return
	}
	if _, ok := s.accessibleLink(w, r, code); !ok {
		return
	}
	from, to := s.analyticsWindow(r)

	// limit defaults to 10; 0 returns every distinct value.
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 10000 {
			limit = n
		}
	}

	totals, err := s.analytics.GetBreakdowns(r.Context(), hostname, code, dimension, from, to, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}
	visits, _, err := s.analytics.SumDailyStats(r.Context(), hostname, code, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}

	var sum int64
	items := make([]map[string]int64, 0, len(totals))
	for _, t := range totals {
		items = append(items, map[string]int64{t.Value: t.Total})
		sum += t.Total
	}
	other := visits - sum
	if other < 0 {
		other = 0
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dimension": dimension,
		"total":     visits,
		"items":     items,
		"other":     other,
	})
}
