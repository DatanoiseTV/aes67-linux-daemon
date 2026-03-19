package api

import (
	"net/http"
)

// handleGetVersion returns the daemon version as a JSON string.
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"version": s.version})
}

// handleGetConfig returns the full daemon configuration as JSON,
// matching the C++ daemon response format.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.config)
}

// handleSetConfig updates daemon configuration fields from the request body.
func (s *Server) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var update map[string]interface{}
	if err := readJSON(r, &update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Apply known fields from the update map.
	cfg := s.config
	if v, ok := update["log_severity"]; ok {
		if f, ok := v.(float64); ok {
			cfg.LogSeverity = int(f)
		}
	}
	if v, ok := update["playout_delay"]; ok {
		if f, ok := v.(float64); ok {
			cfg.PlayoutDelay = int(f)
		}
	}
	if v, ok := update["tic_frame_size_at_1fs"]; ok {
		if f, ok := v.(float64); ok {
			cfg.TICFrameSize = int(f)
		}
	}
	if v, ok := update["max_tic_frame_size"]; ok {
		if f, ok := v.(float64); ok {
			cfg.MaxTICFrameSize = int(f)
		}
	}
	if v, ok := update["sample_rate"]; ok {
		if f, ok := v.(float64); ok {
			cfg.SampleRate = int(f)
		}
	}
	if v, ok := update["rtp_mcast_base"]; ok {
		if str, ok := v.(string); ok {
			cfg.RTPMcastBase = str
		}
	}
	if v, ok := update["rtp_port"]; ok {
		if f, ok := v.(float64); ok {
			cfg.RTPPort = int(f)
		}
	}
	if v, ok := update["ptp_domain"]; ok {
		if f, ok := v.(float64); ok {
			cfg.PTPDomain = int(f)
		}
	}
	if v, ok := update["ptp_dscp"]; ok {
		if f, ok := v.(float64); ok {
			cfg.PTPDSCP = int(f)
		}
	}
	if v, ok := update["sap_mcast_addr"]; ok {
		if str, ok := v.(string); ok {
			cfg.SAPMcastAddr = str
		}
	}
	if v, ok := update["sap_interval"]; ok {
		if f, ok := v.(float64); ok {
			cfg.SAPInterval = int(f)
		}
	}
	if v, ok := update["syslog_proto"]; ok {
		if str, ok := v.(string); ok {
			cfg.SyslogProto = str
		}
	}
	if v, ok := update["syslog_server"]; ok {
		if str, ok := v.(string); ok {
			cfg.SyslogServer = str
		}
	}
	if v, ok := update["status_file"]; ok {
		if str, ok := v.(string); ok {
			cfg.StatusFile = str
		}
	}
	if v, ok := update["mdns_enabled"]; ok {
		if b, ok := v.(bool); ok {
			cfg.MDNSEnabled = b
		}
	}
	if v, ok := update["custom_node_id"]; ok {
		if str, ok := v.(string); ok {
			cfg.CustomNodeID = str
		}
	}
	if v, ok := update["ptp_status_script"]; ok {
		if str, ok := v.(string); ok {
			cfg.PTPStatusScript = str
		}
	}
	if v, ok := update["auto_sinks_update"]; ok {
		if b, ok := v.(bool); ok {
			cfg.AutoSinksUpdate = b
		}
	}
	if v, ok := update["streamer_enabled"]; ok {
		if b, ok := v.(bool); ok {
			cfg.StreamerEnabled = b
		}
	}

	w.WriteHeader(http.StatusOK)
}
