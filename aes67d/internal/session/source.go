package session

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/netlink"
)

// maxStreamID is the maximum allowed stream ID (0-63).
const maxStreamID = 63

// AddSource registers a new RTP source stream with the kernel driver.
func (m *Manager) AddSource(src StreamSource) error {
	if src.ID > maxStreamID {
		return fmt.Errorf("source ID %d out of range (0-%d)", src.ID, maxStreamID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sources[src.ID]; exists {
		return fmt.Errorf("source %d already exists", src.ID)
	}

	info, err := m.buildSourceInfo(&src)
	if err != nil {
		return fmt.Errorf("build source info: %w", err)
	}

	handle, err := m.driver.AddRTPStream(info)
	if err != nil {
		return fmt.Errorf("add source stream: %w", err)
	}

	stored := src // copy
	m.sources[src.ID] = &streamInfo{
		handle: handle,
		source: &stored,
	}

	return m.saveStatusLocked()
}

// RemoveSource removes a source stream from the driver and internal state.
func (m *Manager) RemoveSource(id uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	si, ok := m.sources[id]
	if !ok {
		return fmt.Errorf("source %d not found", id)
	}

	if err := m.driver.RemoveRTPStream(si.handle); err != nil {
		return fmt.Errorf("remove source stream: %w", err)
	}

	delete(m.sources, id)
	return m.saveStatusLocked()
}

// buildSourceInfo converts a StreamSource into a kernel RTPStreamInfo.
func (m *Manager) buildSourceInfo(src *StreamSource) (*netlink.RTPStreamInfo, error) {
	info := &netlink.RTPStreamInfo{}
	info.SizeOf = uint32(netlink.RTPStreamInfoSize)
	info.IsSource = 1

	// Name
	copy(info.Name[:], src.Name)

	// Codec
	codec := src.Codec
	if codec == "" {
		codec = "L24"
	}
	copy(info.Codec[:], codec)
	info.WordLength = codecWordLength(codec)

	// Sampling rate from config
	info.SamplingRate = uint32(m.config.SampleRate)

	// Channels
	info.NbOfChannels = uint8(len(src.Map))

	// Max samples per packet
	info.MaxSamplesPerPkt = src.MaxSamplesPerPacket
	if info.MaxSamplesPerPkt == 0 {
		info.MaxSamplesPerPkt = uint32(m.config.SampleRate / 1000) // 1ms default
	}

	// Frame size
	info.FrameSize = info.MaxSamplesPerPkt

	// Destination IP: parse RTPMcastBase and add source ID to last octet
	baseIP := net.ParseIP(m.config.RTPMcastBase).To4()
	if baseIP == nil {
		return nil, fmt.Errorf("invalid RTP multicast base: %s", m.config.RTPMcastBase)
	}
	destIP := make(net.IP, 4)
	copy(destIP, baseIP)
	destIP[3] = baseIP[3] + src.ID
	info.DestIP = binary.LittleEndian.Uint32(destIP)

	// Source IP from config
	if m.config.IPAddr != "" {
		srcIP := net.ParseIP(m.config.IPAddr).To4()
		if srcIP != nil {
			info.SrcIP = binary.LittleEndian.Uint32(srcIP)
		}
	}

	// Destination MAC from multicast IP (01:00:5E + lower 23 bits)
	info.DestMAC = multicastMAC(destIP)

	// TTL
	info.TTL = src.TTL
	if info.TTL == 0 {
		info.TTL = 15
	}

	// DSCP
	info.DSCP = src.DSCP

	// Payload type
	info.PayloadType = src.PayloadType
	if info.PayloadType == 0 {
		info.PayloadType = 98
	}

	// Ports
	info.SrcPort = uint16(m.config.RTPPort)
	info.DestPort = uint16(m.config.RTPPort)

	// SSRC: random
	info.SSRC = rand.Uint32()

	// Stream ID
	info.Id = uint32(src.ID)

	// Routing: map[streamCh] = physicalCh
	for i := range info.Routing {
		info.Routing[i] = 0xFFFFFFFF // unused
	}
	for streamCh, physCh := range src.Map {
		if streamCh < 64 && physCh >= 0 {
			info.Routing[streamCh] = uint32(physCh)
		}
	}

	return info, nil
}

// multicastMAC derives the Ethernet multicast MAC from an IPv4 multicast address.
// The mapping is: 01:00:5E + lower 23 bits of the IP address.
func multicastMAC(ip net.IP) [6]byte {
	ip4 := ip.To4()
	if ip4 == nil {
		return [6]byte{0x01, 0x00, 0x5E, 0x00, 0x00, 0x00}
	}
	return [6]byte{
		0x01, 0x00, 0x5E,
		ip4[1] & 0x7F, // lower 23 bits: clear top bit of second octet
		ip4[2],
		ip4[3],
	}
}
