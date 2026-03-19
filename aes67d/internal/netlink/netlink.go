package netlink

import (
	"sync"
)

// Netlink protocol constants for RAVENNA communication.
const (
	ProtocolU2K = 31 // User-to-kernel commands
	ProtocolK2U = 29 // Kernel-to-user events
)

// MaxPayloadSize is the maximum payload size for netlink messages.
const MaxPayloadSize = MaxPayload

// recvBufSize is the receive buffer size for netlink messages.
const recvBufSize = NlmsghdrSize + MTALSAMsgSize + MaxPayload

// Conn represents a raw netlink socket connection.
type Conn struct {
	fd  int
	pid uint32
	mu  sync.Mutex
}

// Close closes the netlink socket.
func (c *Conn) Close() error {
	return closeSocket(c.fd)
}

// SendCommand sends a command to the kernel module and waits for a reply.
// Returns the error code from the reply, the response payload, and any transport error.
func (c *Conn) SendCommand(msgID int32, payload []byte) (errCode int32, response []byte, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendCommand(msgID, payload)
}

// Listen listens for messages from the kernel module and dispatches them to the handler.
// The handler receives the message ID and payload, and returns a response payload (or nil).
// Listen blocks until the connection is closed or an error occurs.
func (c *Conn) Listen(handler func(msgID int32, payload []byte) []byte) error {
	return c.listen(handler)
}
