package session

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/netlink"
)

// AddSink registers a new RTP sink stream with the kernel driver.
func (m *Manager) AddSink(sink StreamSink) error {
	if sink.ID > maxStreamID {
		return fmt.Errorf("sink ID %d out of range (0-%d)", sink.ID, maxStreamID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sinks[sink.ID]; exists {
		return fmt.Errorf("sink %d already exists", sink.ID)
	}

	handle, err := m.addSinkStream(&sink)
	if err != nil {
		return err
	}

	stored := sink // copy
	m.sinks[sink.ID] = &streamInfo{
		handle: handle,
		sink:   &stored,
	}

	return m.saveStatusLocked()
}

// RemoveSink removes a sink stream from the driver and internal state.
func (m *Manager) RemoveSink(id uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	si, ok := m.sinks[id]
	if !ok {
		return fmt.Errorf("sink %d not found", id)
	}

	if err := m.driver.RemoveRTPStream(si.handle); err != nil {
		return fmt.Errorf("remove sink stream: %w", err)
	}

	delete(m.sinks, id)
	return m.saveStatusLocked()
}

// addSinkStream builds an RTPStreamInfo from the sink and sends it to the driver.
// Caller must hold m.mu.
func (m *Manager) addSinkStream(sink *StreamSink) (uint64, error) {
	info, err := m.buildSinkInfo(sink)
	if err != nil {
		return 0, fmt.Errorf("build sink info: %w", err)
	}
	handle, err := m.driver.AddRTPStream(info)
	if err != nil {
		return 0, fmt.Errorf("add sink stream: %w", err)
	}
	return handle, nil
}

// buildSinkInfo converts a StreamSink into a kernel RTPStreamInfo.
func (m *Manager) buildSinkInfo(sink *StreamSink) (*netlink.RTPStreamInfo, error) {
	info := &netlink.RTPStreamInfo{}
	info.SizeOf = uint32(netlink.RTPStreamInfoSize)
	info.IsSource = 0

	// Name
	copy(info.Name[:], sink.Name)

	// Stream ID
	info.Id = uint32(sink.ID)

	// Playout delay
	info.PlayOutDelay = sink.Delay
	if info.PlayOutDelay == 0 && m.config.PlayoutDelay > 0 {
		info.PlayOutDelay = uint32(m.config.PlayoutDelay)
	}

	// Determine stream parameters from SDP or defaults.
	if sink.UseSDP && sink.SDP != "" {
		sdpInfo, err := ParseSDP(sink.SDP)
		if err != nil {
			return nil, fmt.Errorf("parse sink SDP: %w", err)
		}

		// Destination address from SDP
		destIP := net.ParseIP(sdpInfo.ConnectionAddr).To4()
		if destIP == nil {
			return nil, fmt.Errorf("invalid SDP connection address: %s", sdpInfo.ConnectionAddr)
		}
		info.DestIP = binary.LittleEndian.Uint32(destIP)
		info.DestMAC = multicastMAC(destIP)

		// Port
		info.DestPort = uint16(sdpInfo.Port)
		info.SrcPort = uint16(sdpInfo.Port)

		// Codec
		codec := sdpInfo.Codec
		if codec == "" {
			codec = "L24"
		}
		copy(info.Codec[:], codec)
		info.WordLength = codecWordLength(codec)

		// Sampling rate
		info.SamplingRate = uint32(sdpInfo.SampleRate)
		if info.SamplingRate == 0 {
			info.SamplingRate = uint32(m.config.SampleRate)
		}

		// Channels
		info.NbOfChannels = uint8(sdpInfo.Channels)

		// Payload type
		info.PayloadType = uint8(sdpInfo.PayloadType)

		// TTL
		info.TTL = uint8(sdpInfo.TTL)

		// Frame size from ptime
		if sdpInfo.PTime > 0 {
			info.FrameSize = uint32(float64(sdpInfo.SampleRate) * sdpInfo.PTime / 1000.0)
			info.MaxSamplesPerPkt = info.FrameSize
		}
	} else {
		// Non-SDP sink: use config defaults.
		info.SamplingRate = uint32(m.config.SampleRate)
		info.DestPort = uint16(m.config.RTPPort)
		info.SrcPort = uint16(m.config.RTPPort)

		codec := "L24"
		copy(info.Codec[:], codec)
		info.WordLength = codecWordLength(codec)

		info.NbOfChannels = uint8(len(sink.Map))
		info.PayloadType = 98
		info.TTL = 15
	}

	// Source IP from config
	if m.config.IPAddr != "" {
		srcIP := net.ParseIP(m.config.IPAddr).To4()
		if srcIP != nil {
			info.SrcIP = binary.LittleEndian.Uint32(srcIP)
		}
	}

	// Routing: map[streamCh] = physicalCh
	for i := range info.Routing {
		info.Routing[i] = 0xFFFFFFFF // unused
	}
	for streamCh, physCh := range sink.Map {
		if streamCh < 64 && physCh >= 0 {
			info.Routing[streamCh] = uint32(physCh)
		}
	}

	return info, nil
}
