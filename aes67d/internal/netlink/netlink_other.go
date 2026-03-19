//go:build !linux

package netlink

import (
	"fmt"
	"runtime"
)

// Dial creates a new netlink socket connection with the given protocol.
// On non-Linux platforms, this returns an error since netlink is Linux-only.
func Dial(protocol int) (*Conn, error) {
	return nil, fmt.Errorf("netlink sockets are not supported on %s", runtime.GOOS)
}

func closeSocket(fd int) error {
	return nil // no-op on non-Linux
}

func (c *Conn) sendCommand(msgID int32, payload []byte) (errCode int32, response []byte, err error) {
	return 0, nil, fmt.Errorf("netlink sockets are not supported on %s", runtime.GOOS)
}

func (c *Conn) listen(handler func(msgID int32, payload []byte) []byte) error {
	return fmt.Errorf("netlink sockets are not supported on %s", runtime.GOOS)
}
