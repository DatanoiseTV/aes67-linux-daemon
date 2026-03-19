package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ravennaServiceType is the DNS-SD service type for RAVENNA devices.
	ravennaServiceType = "_ravenna._udp"
	// mdnsBrowseDomain is the default mDNS browsing domain.
	mdnsBrowseDomain = "local."
)

// MDNSBrowser discovers RAVENNA sources via mDNS/DNS-SD.
type MDNSBrowser struct {
	sources  map[string]*RemoteSource
	mu       sync.RWMutex
	resolver *zeroconf.Resolver
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewMDNSBrowser creates a new mDNS browser for RAVENNA service discovery.
func NewMDNSBrowser() *MDNSBrowser {
	return &MDNSBrowser{
		sources: make(map[string]*RemoteSource),
		done:    make(chan struct{}),
	}
}

// Start begins browsing for _ravenna._udp services via mDNS.
func (m *MDNSBrowser) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("mdns: create resolver: %w", err)
	}
	m.resolver = resolver

	go m.browseLoop(ctx)
	return nil
}

// Stop halts mDNS browsing.
func (m *MDNSBrowser) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
}

// GetSources returns a thread-safe snapshot of all mDNS-discovered sources.
func (m *MDNSBrowser) GetSources() []RemoteSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RemoteSource, 0, len(m.sources))
	for _, src := range m.sources {
		result = append(result, *src)
	}
	return result
}

func (m *MDNSBrowser) browseLoop(ctx context.Context) {
	defer close(m.done)

	for {
		entries := make(chan *zeroconf.ServiceEntry)

		go func() {
			for entry := range entries {
				m.handleEntry(entry)
			}
		}()

		// Browse with a timeout; re-browse periodically to catch new devices.
		browseCtx, browseCancel := context.WithTimeout(ctx, 30*time.Second)
		err := m.resolver.Browse(browseCtx, ravennaServiceType, mdnsBrowseDomain, entries)
		if err != nil {
			browseCancel()
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(5 * time.Second)
				continue
			}
		}

		// Wait for browse to complete.
		<-browseCtx.Done()
		browseCancel()

		select {
		case <-ctx.Done():
			return
		default:
			// Brief pause before re-browsing.
			time.Sleep(5 * time.Second)
		}
	}
}

func (m *MDNSBrowser) handleEntry(entry *zeroconf.ServiceEntry) {
	if entry == nil {
		return
	}

	id := entry.Instance
	name := entry.Instance
	var address string

	if len(entry.AddrIPv4) > 0 {
		address = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		address = entry.AddrIPv6[0].String()
	}

	// Extract additional info from TXT records.
	domain := ""
	sdp := ""
	for _, txt := range entry.Text {
		if strings.HasPrefix(txt, "name=") {
			name = strings.TrimPrefix(txt, "name=")
		}
		if strings.HasPrefix(txt, "clock-domain=") {
			domain = strings.TrimPrefix(txt, "clock-domain=")
		}
		if strings.HasPrefix(txt, "sdp=") {
			sdp = strings.TrimPrefix(txt, "sdp=")
		}
	}

	m.mu.Lock()
	m.sources[id] = &RemoteSource{
		Source:         "mDNS",
		ID:             id,
		Name:           name,
		Domain:         domain,
		Address:        address,
		SDP:            sdp,
		LastSeen:       time.Now(),
		AnnouncePeriod: 0, // mDNS doesn't use periodic announcements the same way
	}
	m.mu.Unlock()
}
