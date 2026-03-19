package netlink

import (
	"testing"
)

func TestRTPStreamInfoMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &RTPStreamInfo{
		SizeOf:           RTPStreamInfoSize,
		Is8021Q:          1,
		VLANId:           100,
		IfPortId:         42,
		PlayOutDelay:     1000,
		FrameSize:        48,
		MaxSamplesPerPkt: 48,
		DSCP:             46,
		RTCPSrcIP:        0xC0A80101,
		SrcIP:            0xC0A80101,
		DestIP:           0xEF010101,
		TTL:              128,
		SrcPort:          5004,
		DestPort:         5004,
		RTCPSrcPort:      5005,
		RTCPDestPort:     5005,
		PayloadType:      97,
		SSRC:             0x12345678,
		SSRCInitialized:  1,
		RTPTimestampOff:  0,
		SamplingRate:     48000,
		WordLength:       24,
		NbOfChannels:     2,
		IsSource:         1,
		Id:               7,
	}

	copy(orig.Name[:], "test-stream")
	copy(orig.DestMAC[:], []byte{0x01, 0x00, 0x5E, 0x01, 0x01, 0x01})
	copy(orig.Codec[:], "L24")

	orig.Routing[0] = 1
	orig.Routing[1] = 2

	data, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) != RTPStreamInfoSize {
		t.Fatalf("Marshal produced %d bytes, expected %d", len(data), RTPStreamInfoSize)
	}

	decoded := &RTPStreamInfo{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify all fields
	if decoded.SizeOf != orig.SizeOf {
		t.Errorf("SizeOf: got %d, want %d", decoded.SizeOf, orig.SizeOf)
	}
	if decoded.Is8021Q != orig.Is8021Q {
		t.Errorf("Is8021Q: got %d, want %d", decoded.Is8021Q, orig.Is8021Q)
	}
	if decoded.VLANId != orig.VLANId {
		t.Errorf("VLANId: got %d, want %d", decoded.VLANId, orig.VLANId)
	}
	if decoded.IfPortId != orig.IfPortId {
		t.Errorf("IfPortId: got %d, want %d", decoded.IfPortId, orig.IfPortId)
	}
	if decoded.Name != orig.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.PlayOutDelay != orig.PlayOutDelay {
		t.Errorf("PlayOutDelay: got %d, want %d", decoded.PlayOutDelay, orig.PlayOutDelay)
	}
	if decoded.FrameSize != orig.FrameSize {
		t.Errorf("FrameSize: got %d, want %d", decoded.FrameSize, orig.FrameSize)
	}
	if decoded.MaxSamplesPerPkt != orig.MaxSamplesPerPkt {
		t.Errorf("MaxSamplesPerPkt: got %d, want %d", decoded.MaxSamplesPerPkt, orig.MaxSamplesPerPkt)
	}
	if decoded.DestMAC != orig.DestMAC {
		t.Errorf("DestMAC mismatch")
	}
	if decoded.DSCP != orig.DSCP {
		t.Errorf("DSCP: got %d, want %d", decoded.DSCP, orig.DSCP)
	}
	if decoded.SrcIP != orig.SrcIP {
		t.Errorf("SrcIP: got %#x, want %#x", decoded.SrcIP, orig.SrcIP)
	}
	if decoded.DestIP != orig.DestIP {
		t.Errorf("DestIP: got %#x, want %#x", decoded.DestIP, orig.DestIP)
	}
	if decoded.TTL != orig.TTL {
		t.Errorf("TTL: got %d, want %d", decoded.TTL, orig.TTL)
	}
	if decoded.SrcPort != orig.SrcPort {
		t.Errorf("SrcPort: got %d, want %d", decoded.SrcPort, orig.SrcPort)
	}
	if decoded.DestPort != orig.DestPort {
		t.Errorf("DestPort: got %d, want %d", decoded.DestPort, orig.DestPort)
	}
	if decoded.RTCPSrcPort != orig.RTCPSrcPort {
		t.Errorf("RTCPSrcPort: got %d, want %d", decoded.RTCPSrcPort, orig.RTCPSrcPort)
	}
	if decoded.RTCPDestPort != orig.RTCPDestPort {
		t.Errorf("RTCPDestPort: got %d, want %d", decoded.RTCPDestPort, orig.RTCPDestPort)
	}
	if decoded.PayloadType != orig.PayloadType {
		t.Errorf("PayloadType: got %d, want %d", decoded.PayloadType, orig.PayloadType)
	}
	if decoded.SSRC != orig.SSRC {
		t.Errorf("SSRC: got %#x, want %#x", decoded.SSRC, orig.SSRC)
	}
	if decoded.SSRCInitialized != orig.SSRCInitialized {
		t.Errorf("SSRCInitialized: got %d, want %d", decoded.SSRCInitialized, orig.SSRCInitialized)
	}
	if decoded.SamplingRate != orig.SamplingRate {
		t.Errorf("SamplingRate: got %d, want %d", decoded.SamplingRate, orig.SamplingRate)
	}
	if decoded.Codec != orig.Codec {
		t.Errorf("Codec mismatch")
	}
	if decoded.WordLength != orig.WordLength {
		t.Errorf("WordLength: got %d, want %d", decoded.WordLength, orig.WordLength)
	}
	if decoded.NbOfChannels != orig.NbOfChannels {
		t.Errorf("NbOfChannels: got %d, want %d", decoded.NbOfChannels, orig.NbOfChannels)
	}
	if decoded.IsSource != orig.IsSource {
		t.Errorf("IsSource: got %d, want %d", decoded.IsSource, orig.IsSource)
	}
	if decoded.Id != orig.Id {
		t.Errorf("Id: got %d, want %d", decoded.Id, orig.Id)
	}
	if decoded.Routing[0] != 1 || decoded.Routing[1] != 2 {
		t.Errorf("Routing: got [%d, %d], want [1, 2]", decoded.Routing[0], decoded.Routing[1])
	}
}

func TestRTPStreamInfoSize(t *testing.T) {
	// Verify the computed constant matches actual serialization
	info := &RTPStreamInfo{}
	data, err := info.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) != RTPStreamInfoSize {
		t.Errorf("Marshaled size %d != RTPStreamInfoSize constant %d", len(data), RTPStreamInfoSize)
	}
}

func TestRTPStreamInfoUnmarshalTooShort(t *testing.T) {
	info := &RTPStreamInfo{}
	err := info.Unmarshal([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestPTPConfigMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &PTPConfig{Domain: 127, DSCP: 46}

	data, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) != PTPConfigSize {
		t.Fatalf("size: got %d, want %d", len(data), PTPConfigSize)
	}

	decoded := &PTPConfig{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if *decoded != *orig {
		t.Errorf("got %+v, want %+v", decoded, orig)
	}
}

func TestPTPConfigUnmarshalTooShort(t *testing.T) {
	cfg := &PTPConfig{}
	if err := cfg.Unmarshal([]byte{1}); err == nil {
		t.Error("expected error for short data")
	}
}

func TestPTPStatusMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &PTPStatus{
		LockStatus: 1,
		GMID:       0xAABBCCDDEEFF0011,
		Jitter:     -42,
	}

	data, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) != PTPStatusSize {
		t.Fatalf("size: got %d, want %d", len(data), PTPStatusSize)
	}

	decoded := &PTPStatus{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if *decoded != *orig {
		t.Errorf("got %+v, want %+v", decoded, orig)
	}
}

func TestPTPStatusUnmarshalTooShort(t *testing.T) {
	s := &PTPStatus{}
	if err := s.Unmarshal(make([]byte, 10)); err == nil {
		t.Error("expected error for short data")
	}
}

func TestStreamStatusMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &StreamStatus{
		Flags:   0xFF00FF00,
		MinTime: -100,
	}

	data, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) != StreamStatusSize {
		t.Fatalf("size: got %d, want %d", len(data), StreamStatusSize)
	}

	decoded := &StreamStatus{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if *decoded != *orig {
		t.Errorf("got %+v, want %+v", decoded, orig)
	}
}

func TestStreamStatusUnmarshalTooShort(t *testing.T) {
	s := &StreamStatus{}
	if err := s.Unmarshal(make([]byte, 4)); err == nil {
		t.Error("expected error for short data")
	}
}
