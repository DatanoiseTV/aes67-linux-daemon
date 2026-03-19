// Package session manages the lifecycle of AES67 RTP sources and sinks.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/config"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/driver"
	"github.com/bondagit/aes67-linux-daemon/aes67d/internal/netlink"
)

// streamInfo holds the kernel handle and the user-level descriptor for a stream.
type streamInfo struct {
	handle uint64
	source *StreamSource // non-nil for sources
	sink   *StreamSink   // non-nil for sinks
}

// Manager coordinates source/sink operations against the kernel driver.
type Manager struct {
	driver  *driver.Driver
	config  *config.Config
	sources map[uint8]*streamInfo // id -> stream info
	sinks   map[uint8]*streamInfo
	mu      sync.RWMutex
}

// NewManager creates a Manager. Call Init to restore persisted state.
func NewManager(drv *driver.Driver, cfg *config.Config) *Manager {
	return &Manager{
		driver:  drv,
		config:  cfg,
		sources: make(map[uint8]*streamInfo),
		sinks:   make(map[uint8]*streamInfo),
	}
}

// Init loads the status file and restores previously active streams.
func (m *Manager) Init() error {
	if err := m.loadStatus(); err != nil {
		// A missing status file on first boot is not an error.
		if !os.IsNotExist(err) {
			return fmt.Errorf("session init: %w", err)
		}
	}
	return nil
}

// --- Source accessors --------------------------------------------------------

// GetSources returns a snapshot of all active sources.
func (m *Manager) GetSources() []StreamSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]StreamSource, 0, len(m.sources))
	for _, si := range m.sources {
		out = append(out, *si.source)
	}
	return out
}

// GetSource returns a single source by ID.
func (m *Manager) GetSource(id uint8) (StreamSource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	si, ok := m.sources[id]
	if !ok {
		return StreamSource{}, false
	}
	return *si.source, true
}

// --- Sink accessors ----------------------------------------------------------

// GetSinks returns a snapshot of all active sinks.
func (m *Manager) GetSinks() []StreamSink {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]StreamSink, 0, len(m.sinks))
	for _, si := range m.sinks {
		out = append(out, *si.sink)
	}
	return out
}

// GetSink returns a single sink by ID.
func (m *Manager) GetSink(id uint8) (StreamSink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	si, ok := m.sinks[id]
	if !ok {
		return StreamSink{}, false
	}
	return *si.sink, true
}

// GetSinkStatus queries the kernel for the runtime status of a sink.
func (m *Manager) GetSinkStatus(id uint8) (*SinkStatus, error) {
	m.mu.RLock()
	si, ok := m.sinks[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sink %d not found", id)
	}

	st, err := m.driver.GetStreamStatus(si.handle)
	if err != nil {
		return nil, fmt.Errorf("get sink %d status: %w", id, err)
	}

	return convertStreamStatus(st), nil
}

// convertStreamStatus maps kernel StreamStatus flags to the SinkStatus struct.
func convertStreamStatus(st *netlink.StreamStatus) *SinkStatus {
	return &SinkStatus{
		IsSeqIDError:       st.Flags&0x01 != 0,
		IsSSRCError:        st.Flags&0x02 != 0,
		IsPayloadTypeError: st.Flags&0x04 != 0,
		IsSACError:         st.Flags&0x08 != 0,
		IsReceiving:        st.Flags&0x10 != 0,
		IsMuted:            st.Flags&0x20 != 0,
		IsSomeMuted:        st.Flags&0x40 != 0,
		IsAllMuted:         st.Flags&0x80 != 0,
		MinTime:            st.MinTime,
	}
}

// --- Matrix ------------------------------------------------------------------

// MatrixState describes the audio routing matrix between sources and sinks.
type MatrixState struct {
	Inputs  []MatrixInput  `json:"inputs"`
	Outputs []MatrixOutput `json:"outputs"`
	Routes  []MatrixRoute  `json:"routes"`
}

// MatrixInput represents a source in the matrix.
type MatrixInput struct {
	ID       uint8    `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// MatrixOutput represents a sink in the matrix.
type MatrixOutput struct {
	ID       uint8    `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// MatrixRoute maps a source channel to a sink channel.
type MatrixRoute struct {
	Src [2]int `json:"src"` // [streamId, channelIdx]
	Dst [2]int `json:"dst"`
}

// GetMatrix builds the current routing matrix from active sources and sinks.
func (m *Manager) GetMatrix() MatrixState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := MatrixState{}

	for _, si := range m.sources {
		src := si.source
		channels := make([]string, len(src.Map))
		for i := range channels {
			channels[i] = fmt.Sprintf("%s ch%d", src.Name, i)
		}
		state.Inputs = append(state.Inputs, MatrixInput{
			ID:       src.ID,
			Name:     src.Name,
			Channels: channels,
		})
	}

	for _, si := range m.sinks {
		snk := si.sink
		channels := make([]string, len(snk.Map))
		for i := range channels {
			channels[i] = fmt.Sprintf("%s ch%d", snk.Name, i)
		}
		state.Outputs = append(state.Outputs, MatrixOutput{
			ID:       snk.ID,
			Name:     snk.Name,
			Channels: channels,
		})

		// Derive routes: for each sink channel, check if it maps to a
		// physical channel that is also used by a source channel.
		for sinkCh, physCh := range snk.Map {
			if physCh < 0 {
				continue
			}
			for _, srcSI := range m.sources {
				for srcCh, srcPhys := range srcSI.source.Map {
					if srcPhys == physCh {
						state.Routes = append(state.Routes, MatrixRoute{
							Src: [2]int{int(srcSI.source.ID), srcCh},
							Dst: [2]int{int(snk.ID), sinkCh},
						})
					}
				}
			}
		}
	}

	return state
}

// SetMatrixRoute updates a sink's channel map and re-adds the stream.
func (m *Manager) SetMatrixRoute(srcStream, srcCh, dstStream, dstCh uint8, action string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dstSI, ok := m.sinks[dstStream]
	if !ok {
		return fmt.Errorf("destination sink %d not found", dstStream)
	}

	sink := dstSI.sink
	if int(dstCh) >= len(sink.Map) {
		return fmt.Errorf("destination channel %d out of range (sink has %d channels)", dstCh, len(sink.Map))
	}

	switch action {
	case "add":
		// Look up the source to find the physical channel.
		srcSI, ok := m.sources[srcStream]
		if !ok {
			return fmt.Errorf("source %d not found", srcStream)
		}
		if int(srcCh) >= len(srcSI.source.Map) {
			return fmt.Errorf("source channel %d out of range", srcCh)
		}
		sink.Map[dstCh] = srcSI.source.Map[srcCh]

	case "remove":
		sink.Map[dstCh] = -1

	default:
		return fmt.Errorf("unknown matrix action: %s", action)
	}

	// Re-add the stream with updated routing.
	if err := m.driver.RemoveRTPStream(dstSI.handle); err != nil {
		return fmt.Errorf("remove sink for re-add: %w", err)
	}

	handle, err := m.addSinkStream(sink)
	if err != nil {
		return fmt.Errorf("re-add sink: %w", err)
	}
	dstSI.handle = handle

	return m.saveStatusLocked()
}

// --- Persistence -------------------------------------------------------------

// statusFile is the on-disk representation of active streams.
type statusFile struct {
	Sources []StreamSource `json:"sources"`
	Sinks   []StreamSink   `json:"sinks"`
}

func (m *Manager) statusPath() string {
	if m.config.StatusFile != "" {
		return m.config.StatusFile
	}
	return "./status.json"
}

// saveStatus persists the current source/sink state (acquires read lock).
func (m *Manager) saveStatus() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveStatusLocked()
}

// saveStatusLocked persists state; caller must hold at least a read lock.
func (m *Manager) saveStatusLocked() error {
	sf := statusFile{
		Sources: make([]StreamSource, 0, len(m.sources)),
		Sinks:   make([]StreamSink, 0, len(m.sinks)),
	}
	for _, si := range m.sources {
		sf.Sources = append(sf.Sources, *si.source)
	}
	for _, si := range m.sinks {
		sf.Sinks = append(sf.Sinks, *si.sink)
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(m.statusPath(), data, 0644); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	return nil
}

// loadStatus reads the status file and restores streams through the driver.
func (m *Manager) loadStatus() error {
	data, err := os.ReadFile(m.statusPath())
	if err != nil {
		return err
	}
	var sf statusFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse status: %w", err)
	}

	for i := range sf.Sources {
		src := sf.Sources[i]
		if err := m.AddSource(src); err != nil {
			return fmt.Errorf("restore source %d: %w", src.ID, err)
		}
	}
	for i := range sf.Sinks {
		sink := sf.Sinks[i]
		if err := m.AddSink(sink); err != nil {
			return fmt.Errorf("restore sink %d: %w", sink.ID, err)
		}
	}
	return nil
}

// codecWordLength returns the word length in bytes for a given codec name.
func codecWordLength(codec string) uint8 {
	switch codec {
	case "L16":
		return 2
	case "L24":
		return 3
	case "L2432", "AM824":
		return 4
	case "DSD64":
		return 1
	case "DSD128":
		return 2
	default:
		return 2 // default to 16-bit
	}
}
