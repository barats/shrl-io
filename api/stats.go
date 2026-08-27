package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/barats/shrl-io/internal/service"
)

// getStats returns aggregate analytics across the caller's Personal links.
func (s *server) getStats(w http.ResponseWriter, r *http.Request) {
	from, to := s.linkSvc.StatsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	st, err := s.linkSvc.GetStats(r.Context(), currentUser(r).ID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	writeStats(w, st)
}

// getTeamStats returns aggregate analytics across a team's links, read-only
// for members.
func (s *server) getTeamStats(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	from, to := s.linkSvc.StatsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	st, err := s.linkSvc.GetTeamStats(r.Context(), currentUser(r), id, from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeStats(w, st)
}

// getStatsBreakdowns returns one dimension's full breakdown across the
// caller's Personal links; the dashboard "More" dialog's data source.
func (s *server) getStatsBreakdowns(w http.ResponseWriter, r *http.Request) {
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "referrer"
	}
	from, to := s.linkSvc.StatsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100000 {
			limit = n
		}
	}
	b, err := s.linkSvc.GetStatsBreakdowns(r.Context(), currentUser(r).ID, dimension, from, to, limit)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// getTopLinks returns the caller's links ranked within a window; the Top
// Links "More" dialog's data source. metric is visits (default) or visitors.
func (s *server) getTopLinks(w http.ResponseWriter, r *http.Request) {
	from, to := s.linkSvc.StatsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	byVisits := r.URL.Query().Get("metric") != "visitors"
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100000 {
			limit = n
		}
	}
	links, err := s.linkSvc.GetTopLinks(r.Context(), currentUser(r).ID, from, to, byVisits, limit)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func writeStats(w http.ResponseWriter, st service.Stats) {
	writeJSON(w, http.StatusOK, map[string]any{
		"total_links":    st.TotalLinks,
		"total_visits":   st.TotalVisits,
		"window_visits":  st.WindowVisits,
		"window_uniques": st.WindowUniques,
		"timeseries":     st.Timeseries,
	})
}
