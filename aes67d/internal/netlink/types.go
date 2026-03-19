package netlink

import (
	"encoding/binary"
	"fmt"
)

// RTPStreamInfo matches the C struct layout exactly (packed, no padding).
// Total size: 308 bytes.
type RTPStreamInfo struct {
	SizeOf           uint32
	Is8021Q          int8
	VLANId           uint16
	IfPortId         uint32
	Name             [64]byte
	PlayOutDelay     uint32
	FrameSize        uint32
	MaxSamplesPerPkt uint32
	DestMAC          [6]byte
	DSCP             uint8
	RTCPSrcIP        uint32
	SrcIP            uint32
	DestIP           uint32
	TTL              uint8
	SrcPort          uint16
	DestPort         uint16
	RTCPSrcPort      uint16
	RTCPDestPort     uint16
	PayloadType      uint8
	SSRC             uint32
	SSRCInitialized  int8
	RTPTimestampOff  uint32
	SamplingRate     uint32
	Codec            [10]byte
	WordLength       uint8
	NbOfChannels     uint8
	IsSource         int8
	Id               uint32
	Routing          [64]uint32
}

// RTPStreamInfoSize is the packed size of RTPStreamInfo.
const RTPStreamInfoSize = 4 + 1 + 2 + 4 + 64 + 4 + 4 + 4 + 6 + 1 + 4 + 4 + 4 + 1 + 2 + 2 + 2 + 2 + 1 + 4 + 1 + 4 + 4 + 10 + 1 + 1 + 1 + 4 + 64*4 // 392

// Marshal serializes RTPStreamInfo to bytes using packed layout (no padding).
func (r *RTPStreamInfo) Marshal() ([]byte, error) {
	buf := make([]byte, RTPStreamInfoSize)
	off := 0

	binary.LittleEndian.PutUint32(buf[off:], r.SizeOf)
	off += 4

	buf[off] = byte(r.Is8021Q)
	off += 1

	binary.LittleEndian.PutUint16(buf[off:], r.VLANId)
	off += 2

	binary.LittleEndian.PutUint32(buf[off:], r.IfPortId)
	off += 4

	copy(buf[off:off+64], r.Name[:])
	off += 64

	binary.LittleEndian.PutUint32(buf[off:], r.PlayOutDelay)
	off += 4

	binary.LittleEndian.PutUint32(buf[off:], r.FrameSize)
	off += 4

	binary.LittleEndian.PutUint32(buf[off:], r.MaxSamplesPerPkt)
	off += 4

	copy(buf[off:off+6], r.DestMAC[:])
	off += 6

	buf[off] = r.DSCP
	off += 1

	binary.LittleEndian.PutUint32(buf[off:], r.RTCPSrcIP)
	off += 4

	binary.LittleEndian.PutUint32(buf[off:], r.SrcIP)
	off += 4

	binary.LittleEndian.PutUint32(buf[off:], r.DestIP)
	off += 4

	buf[off] = r.TTL
	off += 1

	binary.LittleEndian.PutUint16(buf[off:], r.SrcPort)
	off += 2

	binary.LittleEndian.PutUint16(buf[off:], r.DestPort)
	off += 2

	binary.LittleEndian.PutUint16(buf[off:], r.RTCPSrcPort)
	off += 2

	binary.LittleEndian.PutUint16(buf[off:], r.RTCPDestPort)
	off += 2

	buf[off] = r.PayloadType
	off += 1

	binary.LittleEndian.PutUint32(buf[off:], r.SSRC)
	off += 4

	buf[off] = byte(r.SSRCInitialized)
	off += 1

	binary.LittleEndian.PutUint32(buf[off:], r.RTPTimestampOff)
	off += 4

	binary.LittleEndian.PutUint32(buf[off:], r.SamplingRate)
	off += 4

	copy(buf[off:off+10], r.Codec[:])
	off += 10

	buf[off] = r.WordLength
	off += 1

	buf[off] = r.NbOfChannels
	off += 1

	buf[off] = byte(r.IsSource)
	off += 1

	binary.LittleEndian.PutUint32(buf[off:], r.Id)
	off += 4

	for i := 0; i < 64; i++ {
		binary.LittleEndian.PutUint32(buf[off:], r.Routing[i])
		off += 4
	}

	return buf, nil
}

// Unmarshal deserializes RTPStreamInfo from bytes using packed layout.
func (r *RTPStreamInfo) Unmarshal(data []byte) error {
	if len(data) < RTPStreamInfoSize {
		return fmt.Errorf("data too short for RTPStreamInfo: need %d, got %d", RTPStreamInfoSize, len(data))
	}

	off := 0

	r.SizeOf = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.Is8021Q = int8(data[off])
	off += 1

	r.VLANId = binary.LittleEndian.Uint16(data[off:])
	off += 2

	r.IfPortId = binary.LittleEndian.Uint32(data[off:])
	off += 4

	copy(r.Name[:], data[off:off+64])
	off += 64

	r.PlayOutDelay = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.FrameSize = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.MaxSamplesPerPkt = binary.LittleEndian.Uint32(data[off:])
	off += 4

	copy(r.DestMAC[:], data[off:off+6])
	off += 6

	r.DSCP = data[off]
	off += 1

	r.RTCPSrcIP = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.SrcIP = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.DestIP = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.TTL = data[off]
	off += 1

	r.SrcPort = binary.LittleEndian.Uint16(data[off:])
	off += 2

	r.DestPort = binary.LittleEndian.Uint16(data[off:])
	off += 2

	r.RTCPSrcPort = binary.LittleEndian.Uint16(data[off:])
	off += 2

	r.RTCPDestPort = binary.LittleEndian.Uint16(data[off:])
	off += 2

	r.PayloadType = data[off]
	off += 1

	r.SSRC = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.SSRCInitialized = int8(data[off])
	off += 1

	r.RTPTimestampOff = binary.LittleEndian.Uint32(data[off:])
	off += 4

	r.SamplingRate = binary.LittleEndian.Uint32(data[off:])
	off += 4

	copy(r.Codec[:], data[off:off+10])
	off += 10

	r.WordLength = data[off]
	off += 1

	r.NbOfChannels = data[off]
	off += 1

	r.IsSource = int8(data[off])
	off += 1

	r.Id = binary.LittleEndian.Uint32(data[off:])
	off += 4

	for i := 0; i < 64; i++ {
		r.Routing[i] = binary.LittleEndian.Uint32(data[off:])
		off += 4
	}

	return nil
}

// PTPConfig matches the C struct for PTP configuration (packed, 2 bytes).
type PTPConfig struct {
	Domain uint8
	DSCP   uint8
}

// PTPConfigSize is the packed size of PTPConfig.
const PTPConfigSize = 2

// Marshal serializes PTPConfig to bytes.
func (p *PTPConfig) Marshal() ([]byte, error) {
	return []byte{p.Domain, p.DSCP}, nil
}

// Unmarshal deserializes PTPConfig from bytes.
func (p *PTPConfig) Unmarshal(data []byte) error {
	if len(data) < PTPConfigSize {
		return fmt.Errorf("data too short for PTPConfig: need %d, got %d", PTPConfigSize, len(data))
	}
	p.Domain = data[0]
	p.DSCP = data[1]
	return nil
}

// PTPStatus matches the C struct for PTP status (packed, 16 bytes).
type PTPStatus struct {
	LockStatus int32
	GMID       uint64
	Jitter     int32
}

// PTPStatusSize is the packed size of PTPStatus.
const PTPStatusSize = 16

// Marshal serializes PTPStatus to bytes.
func (p *PTPStatus) Marshal() ([]byte, error) {
	buf := make([]byte, PTPStatusSize)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(p.LockStatus))
	binary.LittleEndian.PutUint64(buf[4:12], p.GMID)
	binary.LittleEndian.PutUint32(buf[12:16], uint32(p.Jitter))
	return buf, nil
}

// Unmarshal deserializes PTPStatus from bytes.
func (p *PTPStatus) Unmarshal(data []byte) error {
	if len(data) < PTPStatusSize {
		return fmt.Errorf("data too short for PTPStatus: need %d, got %d", PTPStatusSize, len(data))
	}
	p.LockStatus = int32(binary.LittleEndian.Uint32(data[0:4]))
	p.GMID = binary.LittleEndian.Uint64(data[4:12])
	p.Jitter = int32(binary.LittleEndian.Uint32(data[12:16]))
	return nil
}

// StreamStatus matches the C struct for stream status (packed, 8 bytes).
type StreamStatus struct {
	Flags   uint32
	MinTime int32
}

// StreamStatusSize is the packed size of StreamStatus.
const StreamStatusSize = 8

// Marshal serializes StreamStatus to bytes.
func (s *StreamStatus) Marshal() ([]byte, error) {
	buf := make([]byte, StreamStatusSize)
	binary.LittleEndian.PutUint32(buf[0:4], s.Flags)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(s.MinTime))
	return buf, nil
}

// Unmarshal deserializes StreamStatus from bytes.
func (s *StreamStatus) Unmarshal(data []byte) error {
	if len(data) < StreamStatusSize {
		return fmt.Errorf("data too short for StreamStatus: need %d, got %d", StreamStatusSize, len(data))
	}
	s.Flags = binary.LittleEndian.Uint32(data[0:4])
	s.MinTime = int32(binary.LittleEndian.Uint32(data[4:8]))
	return nil
}
