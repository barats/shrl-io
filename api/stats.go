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

func writeStats(w http.ResponseWriter, st service.Stats) {
	writeJSON(w, http.StatusOK, map[string]any{
		"total_links":    st.TotalLinks,
		"total_visits":   st.TotalVisits,
		"window_visits":  st.WindowVisits,
		"window_uniques": st.WindowUniques,
		"timeseries":     st.Timeseries,
	})
}
