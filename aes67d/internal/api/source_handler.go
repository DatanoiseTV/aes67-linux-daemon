package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/session"
)

// handleGetSources returns all active sources wrapped in a JSON object.
func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	sources := s.session.GetSources()
	if sources == nil {
		sources = []session.StreamSource{}
	}
	writeJSON(w, map[string]interface{}{"sources": sources})
}

// handleGetSourceSDP returns the SDP text for a source as text/plain.
func (s *Server) handleGetSourceSDP(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	src, ok := s.session.GetSource(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("source %d not found", id))
		return
	}

	sdpInfo := s.buildSDPInfoFromSource(src)
	sdpText := session.GenerateSDP(sdpInfo)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(sdpText))
}

// buildSDPInfoFromSource constructs an SDPInfo from a StreamSource and daemon config.
func (s *Server) buildSDPInfoFromSource(src session.StreamSource) session.SDPInfo {
	codec := src.Codec
	if codec == "" {
		codec = "L24"
	}

	channels := len(src.Map)
	if channels == 0 {
		channels = 1
	}

	sampleRate := s.config.SampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}

	// Compute multicast address from base + source ID
	baseIP := net.ParseIP(s.config.RTPMcastBase).To4()
	mcastAddr := s.config.RTPMcastBase
	if baseIP != nil {
		destIP := make(net.IP, 4)
		copy(destIP, baseIP)
		destIP[3] = baseIP[3] + src.ID
		mcastAddr = destIP.String()
	}

	port := s.config.RTPPort
	if port == 0 {
		port = 5004
	}

	payloadType := int(src.PayloadType)
	if payloadType == 0 {
		payloadType = 98
	}

	// Compute ptime from max_samples_per_packet
	maxSamples := int(src.MaxSamplesPerPacket)
	if maxSamples == 0 {
		maxSamples = sampleRate / 1000 // 1ms default
	}
	ptime := float64(maxSamples) * 1000.0 / float64(sampleRate)

	return session.SDPInfo{
		SessionName:    src.Name,
		Origin:         fmt.Sprintf("- 0 0 IN IP4 %s", s.config.IPAddr),
		ConnectionAddr: mcastAddr,
		TTL:            int(src.TTL),
		Port:           port,
		PayloadType:    payloadType,
		Codec:          codec,
		SampleRate:     sampleRate,
		Channels:       channels,
		PTPDomain:      s.config.PTPDomain,
		PTPTraceable:   src.RefClkPTPTraceable,
		Direction:      "sendonly",
		PTime:          ptime,
	}
}

// handlePutSource creates or updates a source stream.
func (s *Server) handlePutSource(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var src session.StreamSource
	if err := readJSON(r, &src); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	src.ID = id

	if err := s.session.AddSource(src); err != nil {
		writeError(w, http.StatusInternalServerError, "add source: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleDeleteSource removes a source stream.
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.session.RemoveSource(id); err != nil {
		writeError(w, http.StatusNotFound, "remove source: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
