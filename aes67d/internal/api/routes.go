package api

// registerRoutes sets up all API and static file routes on the server's mux.
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("GET /api/version", s.handleGetVersion)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/config", s.handleSetConfig)
	s.mux.HandleFunc("GET /api/ptp/status", s.handleGetPTPStatus)
	s.mux.HandleFunc("GET /api/ptp/config", s.handleGetPTPConfig)
	s.mux.HandleFunc("POST /api/ptp/config", s.handleSetPTPConfig)
	s.mux.HandleFunc("GET /api/sources", s.handleGetSources)
	s.mux.HandleFunc("GET /api/source/sdp/{id}", s.handleGetSourceSDP)
	s.mux.HandleFunc("PUT /api/source/{id}", s.handlePutSource)
	s.mux.HandleFunc("DELETE /api/source/{id}", s.handleDeleteSource)
	s.mux.HandleFunc("GET /api/sinks", s.handleGetSinks)
	s.mux.HandleFunc("GET /api/sink/status/{id}", s.handleGetSinkStatus)
	s.mux.HandleFunc("PUT /api/sink/{id}", s.handlePutSink)
	s.mux.HandleFunc("DELETE /api/sink/{id}", s.handleDeleteSink)
	s.mux.HandleFunc("GET /api/browse/sources/{filter}", s.handleBrowseSources)
	s.mux.HandleFunc("GET /api/matrix", s.handleGetMatrix)
	s.mux.HandleFunc("PUT /api/matrix", s.handleSetMatrix)
	s.mux.HandleFunc("PUT /api/matrix/route", s.handleSetMatrixRoute)

	// Static files + SPA
	s.mux.Handle("/", s.staticHandler())
}
