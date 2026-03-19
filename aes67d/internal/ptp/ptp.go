package ptp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/netlink"
)

// Driver defines the interface for PTP driver operations.
// This is satisfied by *driver.Driver once that package is implemented.
type Driver interface {
	GetPTPStatus() (*netlink.PTPStatus, error)
	GetPTPConfig() (*netlink.PTPConfig, error)
	SetPTPConfig(cfg *netlink.PTPConfig) error
}

// Status represents the current PTP synchronization status.
type Status struct {
	StatusStr string `json:"status"` // "locked", "locking", "unlocked"
	GMID      string `json:"gmid"`   // formatted as "XX-XX-XX-XX-XX-XX-XX-XX"
	Jitter    int32  `json:"jitter"`
}

// Config represents the PTP configuration.
type Config struct {
	Domain int `json:"domain"`
	DSCP   int `json:"dscp"`
}

// Monitor polls PTP status from the kernel driver and caches results.
type Monitor struct {
	driver       Driver
	status       Status
	config       Config
	mu           sync.RWMutex
	onChange     []func(status string)
	pollInterval time.Duration
}

// NewMonitor creates a new PTP monitor with the given driver and poll interval.
func NewMonitor(drv Driver, pollInterval time.Duration) *Monitor {
	return &Monitor{
		driver:       drv,
		pollInterval: pollInterval,
		status:       Status{StatusStr: "unlocked", GMID: "00-00-00-00-00-00-00-00"},
	}
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

func (m *Monitor) poll() {
	ptpStatus, err := m.driver.GetPTPStatus()
	if err != nil {
		return
	}

	newStatus := formatStatus(ptpStatus)

	m.mu.Lock()
	oldStatusStr := m.status.StatusStr
	m.status = newStatus
	m.mu.Unlock()

	// Fire callbacks on status change
	if newStatus.StatusStr != oldStatusStr {
		m.mu.RLock()
		callbacks := make([]func(string), len(m.onChange))
		copy(callbacks, m.onChange)
		m.mu.RUnlock()

		for _, fn := range callbacks {
			fn(newStatus.StatusStr)
		}
	}
}

func formatStatus(s *netlink.PTPStatus) Status {
	var statusStr string
	switch s.LockStatus {
	case 0:
		statusStr = "unlocked"
	case 1:
		statusStr = "locking"
	case 2:
		statusStr = "locked"
	default:
		statusStr = "unlocked"
	}

	// Format GMID as XX-XX-XX-XX-XX-XX-XX-XX from uint64
	gmid := fmt.Sprintf("%02X-%02X-%02X-%02X-%02X-%02X-%02X-%02X",
		byte(s.GMID>>56), byte(s.GMID>>48), byte(s.GMID>>40), byte(s.GMID>>32),
		byte(s.GMID>>24), byte(s.GMID>>16), byte(s.GMID>>8), byte(s.GMID))

	return Status{StatusStr: statusStr, GMID: gmid, Jitter: s.Jitter}
}

// GetStatus returns the cached PTP status.
func (m *Monitor) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetConfig returns the cached PTP configuration.
func (m *Monitor) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetConfig updates the PTP configuration on the driver and caches it.
func (m *Monitor) SetConfig(cfg Config) error {
	ptpCfg := &netlink.PTPConfig{Domain: uint8(cfg.Domain), DSCP: uint8(cfg.DSCP)}
	if err := m.driver.SetPTPConfig(ptpCfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()
	return nil
}

// OnChange registers a callback that fires when the PTP lock status changes.
func (m *Monitor) OnChange(fn func(status string)) {
	m.mu.Lock()
	m.onChange = append(m.onChange, fn)
	m.mu.Unlock()
}

// InitConfig sets the initial PTP configuration on the driver and caches it.
func (m *Monitor) InitConfig(domain, dscp int) error {
	cfg := &netlink.PTPConfig{Domain: uint8(domain), DSCP: uint8(dscp)}
	if err := m.driver.SetPTPConfig(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.config = Config{Domain: domain, DSCP: dscp}
	m.mu.Unlock()
	return nil
}
