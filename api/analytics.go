package main

import (
	"net/http"
	"strconv"
	"time"
)

func (s *server) getAnalytics(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	from, to := s.linkSvc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	a, err := s.linkSvc.GetAnalytics(r.Context(), currentUser(r), code, from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":        a.Code,
		"window_days": a.RetentionDays,
		"lifetime":    map[string]int64{"visits": a.LifetimeVisits},
		"window":      map[string]int64{"visits": a.WindowVisits, "unique_visitors": a.WindowUniques},
	})
}

func (s *server) getAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	from, to := s.linkSvc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	rows, err := s.linkSvc.GetTimeseries(r.Context(), currentUser(r), code, from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *server) getAnalyticsBreakdowns(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "referrer"
	}
	from, to := s.linkSvc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())

	// limit defaults to 10; 0 returns every distinct value.
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 10000 {
			limit = n
		}
	}

	b, err := s.linkSvc.GetBreakdowns(r.Context(), currentUser(r), code, dimension, from, to, limit)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	items := make([]map[string]int64, 0, len(b.Items))
	for _, item := range b.Items {
		items = append(items, map[string]int64{item.Value: item.Total})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dimension": b.Dimension,
		"total":     b.Total,
		"items":     items,
		"other":     b.Other,
	})
}
