// Package discovery implements SAP (RFC 2974) and mDNS/DNS-SD source discovery
// for AES67 and RAVENNA audio streams.
package discovery

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// SAPMulticastAddr is the SAP announcement multicast group.
	SAPMulticastAddr = "239.255.255.255"
	// SAPPort is the SAP announcement port.
	SAPPort = 9875
	// SAPVersion is the SAP protocol version we support.
	SAPVersion = 1
	// SAPMIMEType is the content type for SDP payloads in SAP.
	SAPMIMEType = "application/sdp"

	// defaultExpireMultiplier defines how many missed announce periods
	// before a source is considered expired.
	defaultExpireMultiplier = 3
	// defaultAnnouncePeriod is the fallback announce period in seconds
	// when not explicitly known.
	defaultAnnouncePeriod = 300
)

// RemoteSource represents a discovered AES67/RAVENNA audio source.
type RemoteSource struct {
	Source         string    // discovery method: "SAP" or "mDNS"
	ID             string    // unique identifier (SAP message hash or mDNS instance)
	Name           string    // from SDP s= line or mDNS instance name
	Domain         string    // from SDP a=clock-domain or empty
	Address        string    // from SDP c= line or mDNS SRV target
	SDP            string    // raw SDP text (SAP only)
	LastSeen       time.Time // when we last received an announcement
	AnnouncePeriod int       // seconds between announcements
}

// SAPListener listens for SAP announcements on the multicast group and
// maintains a thread-safe map of discovered remote sources.
type SAPListener struct {
	sources map[string]*RemoteSource
	mu      sync.RWMutex
	iface   string
	conn    *net.UDPConn
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewSAPListener creates a new SAP listener bound to the given network interface.
// If iface is empty, the system default interface is used.
func NewSAPListener(iface string) *SAPListener {
	return &SAPListener{
		sources: make(map[string]*RemoteSource),
		iface:   iface,
		done:    make(chan struct{}),
	}
}

// Start joins the SAP multicast group and begins receiving announcements.
// It blocks until the context is cancelled or Stop is called.
func (s *SAPListener) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	addr := &net.UDPAddr{
		IP:   net.ParseIP(SAPMulticastAddr),
		Port: SAPPort,
	}

	var ifi *net.Interface
	if s.iface != "" {
		var err error
		ifi, err = net.InterfaceByName(s.iface)
		if err != nil {
			return fmt.Errorf("sap: interface %q: %w", s.iface, err)
		}
	}

	conn, err := net.ListenMulticastUDP("udp4", ifi, addr)
	if err != nil {
		return fmt.Errorf("sap: listen multicast: %w", err)
	}
	s.conn = conn

	// Set a reasonable read buffer size.
	_ = conn.SetReadBuffer(65536)

	go s.receiveLoop(ctx)
	go s.expiryLoop(ctx)

	return nil
}

// Stop shuts down the SAP listener and closes the multicast connection.
func (s *SAPListener) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	<-s.done
}

// GetSources returns a thread-safe snapshot of all currently known remote sources.
func (s *SAPListener) GetSources() []RemoteSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RemoteSource, 0, len(s.sources))
	for _, src := range s.sources {
		result = append(result, *src)
	}
	return result
}

func (s *SAPListener) receiveLoop(ctx context.Context) {
	defer close(s.done)

	buf := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		if n < 8 {
			continue
		}

		s.handlePacket(buf[:n])
	}
}

func (s *SAPListener) expiryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.expireSources()
		}
	}
}

func (s *SAPListener) expireSources() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, src := range s.sources {
		period := src.AnnouncePeriod
		if period <= 0 {
			period = defaultAnnouncePeriod
		}
		expiry := src.LastSeen.Add(time.Duration(period*defaultExpireMultiplier) * time.Second)
		if now.After(expiry) {
			delete(s.sources, id)
		}
	}
}

// handlePacket parses a raw SAP packet and updates the source map.
func (s *SAPListener) handlePacket(data []byte) {
	pkt, err := ParseSAPPacket(data)
	if err != nil {
		return
	}

	id := fmt.Sprintf("%04x", pkt.MsgIDHash)

	if pkt.IsDeletion {
		s.mu.Lock()
		delete(s.sources, id)
		s.mu.Unlock()
		return
	}

	name := extractSDPField(pkt.SDP, "s=")
	address := extractSDPConnectionAddress(pkt.SDP)
	domain := extractSDPAttribute(pkt.SDP, "clock-domain")

	s.mu.Lock()
	s.sources[id] = &RemoteSource{
		Source:         "SAP",
		ID:             id,
		Name:           name,
		Domain:         domain,
		Address:        address,
		SDP:            pkt.SDP,
		LastSeen:       time.Now(),
		AnnouncePeriod: defaultAnnouncePeriod,
	}
	s.mu.Unlock()
}

// SAPPacket represents a parsed SAP (RFC 2974) packet.
type SAPPacket struct {
	Version    uint8
	IsDeletion bool
	MsgIDHash  uint16
	OriginIP   net.IP
	AuthLen    uint8
	SDP        string
}

// ParseSAPPacket parses raw bytes into a SAPPacket.
// SAP header layout (RFC 2974):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| V=1 |A|R|T|E|C|    auth len  |         msg id hash           |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                      originating source                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func ParseSAPPacket(data []byte) (*SAPPacket, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("sap: packet too short (%d bytes)", len(data))
	}

	flags := data[0]
	version := (flags >> 5) & 0x07
	if version != SAPVersion {
		return nil, fmt.Errorf("sap: unsupported version %d", version)
	}

	isDeletion := (flags & 0x04) != 0 // A bit (bit 2 of first byte, after version)
	// T bit indicates IPv6 originating source (bit 4, 0x10 mask after version).
	isIPv6 := (flags & 0x10) != 0

	authLen := data[1]
	msgIDHash := binary.BigEndian.Uint16(data[2:4])

	originSize := 4
	if isIPv6 {
		originSize = 16
	}

	headerEnd := 4 + originSize + int(authLen)*4
	if len(data) < headerEnd {
		return nil, fmt.Errorf("sap: packet too short for header")
	}

	originIP := make(net.IP, originSize)
	copy(originIP, data[4:4+originSize])

	payload := data[headerEnd:]

	// Skip optional MIME type; if present it's null-terminated.
	// Default is "application/sdp".
	sdpData := string(payload)
	if idx := strings.Index(sdpData, "\x00"); idx >= 0 {
		mimeType := sdpData[:idx]
		_ = mimeType // we only handle application/sdp
		sdpData = sdpData[idx+1:]
	}

	return &SAPPacket{
		Version:    version,
		IsDeletion: isDeletion,
		MsgIDHash:  msgIDHash,
		OriginIP:   originIP,
		AuthLen:    authLen,
		SDP:        sdpData,
	}, nil
}

// BuildSAPPacket constructs a SAP announcement packet for the given SDP and source IP.
func BuildSAPPacket(sdp string, srcIP net.IP, msgIDHash uint16, isDeletion bool) []byte {
	srcIP = srcIP.To4()
	if srcIP == nil {
		return nil
	}

	// flags: version=1, A bit for deletion
	flags := byte(SAPVersion << 5)
	if isDeletion {
		flags |= 0x04
	}

	header := make([]byte, 8)
	header[0] = flags
	header[1] = 0 // auth len
	binary.BigEndian.PutUint16(header[2:4], msgIDHash)
	copy(header[4:8], srcIP)

	mime := []byte(SAPMIMEType + "\x00")
	pkt := make([]byte, 0, len(header)+len(mime)+len(sdp))
	pkt = append(pkt, header...)
	pkt = append(pkt, mime...)
	pkt = append(pkt, []byte(sdp)...)
	return pkt
}

// SAPAnnouncer periodically sends SAP announcements for local sources.
type SAPAnnouncer struct {
	iface    string
	interval int // seconds between announcements
	conn     *net.UDPConn
	cancel   context.CancelFunc
	done     chan struct{}

	mu      sync.Mutex
	entries []sapEntry
}

type sapEntry struct {
	sdp       string
	srcIP     net.IP
	msgIDHash uint16
}

// NewSAPAnnouncer creates a new SAP announcer for the given interface and interval.
func NewSAPAnnouncer(iface string, interval int) *SAPAnnouncer {
	if interval <= 0 {
		interval = defaultAnnouncePeriod
	}
	return &SAPAnnouncer{
		iface:    iface,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Announce registers an SDP for periodic SAP announcement.
func (a *SAPAnnouncer) Announce(sdp string, srcIP net.IP) error {
	if srcIP.To4() == nil {
		return fmt.Errorf("sap: only IPv4 sources supported")
	}

	// Simple hash from source IP and SDP content.
	hash := sapHash(srcIP, sdp)

	a.mu.Lock()
	a.entries = append(a.entries, sapEntry{
		sdp:       sdp,
		srcIP:     srcIP,
		msgIDHash: hash,
	})
	a.mu.Unlock()
	return nil
}

// Start begins sending SAP announcements at the configured interval.
func (a *SAPAnnouncer) Start(ctx context.Context) error {
	ctx, a.cancel = context.WithCancel(ctx)

	dst := &net.UDPAddr{
		IP:   net.ParseIP(SAPMulticastAddr),
		Port: SAPPort,
	}

	var laddr *net.UDPAddr
	if a.iface != "" {
		ifi, err := net.InterfaceByName(a.iface)
		if err != nil {
			return fmt.Errorf("sap announcer: interface %q: %w", a.iface, err)
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			return fmt.Errorf("sap announcer: interface addrs: %w", err)
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				laddr = &net.UDPAddr{IP: ipnet.IP}
				break
			}
		}
	}

	conn, err := net.DialUDP("udp4", laddr, dst)
	if err != nil {
		return fmt.Errorf("sap announcer: dial: %w", err)
	}
	a.conn = conn

	go a.sendLoop(ctx)
	return nil
}

// Stop halts SAP announcements.
func (a *SAPAnnouncer) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.conn != nil {
		a.conn.Close()
	}
	<-a.done
}

func (a *SAPAnnouncer) sendLoop(ctx context.Context) {
	defer close(a.done)

	// Send immediately, then at interval.
	a.sendAll()

	ticker := time.NewTicker(time.Duration(a.interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sendAll()
		}
	}
}

func (a *SAPAnnouncer) sendAll() {
	a.mu.Lock()
	entries := make([]sapEntry, len(a.entries))
	copy(entries, a.entries)
	a.mu.Unlock()

	for _, e := range entries {
		pkt := BuildSAPPacket(e.sdp, e.srcIP, e.msgIDHash, false)
		if pkt != nil && a.conn != nil {
			_, _ = a.conn.Write(pkt)
		}
	}
}

// sapHash computes a simple 16-bit hash for a SAP message ID from source IP and SDP.
func sapHash(ip net.IP, sdp string) uint16 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	h := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
	for _, b := range []byte(sdp) {
		h = h*31 + uint32(b)
	}
	return uint16(h ^ (h >> 16))
}

// extractSDPField extracts the value of a single-character SDP field (e.g., "s=").
func extractSDPField(sdp, prefix string) string {
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

// extractSDPConnectionAddress extracts the address from an SDP c= line.
// Format: c=IN IP4 <address>[/<ttl>[/<count>]]
func extractSDPConnectionAddress(sdp string) string {
	cLine := extractSDPField(sdp, "c=")
	if cLine == "" {
		return ""
	}
	parts := strings.Fields(cLine)
	if len(parts) >= 3 {
		addr := parts[2]
		// Strip TTL and count suffix.
		if idx := strings.Index(addr, "/"); idx >= 0 {
			addr = addr[:idx]
		}
		return addr
	}
	return ""
}

// extractSDPAttribute extracts the value of an SDP attribute (a=<attr>:<value>).
func extractSDPAttribute(sdp, attr string) string {
	prefix := "a=" + attr + ":"
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}
