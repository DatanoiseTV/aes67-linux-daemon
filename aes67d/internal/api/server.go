// Package api provides the HTTP REST API server for the AES67 daemon.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/config"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/discovery"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/ptp"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/session"
)

// Server is the HTTP API server that exposes REST endpoints for managing
// AES67 sources, sinks, PTP, and the audio routing matrix.
type Server struct {
	config    *config.Config
	session   *session.Manager
	ptp       *ptp.Monitor
	discovery *discovery.Manager
	mux       *http.ServeMux
	version   string
}

// NewServer creates a new HTTP API server with all routes registered.
func NewServer(cfg *config.Config, sess *session.Manager, ptpMon *ptp.Monitor, disc *discovery.Manager, version string) *Server {
	s := &Server{
		config:    cfg,
		session:   sess,
		ptp:       ptpMon,
		discovery: disc,
		version:   version,
	}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Handler returns the HTTP handler with CORS middleware applied.
func (s *Server) Handler() http.Handler {
	return corsMiddleware(s.mux)
}

// ListenAndServe starts the HTTP server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// corsMiddleware adds CORS headers to all responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "x-user-id, Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// staticHandler serves static files from the configured HTTP base directory,
// with SPA catch-all support (serves index.html for unmatched paths).
func (s *Server) staticHandler() http.Handler {
	baseDir := s.config.HTTPBaseDir
	fs := http.FileServer(http.Dir(baseDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the file exists, serve it
		path := filepath.Join(baseDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		// SPA catch-all: serve index.html
		http.ServeFile(w, r, filepath.Join(baseDir, "index.html"))
	})
}

// --- JSON helpers ------------------------------------------------------------

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// readJSON decodes the request body as JSON into v.
func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// writeError writes a plain-text error response with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

// pathID extracts and validates the {id} path parameter as a stream ID (0-63).
func pathID(r *http.Request) (uint8, error) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 0 || id > 63 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint8(id), nil
}
