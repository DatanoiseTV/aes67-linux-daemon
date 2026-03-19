// Package session provides SDP parsing and generation for AES67 streams.
package session

import (
	"fmt"
	"strconv"
	"strings"
)

// SDPInfo holds parsed SDP information for an AES67 stream.
type SDPInfo struct {
	SessionName    string
	Origin         string
	ConnectionAddr string  // multicast IP
	TTL            int
	Port           int
	PayloadType    int
	Codec          string  // L16, L24, AM824, etc.
	SampleRate     int
	Channels       int
	PTPDomain      int
	PTPTraceable   bool
	Direction      string  // "sendonly", "recvonly"
	PTime          float64 // packet time in ms
}

// ParseSDP parses a standard SDP text (RFC 4566) with AES67-specific attributes
// and returns a populated SDPInfo.
func ParseSDP(text string) (*SDPInfo, error) {
	info := &SDPInfo{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	var versionSeen bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(line) < 2 || line[1] != '=' {
			continue
		}

		key := line[0]
		value := line[2:]

		switch key {
		case 'v':
			if value != "0" {
				return nil, fmt.Errorf("unsupported SDP version: %s", value)
			}
			versionSeen = true

		case 'o':
			info.Origin = value

		case 's':
			info.SessionName = value

		case 'c':
			if err := parseConnection(value, info); err != nil {
				return nil, fmt.Errorf("invalid connection line: %w", err)
			}

		case 'm':
			if err := parseMedia(value, info); err != nil {
				return nil, fmt.Errorf("invalid media line: %w", err)
			}

		case 'a':
			if err := parseAttribute(value, info); err != nil {
				return nil, fmt.Errorf("invalid attribute: %w", err)
			}
		}
	}

	if !versionSeen {
		return nil, fmt.Errorf("missing SDP version line (v=)")
	}

	return info, nil
}

// parseConnection parses "IN IP4 <addr>/<ttl>" from the c= line.
func parseConnection(value string, info *SDPInfo) error {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return fmt.Errorf("expected at least 3 fields, got %d", len(parts))
	}
	if parts[0] != "IN" || parts[1] != "IP4" {
		return fmt.Errorf("unsupported network/address type: %s %s", parts[0], parts[1])
	}

	addrPart := parts[2]
	if idx := strings.Index(addrPart, "/"); idx >= 0 {
		info.ConnectionAddr = addrPart[:idx]
		ttl, err := strconv.Atoi(addrPart[idx+1:])
		if err != nil {
			return fmt.Errorf("invalid TTL: %w", err)
		}
		info.TTL = ttl
	} else {
		info.ConnectionAddr = addrPart
	}

	return nil
}

// parseMedia parses "audio <port> RTP/AVP <payload-type>" from the m= line.
func parseMedia(value string, info *SDPInfo) error {
	parts := strings.Fields(value)
	if len(parts) < 4 {
		return fmt.Errorf("expected at least 4 fields, got %d", len(parts))
	}
	if parts[0] != "audio" {
		return fmt.Errorf("unsupported media type: %s", parts[0])
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	info.Port = port

	pt, err := strconv.Atoi(parts[3])
	if err != nil {
		return fmt.Errorf("invalid payload type: %w", err)
	}
	info.PayloadType = pt

	return nil
}

// parseAttribute parses a= lines including rtpmap, ptime, clock-domain, etc.
func parseAttribute(value string, info *SDPInfo) error {
	switch {
	case value == "sendonly":
		info.Direction = "sendonly"

	case value == "recvonly":
		info.Direction = "recvonly"

	case strings.HasPrefix(value, "rtpmap:"):
		return parseRTPMap(value[7:], info)

	case strings.HasPrefix(value, "ptime:"):
		pt, err := strconv.ParseFloat(value[6:], 64)
		if err != nil {
			return fmt.Errorf("invalid ptime: %w", err)
		}
		info.PTime = pt

	case strings.HasPrefix(value, "clock-domain:PTPv2 "):
		domain, err := strconv.Atoi(value[19:])
		if err != nil {
			return fmt.Errorf("invalid PTP domain: %w", err)
		}
		info.PTPDomain = domain

	case strings.HasPrefix(value, "ts-refclk:ptp="):
		refVal := value[14:]
		if strings.Contains(refVal, "traceable") {
			info.PTPTraceable = true
		}
	}

	return nil
}

// parseRTPMap parses "<pt> <codec>/<rate>/<channels>" from an rtpmap attribute.
func parseRTPMap(value string, info *SDPInfo) error {
	// value is like "98 L24/48000/2"
	parts := strings.SplitN(value, " ", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid rtpmap format")
	}

	codecParts := strings.Split(parts[1], "/")
	if len(codecParts) < 2 {
		return fmt.Errorf("invalid codec format: %s", parts[1])
	}

	info.Codec = codecParts[0]

	rate, err := strconv.Atoi(codecParts[1])
	if err != nil {
		return fmt.Errorf("invalid sample rate: %w", err)
	}
	info.SampleRate = rate

	if len(codecParts) >= 3 {
		ch, err := strconv.Atoi(codecParts[2])
		if err != nil {
			return fmt.Errorf("invalid channels: %w", err)
		}
		info.Channels = ch
	} else {
		info.Channels = 1
	}

	return nil
}

// GenerateSDP produces a valid AES67 SDP announcement string from an SDPInfo.
func GenerateSDP(info SDPInfo) string {
	// Default direction
	direction := info.Direction
	if direction == "" {
		direction = "sendonly"
	}

	// Default TTL
	ttl := info.TTL
	if ttl == 0 {
		ttl = 32
	}

	// Extract source IP from Origin or use a default
	sourceIP := "0.0.0.0"
	originLine := info.Origin
	if originLine == "" {
		originLine = fmt.Sprintf("- 0 0 IN IP4 %s", sourceIP)
	}

	// PTP reference clock line
	ptpRef := "a=ts-refclk:ptp=traceable"
	if !info.PTPTraceable {
		ptpRef = fmt.Sprintf("a=ts-refclk:ptp=IEEE802.1AS-2011:00-00-00-00-00-00-00-00")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "v=0\n")
	fmt.Fprintf(&b, "o=%s\n", originLine)
	fmt.Fprintf(&b, "s=%s\n", info.SessionName)
	fmt.Fprintf(&b, "c=IN IP4 %s/%d\n", info.ConnectionAddr, ttl)
	fmt.Fprintf(&b, "t=0 0\n")
	fmt.Fprintf(&b, "m=audio %d RTP/AVP %d\n", info.Port, info.PayloadType)
	fmt.Fprintf(&b, "a=rtpmap:%d %s/%d/%d\n", info.PayloadType, info.Codec, info.SampleRate, info.Channels)
	fmt.Fprintf(&b, "a=ptime:%s\n", formatPTime(info.PTime))
	fmt.Fprintf(&b, "a=clock-domain:PTPv2 %d\n", info.PTPDomain)
	fmt.Fprintf(&b, "%s\n", ptpRef)
	fmt.Fprintf(&b, "a=%s\n", direction)

	return b.String()
}

// formatPTime formats packet time, using integer format when possible.
func formatPTime(ptime float64) string {
	if ptime == float64(int(ptime)) {
		return strconv.Itoa(int(ptime))
	}
	return strconv.FormatFloat(ptime, 'f', -1, 64)
}
