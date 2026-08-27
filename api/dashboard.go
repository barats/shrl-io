package main

import (
	"net/http"
	"time"
)

// getDashboard returns the aggregate dashboard model across the caller's
// Personal links for a window (all five rows react to the same from/to).
func (s *server) getDashboard(w http.ResponseWriter, r *http.Request) {
	from, to := s.linkSvc.StatsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	d, err := s.linkSvc.GetDashboard(r.Context(), currentUser(r).ID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, d)
}
