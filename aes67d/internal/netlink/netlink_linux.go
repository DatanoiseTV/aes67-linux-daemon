//go:build linux

package netlink

import (
	"fmt"
	"syscall"
	"time"
)

// replyTimeout is the maximum time to wait for a reply from the kernel.
const replyTimeout = 1 * time.Second

// Dial creates a new netlink socket connection with the given protocol.
func Dial(protocol int) (*Conn, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, protocol)
	if err != nil {
		return nil, fmt.Errorf("netlink socket: %w", err)
	}

	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(syscall.Getpid()),
	}

	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("netlink bind: %w", err)
	}

	return &Conn{
		fd:  fd,
		pid: uint32(syscall.Getpid()),
	}, nil
}

func closeSocket(fd int) error {
	return syscall.Close(fd)
}

func (c *Conn) sendCommand(msgID int32, payload []byte) (errCode int32, response []byte, err error) {
	msg := BuildMessage(msgID, c.pid, payload)

	dest := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    0, // kernel
	}

	if err := syscall.Sendto(c.fd, msg, 0, dest); err != nil {
		return 0, nil, fmt.Errorf("netlink send: %w", err)
	}

	// Set receive timeout
	tv := syscall.NsecToTimeval(int64(replyTimeout))
	if err := syscall.SetsockoptTimeval(c.fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		return 0, nil, fmt.Errorf("set recv timeout: %w", err)
	}

	buf := make([]byte, recvBufSize)
	n, _, err := syscall.Recvfrom(c.fd, buf, 0)
	if err != nil {
		return 0, nil, fmt.Errorf("netlink recv: %w", err)
	}

	_, alsaMsg, respPayload, err := ParseMessage(buf[:n])
	if err != nil {
		return 0, nil, fmt.Errorf("parsing reply: %w", err)
	}

	return alsaMsg.ErrCode, respPayload, nil
}

func (c *Conn) listen(handler func(msgID int32, payload []byte) []byte) error {
	buf := make([]byte, recvBufSize)

	for {
		n, from, err := syscall.Recvfrom(c.fd, buf, 0)
		if err != nil {
			return fmt.Errorf("netlink recv: %w", err)
		}

		_, alsaMsg, payload, err := ParseMessage(buf[:n])
		if err != nil {
			continue // skip malformed messages
		}

		resp := handler(alsaMsg.ID, payload)

		if resp != nil {
			replyMsg := BuildMessage(alsaMsg.ID, c.pid, resp)
			if err := syscall.Sendto(c.fd, replyMsg, 0, from); err != nil {
				return fmt.Errorf("netlink send reply: %w", err)
			}
		}
	}
}
