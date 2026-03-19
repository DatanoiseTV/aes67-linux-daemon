package api

import (
	"net/http"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/ptp"
)

// handleGetPTPStatus returns the current PTP synchronization status.
func (s *Server) handleGetPTPStatus(w http.ResponseWriter, r *http.Request) {
	status := s.ptp.GetStatus()
	writeJSON(w, status)
}

// handleGetPTPConfig returns the current PTP configuration.
func (s *Server) handleGetPTPConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.ptp.GetConfig()
	writeJSON(w, cfg)
}

// handleSetPTPConfig updates the PTP configuration from the request body.
func (s *Server) handleSetPTPConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ptp.Config
	if err := readJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.ptp.SetConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "set PTP config: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
