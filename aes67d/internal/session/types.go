package session

// StreamSource represents an AES67 audio source (transmitter).
type StreamSource struct {
	ID                  uint8  `json:"id"`
	Enabled             bool   `json:"enabled"`
	Name                string `json:"name"`
	IO                  string `json:"io"`
	MaxSamplesPerPacket uint32 `json:"max_samples_per_packet"`
	Codec               string `json:"codec"`
	Address             string `json:"address"`
	TTL                 uint8  `json:"ttl"`
	PayloadType         uint8  `json:"payload_type"`
	DSCP                uint8  `json:"dscp"`
	RefClkPTPTraceable  bool   `json:"refclk_ptp_traceable"`
	Map                 []int  `json:"map"`
}

// StreamSink represents an AES67 audio sink (receiver).
type StreamSink struct {
	ID               uint8  `json:"id"`
	Name             string `json:"name"`
	IO               string `json:"io"`
	Source           string `json:"source"`
	UseSDP           bool   `json:"use_sdp"`
	SDP              string `json:"sdp"`
	Delay            uint32 `json:"delay"`
	IgnoreRefClkGMID bool   `json:"ignore_refclk_gmid"`
	Map              []int  `json:"map"`
}

// SinkStatus holds runtime status information for a sink.
type SinkStatus struct {
	IsSeqIDError       bool  `json:"is_rtp_seq_id_error"`
	IsSSRCError        bool  `json:"is_rtp_ssrc_error"`
	IsPayloadTypeError bool  `json:"is_rtp_payload_type_error"`
	IsSACError         bool  `json:"is_rtp_sac_error"`
	IsReceiving        bool  `json:"is_receiving_rtp_packet"`
	IsSomeMuted        bool  `json:"is_some_muted"`
	IsAllMuted         bool  `json:"is_all_muted"`
	IsMuted            bool  `json:"is_muted"`
	MinTime            int32 `json:"sink_min_time"`
}

// RemoteSource represents a source discovered via SAP or mDNS.
type RemoteSource struct {
	Source         string `json:"source"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	Domain         string `json:"domain"`
	Address        string `json:"address"`
	SDP            string `json:"sdp"`
	LastSeen       int64  `json:"last_seen"`
	AnnouncePeriod int    `json:"announce_period"`
}
