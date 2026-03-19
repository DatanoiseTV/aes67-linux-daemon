// Package netlink provides raw socket communication with the RAVENNA kernel module
// using custom netlink protocols.
package netlink

import (
	"encoding/binary"
	"fmt"
)

// Message IDs for communication with the RAVENNA kernel module.
const (
	MsgStart             int32 = 0
	MsgStop              int32 = 1
	MsgReset             int32 = 2
	MsgStartIO           int32 = 3
	MsgStopIO            int32 = 4
	MsgSetSampleRate     int32 = 5
	MsgGetSampleRate     int32 = 6
	MsgGetAudioMode      int32 = 7
	MsgSetDSDAudioMode   int32 = 8
	MsgSetTICFrameSize   int32 = 9
	MsgSetMaxTICFrameSize int32 = 10
	MsgSetNumInputs      int32 = 11
	MsgSetNumOutputs     int32 = 12
	MsgGetNumInputs      int32 = 13
	MsgGetNumOutputs     int32 = 14
	MsgSetInterfaceName  int32 = 15
	MsgAddRTPStream      int32 = 16
	MsgRemoveRTPStream   int32 = 17
	MsgUpdateStreamName  int32 = 18
	MsgGetPTPInfo        int32 = 19
	MsgHello             int32 = 20
	MsgBye               int32 = 21
	MsgPing              int32 = 22
	MsgSetMasterVolume   int32 = 23
	MsgSetMasterSwitch   int32 = 24
	MsgGetMasterVolume   int32 = 25
	MsgGetMasterSwitch   int32 = 26
	MsgGetStreamStatus   int32 = 29
	MsgSetPlayoutDelay   int32 = 30
	MsgSetPTPConfig      int32 = 31
	MsgGetPTPConfig      int32 = 32
	MsgGetPTPStatus      int32 = 33
)

// Netlink message constants.
const (
	NlmsgDone = 0x3 // NLMSG_DONE
)

// NlmsghdrSize is the size of the netlink message header.
const NlmsghdrSize = 16

// MTALSAMsgSize is the size of the MT_ALSA_msg header.
const MTALSAMsgSize = 12

// MaxPayload is the maximum payload size for netlink messages.
const MaxPayload = 1024

// Nlmsghdr represents the netlink message header (16 bytes).
type Nlmsghdr struct {
	Len   uint32
	Type  uint16
	Flags uint16
	Seq   uint32
	Pid   uint32
}

// MTALSAMsg represents the MT_ALSA message header (12 bytes).
type MTALSAMsg struct {
	ID       int32
	ErrCode  int32
	DataSize int32
}

// MarshalNlmsghdr serializes a Nlmsghdr to bytes.
func MarshalNlmsghdr(hdr *Nlmsghdr) []byte {
	buf := make([]byte, NlmsghdrSize)
	binary.LittleEndian.PutUint32(buf[0:4], hdr.Len)
	binary.LittleEndian.PutUint16(buf[4:6], hdr.Type)
	binary.LittleEndian.PutUint16(buf[6:8], hdr.Flags)
	binary.LittleEndian.PutUint32(buf[8:12], hdr.Seq)
	binary.LittleEndian.PutUint32(buf[12:16], hdr.Pid)
	return buf
}

// UnmarshalNlmsghdr deserializes a Nlmsghdr from bytes.
func UnmarshalNlmsghdr(data []byte) (*Nlmsghdr, error) {
	if len(data) < NlmsghdrSize {
		return nil, fmt.Errorf("data too short for nlmsghdr: need %d, got %d", NlmsghdrSize, len(data))
	}
	return &Nlmsghdr{
		Len:   binary.LittleEndian.Uint32(data[0:4]),
		Type:  binary.LittleEndian.Uint16(data[4:6]),
		Flags: binary.LittleEndian.Uint16(data[6:8]),
		Seq:   binary.LittleEndian.Uint32(data[8:12]),
		Pid:   binary.LittleEndian.Uint32(data[12:16]),
	}, nil
}

// MarshalMTALSAMsg serializes an MTALSAMsg to bytes.
func MarshalMTALSAMsg(msg *MTALSAMsg) []byte {
	buf := make([]byte, MTALSAMsgSize)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(msg.ID))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(msg.ErrCode))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(msg.DataSize))
	return buf
}

// UnmarshalMTALSAMsg deserializes an MTALSAMsg from bytes.
func UnmarshalMTALSAMsg(data []byte) (*MTALSAMsg, error) {
	if len(data) < MTALSAMsgSize {
		return nil, fmt.Errorf("data too short for MT_ALSA_msg: need %d, got %d", MTALSAMsgSize, len(data))
	}
	return &MTALSAMsg{
		ID:       int32(binary.LittleEndian.Uint32(data[0:4])),
		ErrCode:  int32(binary.LittleEndian.Uint32(data[4:8])),
		DataSize: int32(binary.LittleEndian.Uint32(data[8:12])),
	}, nil
}

// BuildMessage constructs a complete netlink message with nlmsghdr + MT_ALSA_msg + payload.
func BuildMessage(msgID int32, pid uint32, payload []byte) []byte {
	dataSize := int32(0)
	if payload != nil {
		dataSize = int32(len(payload))
	}

	totalLen := uint32(NlmsghdrSize + MTALSAMsgSize + int(dataSize))

	nlHdr := &Nlmsghdr{
		Len:  totalLen,
		Type: NlmsgDone,
		Pid:  pid,
	}

	alsaMsg := &MTALSAMsg{
		ID:       msgID,
		DataSize: dataSize,
	}

	buf := make([]byte, 0, totalLen)
	buf = append(buf, MarshalNlmsghdr(nlHdr)...)
	buf = append(buf, MarshalMTALSAMsg(alsaMsg)...)
	if payload != nil {
		buf = append(buf, payload...)
	}
	return buf
}

// ParseMessage parses a complete netlink message, returning the nlmsghdr, MT_ALSA_msg, and payload.
func ParseMessage(data []byte) (*Nlmsghdr, *MTALSAMsg, []byte, error) {
	if len(data) < NlmsghdrSize+MTALSAMsgSize {
		return nil, nil, nil, fmt.Errorf("message too short: need at least %d bytes, got %d",
			NlmsghdrSize+MTALSAMsgSize, len(data))
	}

	nlHdr, err := UnmarshalNlmsghdr(data[:NlmsghdrSize])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing nlmsghdr: %w", err)
	}

	alsaMsg, err := UnmarshalMTALSAMsg(data[NlmsghdrSize : NlmsghdrSize+MTALSAMsgSize])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing MT_ALSA_msg: %w", err)
	}

	var payload []byte
	if alsaMsg.DataSize > 0 {
		payloadStart := NlmsghdrSize + MTALSAMsgSize
		payloadEnd := payloadStart + int(alsaMsg.DataSize)
		if payloadEnd > len(data) {
			return nil, nil, nil, fmt.Errorf("payload extends beyond message: need %d bytes, have %d",
				payloadEnd, len(data))
		}
		payload = data[payloadStart:payloadEnd]
	}

	return nlHdr, alsaMsg, payload, nil
}
