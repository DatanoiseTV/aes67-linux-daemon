package api

import (
	"net/http"
)

// handleGetMatrix returns the current audio routing matrix.
func (s *Server) handleGetMatrix(w http.ResponseWriter, r *http.Request) {
	matrix := s.session.GetMatrix()
	writeJSON(w, matrix)
}

// matrixRouteRequest represents a single matrix route change request.
type matrixRouteRequest struct {
	SrcStream  uint8  `json:"src_stream"`
	SrcChannel uint8  `json:"src_channel"`
	DstStream  uint8  `json:"dst_stream"`
	DstChannel uint8  `json:"dst_channel"`
	Action     string `json:"action"`
}

// handleSetMatrix applies multiple matrix route changes from the request body.
func (s *Server) handleSetMatrix(w http.ResponseWriter, r *http.Request) {
	var routes []matrixRouteRequest
	if err := readJSON(r, &routes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	for _, route := range routes {
		if err := s.session.SetMatrixRoute(route.SrcStream, route.SrcChannel, route.DstStream, route.DstChannel, route.Action); err != nil {
			writeError(w, http.StatusInternalServerError, "set matrix route: "+err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// handleSetMatrixRoute applies a single matrix route change from the request body.
func (s *Server) handleSetMatrixRoute(w http.ResponseWriter, r *http.Request) {
	var route matrixRouteRequest
	if err := readJSON(r, &route); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.session.SetMatrixRoute(route.SrcStream, route.SrcChannel, route.DstStream, route.DstChannel, route.Action); err != nil {
		writeError(w, http.StatusInternalServerError, "set matrix route: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
