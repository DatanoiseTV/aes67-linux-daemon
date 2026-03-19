// Package driver provides a high-level wrapper around the netlink package
// for communicating with the RAVENNA kernel module.
package driver

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/netlink"
)

// Event represents a kernel-to-user event received from the RAVENNA module.
type Event struct {
	MsgID   int32
	Payload []byte
}

// Driver manages communication with the RAVENNA kernel module via netlink sockets.
type Driver struct {
	u2k      *netlink.Conn
	k2u      *netlink.Conn
	events   chan Event
	handlers []func(Event)
	mu       sync.RWMutex
}

// New creates a new Driver by opening both user-to-kernel and kernel-to-user
// netlink connections.
func New() (*Driver, error) {
	u2k, err := netlink.Dial(netlink.ProtocolU2K)
	if err != nil {
		return nil, fmt.Errorf("u2k socket: %w", err)
	}
	k2u, err := netlink.Dial(netlink.ProtocolK2U)
	if err != nil {
		u2k.Close()
		return nil, fmt.Errorf("k2u socket: %w", err)
	}
	return &Driver{u2k: u2k, k2u: k2u, events: make(chan Event, 64)}, nil
}

// Close closes both netlink connections.
func (d *Driver) Close() error {
	d.u2k.Close()
	d.k2u.Close()
	return nil
}

// Lifecycle commands

// Hello sends the hello handshake to the kernel module.
func (d *Driver) Hello() error { return d.sendSimple(netlink.MsgHello) }

// Bye sends the goodbye message to the kernel module.
func (d *Driver) Bye() error { return d.sendSimple(netlink.MsgBye) }

// Start sends the start command to the kernel module.
func (d *Driver) Start() error { return d.sendSimple(netlink.MsgStart) }

// Stop sends the stop command to the kernel module.
func (d *Driver) Stop() error { return d.sendSimple(netlink.MsgStop) }

// Reset sends the reset command to the kernel module.
func (d *Driver) Reset() error { return d.sendSimple(netlink.MsgReset) }

// Audio configuration

// SetSampleRate sets the audio sample rate.
func (d *Driver) SetSampleRate(rate uint32) error {
	return d.sendUint32(netlink.MsgSetSampleRate, rate)
}

// GetSampleRate returns the current audio sample rate.
func (d *Driver) GetSampleRate() (uint32, error) {
	return d.getUint32(netlink.MsgGetSampleRate)
}

// SetTICFrameSize sets the TIC frame size.
func (d *Driver) SetTICFrameSize(size uint64) error {
	return d.sendUint64(netlink.MsgSetTICFrameSize, size)
}

// SetMaxTICFrameSize sets the maximum TIC frame size.
func (d *Driver) SetMaxTICFrameSize(size uint64) error {
	return d.sendUint64(netlink.MsgSetMaxTICFrameSize, size)
}

// SetPlayoutDelay sets the playout delay in samples.
func (d *Driver) SetPlayoutDelay(delay int32) error {
	return d.sendInt32(netlink.MsgSetPlayoutDelay, delay)
}

// SetInterfaceName sets the network interface name used by the driver.
func (d *Driver) SetInterfaceName(name string) error {
	buf := make([]byte, 64)
	copy(buf, name)
	return d.send(netlink.MsgSetInterfaceName, buf)
}

// SetNumberOfInputs sets the number of audio input channels.
func (d *Driver) SetNumberOfInputs(n uint32) error {
	return d.sendUint32(netlink.MsgSetNumInputs, n)
}

// SetNumberOfOutputs sets the number of audio output channels.
func (d *Driver) SetNumberOfOutputs(n uint32) error {
	return d.sendUint32(netlink.MsgSetNumOutputs, n)
}

// Stream management

// AddRTPStream adds a new RTP stream and returns its handle.
func (d *Driver) AddRTPStream(info *netlink.RTPStreamInfo) (uint64, error) {
	data, err := info.Marshal()
	if err != nil {
		return 0, err
	}
	errCode, resp, err := d.u2k.SendCommand(netlink.MsgAddRTPStream, data)
	if err != nil {
		return 0, err
	}
	if errCode != 0 {
		return 0, fmt.Errorf("add stream failed: error %d", errCode)
	}
	if len(resp) < 8 {
		return 0, fmt.Errorf("add stream: short response")
	}
	handle := binary.LittleEndian.Uint64(resp[:8])
	return handle, nil
}

// RemoveRTPStream removes an RTP stream by its handle.
func (d *Driver) RemoveRTPStream(handle uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, handle)
	return d.send(netlink.MsgRemoveRTPStream, buf)
}

// GetStreamStatus returns the status of an RTP stream by its handle.
func (d *Driver) GetStreamStatus(handle uint64) (*netlink.StreamStatus, error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, handle)
	errCode, resp, err := d.u2k.SendCommand(netlink.MsgGetStreamStatus, buf)
	if err != nil {
		return nil, err
	}
	if errCode != 0 {
		return nil, fmt.Errorf("get status failed: error %d", errCode)
	}
	var status netlink.StreamStatus
	if err := status.Unmarshal(resp); err != nil {
		return nil, err
	}
	return &status, nil
}

// PTP configuration and status

// SetPTPConfig sends the PTP configuration to the kernel module.
func (d *Driver) SetPTPConfig(cfg *netlink.PTPConfig) error {
	data, _ := cfg.Marshal()
	return d.send(netlink.MsgSetPTPConfig, data)
}

// GetPTPConfig retrieves the current PTP configuration from the kernel module.
func (d *Driver) GetPTPConfig() (*netlink.PTPConfig, error) {
	errCode, resp, err := d.u2k.SendCommand(netlink.MsgGetPTPConfig, nil)
	if err != nil {
		return nil, err
	}
	if errCode != 0 {
		return nil, fmt.Errorf("get ptp config: error %d", errCode)
	}
	var cfg netlink.PTPConfig
	if err := cfg.Unmarshal(resp); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetPTPStatus retrieves the current PTP status from the kernel module.
func (d *Driver) GetPTPStatus() (*netlink.PTPStatus, error) {
	errCode, resp, err := d.u2k.SendCommand(netlink.MsgGetPTPStatus, nil)
	if err != nil {
		return nil, err
	}
	if errCode != 0 {
		return nil, fmt.Errorf("get ptp status: error %d", errCode)
	}
	var status netlink.PTPStatus
	if err := status.Unmarshal(resp); err != nil {
		return nil, err
	}
	return &status, nil
}

// K2U event listener

// ListenEvents starts listening for kernel-to-user events in a goroutine
// and blocks until the context is cancelled.
func (d *Driver) ListenEvents(ctx context.Context) error {
	go func() {
		d.k2u.Listen(func(msgID int32, payload []byte) []byte {
			evt := Event{MsgID: msgID, Payload: payload}
			d.mu.RLock()
			for _, h := range d.handlers {
				h(evt)
			}
			d.mu.RUnlock()
			return nil
		})
	}()
	<-ctx.Done()
	return nil
}

// OnEvent registers a handler function that will be called for each event
// received from the kernel module.
func (d *Driver) OnEvent(handler func(Event)) {
	d.mu.Lock()
	d.handlers = append(d.handlers, handler)
	d.mu.Unlock()
}

// Internal helpers

func (d *Driver) sendSimple(msgID int32) error {
	return d.send(msgID, nil)
}

func (d *Driver) send(msgID int32, data []byte) error {
	errCode, _, err := d.u2k.SendCommand(msgID, data)
	if err != nil {
		return err
	}
	if errCode != 0 {
		return fmt.Errorf("command %d failed: error %d", msgID, errCode)
	}
	return nil
}

func (d *Driver) sendUint32(msgID int32, v uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return d.send(msgID, buf)
}

func (d *Driver) sendUint64(msgID int32, v uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return d.send(msgID, buf)
}

func (d *Driver) sendInt32(msgID int32, v int32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(v))
	return d.send(msgID, buf)
}

func (d *Driver) getUint32(msgID int32) (uint32, error) {
	errCode, resp, err := d.u2k.SendCommand(msgID, nil)
	if err != nil {
		return 0, err
	}
	if errCode != 0 {
		return 0, fmt.Errorf("command %d failed: error %d", msgID, errCode)
	}
	if len(resp) < 4 {
		return 0, fmt.Errorf("command %d: short response", msgID)
	}
	return binary.LittleEndian.Uint32(resp[:4]), nil
}
