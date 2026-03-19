package discovery

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestParseSAPPacket_Announcement(t *testing.T) {
	sdp := "v=0\r\ns=Test Stream\r\nc=IN IP4 239.1.2.3/32\r\na=clock-domain:PTPv2\r\n"
	srcIP := net.IPv4(192, 168, 1, 100)
	msgID := uint16(0xABCD)

	pkt := buildTestSAPPacket(sdp, srcIP, msgID, false)

	parsed, err := ParseSAPPacket(pkt)
	if err != nil {
		t.Fatalf("ParseSAPPacket returned error: %v", err)
	}

	if parsed.Version != SAPVersion {
		t.Errorf("version = %d, want %d", parsed.Version, SAPVersion)
	}
	if parsed.IsDeletion {
		t.Error("IsDeletion = true, want false")
	}
	if parsed.MsgIDHash != msgID {
		t.Errorf("MsgIDHash = 0x%04x, want 0x%04x", parsed.MsgIDHash, msgID)
	}
	if !parsed.OriginIP.Equal(srcIP.To4()) {
		t.Errorf("OriginIP = %v, want %v", parsed.OriginIP, srcIP)
	}
	if parsed.SDP != sdp {
		t.Errorf("SDP = %q, want %q", parsed.SDP, sdp)
	}
}

func TestParseSAPPacket_Deletion(t *testing.T) {
	sdp := "v=0\r\ns=Deleted Stream\r\n"
	srcIP := net.IPv4(10, 0, 0, 1)
	msgID := uint16(0x1234)

	pkt := buildTestSAPPacket(sdp, srcIP, msgID, true)

	parsed, err := ParseSAPPacket(pkt)
	if err != nil {
		t.Fatalf("ParseSAPPacket returned error: %v", err)
	}

	if !parsed.IsDeletion {
		t.Error("IsDeletion = false, want true")
	}
	if parsed.MsgIDHash != msgID {
		t.Errorf("MsgIDHash = 0x%04x, want 0x%04x", parsed.MsgIDHash, msgID)
	}
}

func TestParseSAPPacket_TooShort(t *testing.T) {
	_, err := ParseSAPPacket([]byte{0x20, 0x00, 0x00})
	if err == nil {
		t.Error("expected error for short packet, got nil")
	}
}

func TestParseSAPPacket_BadVersion(t *testing.T) {
	pkt := make([]byte, 8)
	pkt[0] = 0x40 // version 2
	_, err := ParseSAPPacket(pkt)
	if err == nil {
		t.Error("expected error for bad version, got nil")
	}
}

func TestBuildSAPPacket_Roundtrip(t *testing.T) {
	sdp := "v=0\r\ns=Roundtrip Test\r\nc=IN IP4 239.0.0.1\r\n"
	srcIP := net.IPv4(172, 16, 0, 5)
	hash := uint16(0x5678)

	raw := BuildSAPPacket(sdp, srcIP, hash, false)
	if raw == nil {
		t.Fatal("BuildSAPPacket returned nil")
	}

	parsed, err := ParseSAPPacket(raw)
	if err != nil {
		t.Fatalf("ParseSAPPacket on built packet: %v", err)
	}

	if parsed.MsgIDHash != hash {
		t.Errorf("roundtrip MsgIDHash = 0x%04x, want 0x%04x", parsed.MsgIDHash, hash)
	}
	if !parsed.OriginIP.Equal(srcIP.To4()) {
		t.Errorf("roundtrip OriginIP = %v, want %v", parsed.OriginIP, srcIP)
	}
	if parsed.SDP != sdp {
		t.Errorf("roundtrip SDP mismatch")
	}
	if parsed.IsDeletion {
		t.Error("roundtrip IsDeletion = true, want false")
	}
}

func TestBuildSAPPacket_Deletion(t *testing.T) {
	sdp := "v=0\r\ns=Delete Me\r\n"
	srcIP := net.IPv4(192, 168, 0, 1)
	hash := uint16(0x9999)

	raw := BuildSAPPacket(sdp, srcIP, hash, true)
	parsed, err := ParseSAPPacket(raw)
	if err != nil {
		t.Fatalf("ParseSAPPacket: %v", err)
	}
	if !parsed.IsDeletion {
		t.Error("deletion flag not set in built packet")
	}
}

func TestExtractSDPField(t *testing.T) {
	sdp := "v=0\r\ns=My Audio Stream\r\nc=IN IP4 239.1.2.3/32\r\na=clock-domain:PTPv2\r\n"

	if got := extractSDPField(sdp, "s="); got != "My Audio Stream" {
		t.Errorf("s= field = %q, want %q", got, "My Audio Stream")
	}
	if got := extractSDPField(sdp, "v="); got != "0" {
		t.Errorf("v= field = %q, want %q", got, "0")
	}
	if got := extractSDPField(sdp, "z="); got != "" {
		t.Errorf("z= field = %q, want empty", got)
	}
}

func TestExtractSDPConnectionAddress(t *testing.T) {
	tests := []struct {
		sdp  string
		want string
	}{
		{"c=IN IP4 239.1.2.3/32\r\n", "239.1.2.3"},
		{"c=IN IP4 239.0.0.1\r\n", "239.0.0.1"},
		{"v=0\r\n", ""},
	}

	for _, tt := range tests {
		got := extractSDPConnectionAddress(tt.sdp)
		if got != tt.want {
			t.Errorf("extractSDPConnectionAddress(%q) = %q, want %q", tt.sdp, got, tt.want)
		}
	}
}

func TestExtractSDPAttribute(t *testing.T) {
	sdp := "v=0\r\na=clock-domain:PTPv2\r\na=recvonly\r\n"

	if got := extractSDPAttribute(sdp, "clock-domain"); got != "PTPv2" {
		t.Errorf("clock-domain = %q, want %q", got, "PTPv2")
	}
	if got := extractSDPAttribute(sdp, "nonexistent"); got != "" {
		t.Errorf("nonexistent = %q, want empty", got)
	}
}

func TestSourceExpiry(t *testing.T) {
	listener := NewSAPListener("")
	now := time.Now()

	// Add a source that should be expired (last seen well beyond 3x announce period).
	listener.sources["expired"] = &RemoteSource{
		ID:             "expired",
		LastSeen:       now.Add(-1000 * time.Second),
		AnnouncePeriod: 10, // 3 * 10 = 30 seconds
	}

	// Add a source that should NOT be expired.
	listener.sources["active"] = &RemoteSource{
		ID:             "active",
		LastSeen:       now,
		AnnouncePeriod: 300,
	}

	listener.expireSources()

	if _, ok := listener.sources["expired"]; ok {
		t.Error("expired source was not removed")
	}
	if _, ok := listener.sources["active"]; !ok {
		t.Error("active source was incorrectly removed")
	}
}

func TestHandlePacket_AnnouncementAndDeletion(t *testing.T) {
	listener := NewSAPListener("")

	sdp := "v=0\r\ns=Test\r\nc=IN IP4 239.0.0.1\r\n"
	srcIP := net.IPv4(192, 168, 1, 1)
	msgID := uint16(0x1111)

	// Announce.
	pkt := buildTestSAPPacket(sdp, srcIP, msgID, false)
	listener.handlePacket(pkt)

	sources := listener.GetSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source after announcement, got %d", len(sources))
	}
	if sources[0].Name != "Test" {
		t.Errorf("name = %q, want %q", sources[0].Name, "Test")
	}

	// Delete.
	delPkt := buildTestSAPPacket(sdp, srcIP, msgID, true)
	listener.handlePacket(delPkt)

	sources = listener.GetSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 sources after deletion, got %d", len(sources))
	}
}

func TestSAPHash(t *testing.T) {
	ip := net.IPv4(192, 168, 1, 1)
	sdp := "v=0\r\ns=Test\r\n"

	h1 := sapHash(ip, sdp)
	h2 := sapHash(ip, sdp)
	if h1 != h2 {
		t.Error("same input produced different hashes")
	}

	h3 := sapHash(net.IPv4(10, 0, 0, 1), sdp)
	if h1 == h3 {
		t.Error("different IPs produced same hash (unlikely)")
	}
}

// buildTestSAPPacket constructs a raw SAP packet for testing.
func buildTestSAPPacket(sdp string, srcIP net.IP, msgID uint16, isDeletion bool) []byte {
	flags := byte(SAPVersion << 5)
	if isDeletion {
		flags |= 0x04
	}

	header := make([]byte, 8)
	header[0] = flags
	header[1] = 0 // auth len
	binary.BigEndian.PutUint16(header[2:4], msgID)
	copy(header[4:8], srcIP.To4())

	mime := []byte(SAPMIMEType + "\x00")
	pkt := make([]byte, 0, len(header)+len(mime)+len(sdp))
	pkt = append(pkt, header...)
	pkt = append(pkt, mime...)
	pkt = append(pkt, []byte(sdp)...)
	return pkt
}
