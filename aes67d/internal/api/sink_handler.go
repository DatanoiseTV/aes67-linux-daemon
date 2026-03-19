package api

import (
	"fmt"
	"net/http"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/session"
)

// handleGetSinks returns all active sinks wrapped in a JSON object.
func (s *Server) handleGetSinks(w http.ResponseWriter, r *http.Request) {
	sinks := s.session.GetSinks()
	if sinks == nil {
		sinks = []session.StreamSink{}
	}
	writeJSON(w, map[string]interface{}{"sinks": sinks})
}

// handleGetSinkStatus returns the runtime status of a sink.
func (s *Server) handleGetSinkStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, err := s.session.GetSinkStatus(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("sink status: %v", err))
		return
	}

	// Return in the C++ daemon format with nested sink_flags.
	writeJSON(w, map[string]interface{}{
		"sink_flags": map[string]bool{
			"is_rtp_seq_id_error":       status.IsSeqIDError,
			"is_rtp_ssrc_error":         status.IsSSRCError,
			"is_rtp_payload_type_error": status.IsPayloadTypeError,
			"is_rtp_sac_error":          status.IsSACError,
			"is_receiving_rtp_packet":   status.IsReceiving,
			"is_muted":                  status.IsMuted,
			"is_some_muted":             status.IsSomeMuted,
			"is_all_muted":              status.IsAllMuted,
		},
		"sink_min_time": status.MinTime,
	})
}

// handlePutSink creates or updates a sink stream.
func (s *Server) handlePutSink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var sink session.StreamSink
	if err := readJSON(r, &sink); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	sink.ID = id

	if err := s.session.AddSink(sink); err != nil {
		writeError(w, http.StatusInternalServerError, "add sink: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleDeleteSink removes a sink stream.
func (s *Server) handleDeleteSink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.session.RemoveSink(id); err != nil {
		writeError(w, http.StatusNotFound, "remove sink: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
