package netlink

import (
	"testing"
)

func TestNlmsghdrMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &Nlmsghdr{
		Len:   100,
		Type:  NlmsgDone,
		Flags: 0,
		Seq:   42,
		Pid:   1234,
	}

	data := MarshalNlmsghdr(orig)
	if len(data) != NlmsghdrSize {
		t.Fatalf("size: got %d, want %d", len(data), NlmsghdrSize)
	}

	decoded, err := UnmarshalNlmsghdr(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if *decoded != *orig {
		t.Errorf("got %+v, want %+v", decoded, orig)
	}
}

func TestUnmarshalNlmsghdrTooShort(t *testing.T) {
	_, err := UnmarshalNlmsghdr([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestMTALSAMsgMarshalUnmarshalRoundtrip(t *testing.T) {
	orig := &MTALSAMsg{
		ID:       MsgAddRTPStream,
		ErrCode:  -1,
		DataSize: 308,
	}

	data := MarshalMTALSAMsg(orig)
	if len(data) != MTALSAMsgSize {
		t.Fatalf("size: got %d, want %d", len(data), MTALSAMsgSize)
	}

	decoded, err := UnmarshalMTALSAMsg(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if *decoded != *orig {
		t.Errorf("got %+v, want %+v", decoded, orig)
	}
}

func TestUnmarshalMTALSAMsgTooShort(t *testing.T) {
	_, err := UnmarshalMTALSAMsg([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestBuildAndParseMessage(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	pid := uint32(9999)

	msg := BuildMessage(MsgPing, pid, payload)

	expectedLen := NlmsghdrSize + MTALSAMsgSize + len(payload)
	if len(msg) != expectedLen {
		t.Fatalf("message length: got %d, want %d", len(msg), expectedLen)
	}

	nlHdr, alsaMsg, respPayload, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if nlHdr.Len != uint32(expectedLen) {
		t.Errorf("nlHdr.Len: got %d, want %d", nlHdr.Len, expectedLen)
	}
	if nlHdr.Type != NlmsgDone {
		t.Errorf("nlHdr.Type: got %d, want %d", nlHdr.Type, NlmsgDone)
	}
	if nlHdr.Pid != pid {
		t.Errorf("nlHdr.Pid: got %d, want %d", nlHdr.Pid, pid)
	}
	if alsaMsg.ID != MsgPing {
		t.Errorf("alsaMsg.ID: got %d, want %d", alsaMsg.ID, MsgPing)
	}
	if alsaMsg.ErrCode != 0 {
		t.Errorf("alsaMsg.ErrCode: got %d, want 0", alsaMsg.ErrCode)
	}
	if alsaMsg.DataSize != int32(len(payload)) {
		t.Errorf("alsaMsg.DataSize: got %d, want %d", alsaMsg.DataSize, len(payload))
	}
	if len(respPayload) != len(payload) {
		t.Fatalf("payload length: got %d, want %d", len(respPayload), len(payload))
	}
	for i, b := range respPayload {
		if b != payload[i] {
			t.Errorf("payload[%d]: got %#x, want %#x", i, b, payload[i])
		}
	}
}

func TestBuildAndParseMessageNoPayload(t *testing.T) {
	msg := BuildMessage(MsgHello, 1000, nil)

	expectedLen := NlmsghdrSize + MTALSAMsgSize
	if len(msg) != expectedLen {
		t.Fatalf("message length: got %d, want %d", len(msg), expectedLen)
	}

	_, alsaMsg, payload, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if alsaMsg.ID != MsgHello {
		t.Errorf("ID: got %d, want %d", alsaMsg.ID, MsgHello)
	}
	if alsaMsg.DataSize != 0 {
		t.Errorf("DataSize: got %d, want 0", alsaMsg.DataSize)
	}
	if payload != nil {
		t.Errorf("payload should be nil, got %v", payload)
	}
}

func TestParseMessageTooShort(t *testing.T) {
	_, _, _, err := ParseMessage([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short message")
	}
}

func TestParseMessagePayloadTruncated(t *testing.T) {
	// Build a valid message then truncate it
	msg := BuildMessage(MsgPing, 100, []byte{1, 2, 3, 4})
	truncated := msg[:len(msg)-2] // chop off 2 bytes of payload

	_, _, _, err := ParseMessage(truncated)
	if err == nil {
		t.Error("expected error for truncated payload")
	}
}

func TestMessageConstants(t *testing.T) {
	// Verify key message ID values match expected protocol
	tests := []struct {
		name string
		got  int32
		want int32
	}{
		{"MsgStart", MsgStart, 0},
		{"MsgStop", MsgStop, 1},
		{"MsgAddRTPStream", MsgAddRTPStream, 16},
		{"MsgRemoveRTPStream", MsgRemoveRTPStream, 17},
		{"MsgGetPTPStatus", MsgGetPTPStatus, 33},
		{"MsgGetStreamStatus", MsgGetStreamStatus, 29},
		{"MsgSetPTPConfig", MsgSetPTPConfig, 31},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}
