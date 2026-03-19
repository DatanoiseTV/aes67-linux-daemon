package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Config represents the daemon configuration, compatible with the existing
// daemon.conf JSON format used by the C++ daemon.
type Config struct {
	HTTPPort             int    `json:"http_port"`
	RTSPPort             int    `json:"rtsp_port"`
	HTTPBaseDir          string `json:"http_base_dir"`
	LogSeverity          int    `json:"log_severity"`
	PlayoutDelay         int    `json:"playout_delay"`
	TICFrameSize         int    `json:"tic_frame_size_at_1fs"`
	MaxTICFrameSize      int    `json:"max_tic_frame_size"`
	SampleRate           int    `json:"sample_rate"`
	RTPMcastBase         string `json:"rtp_mcast_base"`
	RTPPort              int    `json:"rtp_port"`
	PTPDomain            int    `json:"ptp_domain"`
	PTPDSCP              int    `json:"ptp_dscp"`
	SAPMcastAddr         string `json:"sap_mcast_addr"`
	SAPInterval          int    `json:"sap_interval"`
	SyslogProto          string `json:"syslog_proto"`
	SyslogServer         string `json:"syslog_server"`
	StatusFile           string `json:"status_file"`
	InterfaceName        string `json:"interface_name"`
	MDNSEnabled          bool   `json:"mdns_enabled"`
	CustomNodeID         string `json:"custom_node_id"`
	PTPStatusScript      string `json:"ptp_status_script"`
	StreamerChannels     int    `json:"streamer_channels"`
	StreamerFilesNum     int    `json:"streamer_files_num"`
	StreamerFileDuration int    `json:"streamer_file_duration"`
	StreamerBufferFiles  int    `json:"streamer_player_buffer_files_num"`
	StreamerEnabled      bool   `json:"streamer_enabled"`
	AutoSinksUpdate      bool   `json:"auto_sinks_update"`

	// Runtime fields populated at startup, not persisted.
	NodeID  string `json:"node_id,omitempty"`
	IPAddr  string `json:"ip_addr,omitempty"`
	MACAddr string `json:"mac_addr,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults matching the
// reference daemon.conf shipped with the C++ daemon.
func DefaultConfig() *Config {
	return &Config{
		HTTPPort:             8080,
		RTSPPort:             8854,
		HTTPBaseDir:          "../webui/dist",
		LogSeverity:          2,
		PlayoutDelay:         0,
		TICFrameSize:         48,
		MaxTICFrameSize:      1024,
		SampleRate:           48000,
		RTPMcastBase:         "239.1.0.1",
		RTPPort:              5004,
		PTPDomain:            0,
		PTPDSCP:              48,
		SAPMcastAddr:         "239.255.255.255",
		SAPInterval:          30,
		SyslogProto:          "none",
		SyslogServer:         "255.255.255.254:1234",
		StatusFile:           "./status.json",
		InterfaceName:        "lo",
		MDNSEnabled:          true,
		CustomNodeID:         "",
		PTPStatusScript:      "./scripts/ptp_status.sh",
		StreamerChannels:     8,
		StreamerFilesNum:     8,
		StreamerFileDuration: 1,
		StreamerBufferFiles:  1,
		StreamerEnabled:      false,
		AutoSinksUpdate:      true,
	}
}

// Load reads a JSON configuration file and returns a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the configuration as indented JSON to the given path.
// Runtime-only fields (NodeID, IPAddr, MACAddr) are omitted when empty.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// PopulateRuntime fills NodeID, IPAddr, and MACAddr from the network
// interface specified by InterfaceName.
func (c *Config) PopulateRuntime() error {
	iface, err := net.InterfaceByName(c.InterfaceName)
	if err != nil {
		return fmt.Errorf("config: interface %q: %w", c.InterfaceName, err)
	}

	c.MACAddr = iface.HardwareAddr.String()

	addrs, err := iface.Addrs()
	if err != nil {
		return fmt.Errorf("config: addrs for %q: %w", c.InterfaceName, err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 != nil {
			c.IPAddr = ip4.String()
			break
		}
	}

	if c.CustomNodeID != "" {
		c.NodeID = c.CustomNodeID
	} else {
		c.NodeID = nodeIDFromMAC(iface.HardwareAddr)
	}

	return nil
}

// nodeIDFromMAC generates a node identifier from a MAC address,
// matching the format used by the C++ daemon (uppercase hex without separators).
func nodeIDFromMAC(mac net.HardwareAddr) string {
	hex := strings.ToUpper(strings.ReplaceAll(mac.String(), ":", ""))
	return hex
}
