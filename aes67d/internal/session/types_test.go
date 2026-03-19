package session

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStreamSourceJSONRoundtrip(t *testing.T) {
	src := StreamSource{
		ID:                  1,
		Enabled:             true,
		Name:                "AES67 Source",
		IO:                  "Audio Device",
		MaxSamplesPerPacket: 48,
		Codec:               "L24",
		Address:             "239.1.0.1",
		TTL:                 15,
		PayloadType:         98,
		DSCP:                34,
		RefClkPTPTraceable:  true,
		Map:                 []int{0, 1, 2, 3},
	}

	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got StreamSource
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(src, got) {
		t.Errorf("roundtrip mismatch:\n  got  %+v\n  want %+v", got, src)
	}
}

func TestStreamSinkJSONRoundtrip(t *testing.T) {
	sink := StreamSink{
		ID:               2,
		Name:              "AES67 Sink",
		IO:                "Audio Device",
		Source:            "http://example.com/sdp",
		UseSDP:            true,
		SDP:               "v=0\r\no=- 0 0 IN IP4 239.1.0.1\r\n",
		Delay:             1024,
		IgnoreRefClkGMID:  false,
		Map:               []int{0, 1},
	}

	data, err := json.Marshal(sink)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got StreamSink
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(sink, got) {
		t.Errorf("roundtrip mismatch:\n  got  %+v\n  want %+v", got, sink)
	}
}

func TestSinkStatusJSONFieldNames(t *testing.T) {
	status := SinkStatus{
		IsReceiving: true,
		MinTime:     -100,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	expectedKeys := []string{
		"is_rtp_seq_id_error",
		"is_rtp_ssrc_error",
		"is_rtp_payload_type_error",
		"is_rtp_sac_error",
		"is_receiving_rtp_packet",
		"is_some_muted",
		"is_all_muted",
		"is_muted",
		"sink_min_time",
	}

	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("missing expected JSON key %q", key)
		}
	}
}

func TestRemoteSourceJSON(t *testing.T) {
	rs := RemoteSource{
		Source:         "SAP",
		ID:             "abc123",
		Name:           "Remote Stream",
		Domain:         "local",
		Address:        "239.1.0.5",
		SDP:            "v=0\r\n",
		LastSeen:       10,
		AnnouncePeriod: 30,
	}

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got RemoteSource
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(rs, got) {
		t.Errorf("roundtrip mismatch:\n  got  %+v\n  want %+v", got, rs)
	}
}
