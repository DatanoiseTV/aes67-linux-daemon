package api

import (
	"net/http"
	"strings"
)

// remoteSourceJSON is the JSON-compatible representation of a discovered remote source.
// It converts time.Time to Unix timestamp for C++ daemon format compatibility.
type remoteSourceJSON struct {
	Source         string `json:"source"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	Domain         string `json:"domain"`
	Address        string `json:"address"`
	SDP            string `json:"sdp"`
	LastSeen       int64  `json:"last_seen"`
	AnnouncePeriod int    `json:"announce_period"`
}

// handleBrowseSources returns discovered remote sources, optionally filtered.
func (s *Server) handleBrowseSources(w http.ResponseWriter, r *http.Request) {
	filter := r.PathValue("filter")

	sources := s.discovery.GetAllSources()

	var result []remoteSourceJSON
	for _, src := range sources {
		// Apply filter: "all" returns everything, otherwise filter by source type.
		if filter != "all" && !strings.EqualFold(src.Source, filter) {
			continue
		}
		result = append(result, remoteSourceJSON{
			Source:         src.Source,
			ID:             src.ID,
			Name:           src.Name,
			Domain:         src.Domain,
			Address:        src.Address,
			SDP:            src.SDP,
			LastSeen:       src.LastSeen.Unix(),
			AnnouncePeriod: src.AnnouncePeriod,
		})
	}

	if result == nil {
		result = []remoteSourceJSON{}
	}
	writeJSON(w, map[string]interface{}{"remote_sources": result})
}
