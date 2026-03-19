package session

import (
	"strings"
	"testing"
)

func TestParseSDP(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=- 1234 1 IN IP4 192.168.1.10\r\n" +
		"s=AES67 Source\r\n" +
		"c=IN IP4 239.69.0.1/32\r\n" +
		"t=0 0\r\n" +
		"m=audio 5004 RTP/AVP 98\r\n" +
		"a=rtpmap:98 L24/48000/2\r\n" +
		"a=ptime:1\r\n" +
		"a=clock-domain:PTPv2 0\r\n" +
		"a=ts-refclk:ptp=traceable\r\n" +
		"a=sendonly\r\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	assertEqual(t, "SessionName", info.SessionName, "AES67 Source")
	assertEqual(t, "Origin", info.Origin, "- 1234 1 IN IP4 192.168.1.10")
	assertEqual(t, "ConnectionAddr", info.ConnectionAddr, "239.69.0.1")
	assertEqualInt(t, "TTL", info.TTL, 32)
	assertEqualInt(t, "Port", info.Port, 5004)
	assertEqualInt(t, "PayloadType", info.PayloadType, 98)
	assertEqual(t, "Codec", info.Codec, "L24")
	assertEqualInt(t, "SampleRate", info.SampleRate, 48000)
	assertEqualInt(t, "Channels", info.Channels, 2)
	assertEqualInt(t, "PTPDomain", info.PTPDomain, 0)
	if !info.PTPTraceable {
		t.Error("expected PTPTraceable to be true")
	}
	assertEqual(t, "Direction", info.Direction, "sendonly")
	assertEqualFloat(t, "PTime", info.PTime, 1.0)
}

func TestParseSDPL16Mono(t *testing.T) {
	sdp := "v=0\n" +
		"o=- 5678 2 IN IP4 10.0.0.5\n" +
		"s=L16 Mono Stream\n" +
		"c=IN IP4 239.69.1.1/64\n" +
		"t=0 0\n" +
		"m=audio 5004 RTP/AVP 96\n" +
		"a=rtpmap:96 L16/44100/1\n" +
		"a=ptime:0.125\n" +
		"a=clock-domain:PTPv2 1\n" +
		"a=ts-refclk:ptp=IEEE1588-2008:00-11-22-FF-FE-33-44-55:0\n" +
		"a=recvonly\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	assertEqual(t, "Codec", info.Codec, "L16")
	assertEqualInt(t, "SampleRate", info.SampleRate, 44100)
	assertEqualInt(t, "Channels", info.Channels, 1)
	assertEqualInt(t, "TTL", info.TTL, 64)
	assertEqualInt(t, "PTPDomain", info.PTPDomain, 1)
	assertEqualFloat(t, "PTime", info.PTime, 0.125)
	assertEqual(t, "Direction", info.Direction, "recvonly")
	if info.PTPTraceable {
		t.Error("expected PTPTraceable to be false for non-traceable ref")
	}
}

func TestParseSDPAM824(t *testing.T) {
	sdp := "v=0\n" +
		"o=- 9999 1 IN IP4 192.168.0.100\n" +
		"s=AM824 8ch\n" +
		"c=IN IP4 239.69.2.1/32\n" +
		"t=0 0\n" +
		"m=audio 5004 RTP/AVP 97\n" +
		"a=rtpmap:97 AM824/48000/8\n" +
		"a=ptime:1\n" +
		"a=clock-domain:PTPv2 0\n" +
		"a=ts-refclk:ptp=traceable\n" +
		"a=sendonly\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	assertEqual(t, "Codec", info.Codec, "AM824")
	assertEqualInt(t, "Channels", info.Channels, 8)
	assertEqualInt(t, "PayloadType", info.PayloadType, 97)
}

func TestParseSDPMultiChannel(t *testing.T) {
	sdp := "v=0\n" +
		"o=- 42 1 IN IP4 10.0.0.1\n" +
		"s=64ch Stream\n" +
		"c=IN IP4 239.69.3.1/32\n" +
		"t=0 0\n" +
		"m=audio 5004 RTP/AVP 96\n" +
		"a=rtpmap:96 L24/96000/64\n" +
		"a=ptime:1\n" +
		"a=clock-domain:PTPv2 0\n" +
		"a=ts-refclk:ptp=traceable\n" +
		"a=sendonly\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	assertEqualInt(t, "Channels", info.Channels, 64)
	assertEqualInt(t, "SampleRate", info.SampleRate, 96000)
}

func TestParseSDPMissingVersion(t *testing.T) {
	sdp := "o=- 1234 1 IN IP4 192.168.1.10\n" +
		"s=No Version\n" +
		"c=IN IP4 239.69.0.1/32\n"

	_, err := ParseSDP(sdp)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version-related error, got: %v", err)
	}
}

func TestParseSDPBadVersion(t *testing.T) {
	sdp := "v=1\n" +
		"o=- 1234 1 IN IP4 192.168.1.10\n" +
		"s=Bad Version\n"

	_, err := ParseSDP(sdp)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestParseSDPNoTTL(t *testing.T) {
	sdp := "v=0\n" +
		"o=- 1234 1 IN IP4 192.168.1.10\n" +
		"s=No TTL\n" +
		"c=IN IP4 239.69.0.1\n" +
		"t=0 0\n" +
		"m=audio 5004 RTP/AVP 98\n" +
		"a=rtpmap:98 L24/48000/2\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	assertEqual(t, "ConnectionAddr", info.ConnectionAddr, "239.69.0.1")
	assertEqualInt(t, "TTL", info.TTL, 0)
}

func TestParseSDPExtraWhitespace(t *testing.T) {
	sdp := "  v=0  \n" +
		"  o=- 1234 1 IN IP4 192.168.1.10  \n" +
		"  s=Whitespace Test  \n" +
		"  c=IN IP4 239.69.0.1/32  \n" +
		"  t=0 0  \n" +
		"  m=audio 5004 RTP/AVP 98  \n" +
		"  a=rtpmap:98 L24/48000/2  \n" +
		"  a=ptime:1  \n" +
		"  a=sendonly  \n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	assertEqual(t, "SessionName", info.SessionName, "Whitespace Test")
	assertEqual(t, "Direction", info.Direction, "sendonly")
}

func TestParseSDPRtpmapNoChannels(t *testing.T) {
	sdp := "v=0\n" +
		"o=- 1 1 IN IP4 10.0.0.1\n" +
		"s=Mono Default\n" +
		"c=IN IP4 239.69.0.1/32\n" +
		"m=audio 5004 RTP/AVP 96\n" +
		"a=rtpmap:96 L16/48000\n"

	info, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	assertEqualInt(t, "Channels", info.Channels, 1)
}

func TestGenerateSDP(t *testing.T) {
	info := SDPInfo{
		SessionName:    "Test Stream",
		Origin:         "- 1234 1 IN IP4 192.168.1.10",
		ConnectionAddr: "239.69.0.1",
		TTL:            32,
		Port:           5004,
		PayloadType:    98,
		Codec:          "L24",
		SampleRate:     48000,
		Channels:       2,
		PTPDomain:      0,
		PTPTraceable:   true,
		Direction:      "sendonly",
		PTime:          1,
	}

	sdp := GenerateSDP(info)

	if !strings.Contains(sdp, "v=0") {
		t.Error("missing v=0")
	}
	if !strings.Contains(sdp, "o=- 1234 1 IN IP4 192.168.1.10") {
		t.Error("missing origin line")
	}
	if !strings.Contains(sdp, "s=Test Stream") {
		t.Error("missing session name")
	}
	if !strings.Contains(sdp, "c=IN IP4 239.69.0.1/32") {
		t.Error("missing connection line")
	}
	if !strings.Contains(sdp, "m=audio 5004 RTP/AVP 98") {
		t.Error("missing media line")
	}
	if !strings.Contains(sdp, "a=rtpmap:98 L24/48000/2") {
		t.Error("missing rtpmap")
	}
	if !strings.Contains(sdp, "a=ptime:1") {
		t.Error("missing ptime")
	}
	if !strings.Contains(sdp, "a=clock-domain:PTPv2 0") {
		t.Error("missing clock-domain")
	}
	if !strings.Contains(sdp, "a=ts-refclk:ptp=traceable") {
		t.Error("missing ts-refclk")
	}
	if !strings.Contains(sdp, "a=sendonly") {
		t.Error("missing direction")
	}
}

func TestGenerateSDPRoundtrip(t *testing.T) {
	original := SDPInfo{
		SessionName:    "Roundtrip Test",
		Origin:         "- 5678 3 IN IP4 10.0.0.5",
		ConnectionAddr: "239.69.1.1",
		TTL:            64,
		Port:           5004,
		PayloadType:    96,
		Codec:          "L16",
		SampleRate:     44100,
		Channels:       1,
		PTPDomain:      1,
		PTPTraceable:   true,
		Direction:      "recvonly",
		PTime:          0.125,
	}

	sdpText := GenerateSDP(original)
	parsed, err := ParseSDP(sdpText)
	if err != nil {
		t.Fatalf("roundtrip ParseSDP failed: %v", err)
	}

	assertEqual(t, "SessionName", parsed.SessionName, original.SessionName)
	assertEqual(t, "ConnectionAddr", parsed.ConnectionAddr, original.ConnectionAddr)
	assertEqualInt(t, "TTL", parsed.TTL, original.TTL)
	assertEqualInt(t, "Port", parsed.Port, original.Port)
	assertEqualInt(t, "PayloadType", parsed.PayloadType, original.PayloadType)
	assertEqual(t, "Codec", parsed.Codec, original.Codec)
	assertEqualInt(t, "SampleRate", parsed.SampleRate, original.SampleRate)
	assertEqualInt(t, "Channels", parsed.Channels, original.Channels)
	assertEqualInt(t, "PTPDomain", parsed.PTPDomain, original.PTPDomain)
	if parsed.PTPTraceable != original.PTPTraceable {
		t.Errorf("PTPTraceable: got %v, want %v", parsed.PTPTraceable, original.PTPTraceable)
	}
	assertEqual(t, "Direction", parsed.Direction, original.Direction)
	assertEqualFloat(t, "PTime", parsed.PTime, original.PTime)
}

func TestGenerateSDPDefaultDirection(t *testing.T) {
	info := SDPInfo{
		SessionName:    "Default Direction",
		ConnectionAddr: "239.69.0.1",
		Port:           5004,
		PayloadType:    98,
		Codec:          "L24",
		SampleRate:     48000,
		Channels:       2,
		PTime:          1,
	}

	sdp := GenerateSDP(info)
	if !strings.Contains(sdp, "a=sendonly") {
		t.Error("expected default direction to be sendonly")
	}
}

func TestGenerateSDPDefaultTTL(t *testing.T) {
	info := SDPInfo{
		SessionName:    "Default TTL",
		ConnectionAddr: "239.69.0.1",
		Port:           5004,
		PayloadType:    98,
		Codec:          "L24",
		SampleRate:     48000,
		Channels:       2,
		PTime:          1,
	}

	sdp := GenerateSDP(info)
	if !strings.Contains(sdp, "239.69.0.1/32") {
		t.Error("expected default TTL of 32")
	}
}

func TestFormatPTime(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{1.0, "1"},
		{0.125, "0.125"},
		{0.33333, "0.33333"},
		{4.0, "4"},
	}
	for _, tc := range tests {
		got := formatPTime(tc.input)
		if got != tc.expected {
			t.Errorf("formatPTime(%v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// Test helpers

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func assertEqualInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}

func assertEqualFloat(t *testing.T, field string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %f, want %f", field, got, want)
	}
}
