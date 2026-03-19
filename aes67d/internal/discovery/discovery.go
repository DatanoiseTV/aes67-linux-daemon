package discovery

import (
	"context"
)

// Manager combines SAP and mDNS discovery into a unified source discovery system.
type Manager struct {
	sap         *SAPListener
	mdns        *MDNSBrowser
	mdnsEnabled bool
}

// NewManager creates a new discovery manager.
// iface specifies the network interface for SAP multicast.
// sapInterval is unused by the listener but reserved for future use.
// mdnsEnabled controls whether mDNS browsing is active.
func NewManager(iface string, sapInterval int, mdnsEnabled bool) *Manager {
	m := &Manager{
		sap:         NewSAPListener(iface),
		mdnsEnabled: mdnsEnabled,
	}
	if mdnsEnabled {
		m.mdns = NewMDNSBrowser()
	}
	return m
}

// Start begins all configured discovery mechanisms.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.sap.Start(ctx); err != nil {
		return err
	}
	if m.mdnsEnabled && m.mdns != nil {
		if err := m.mdns.Start(ctx); err != nil {
			m.sap.Stop()
			return err
		}
	}
	return nil
}

// Stop halts all discovery mechanisms.
func (m *Manager) Stop() {
	m.sap.Stop()
	if m.mdnsEnabled && m.mdns != nil {
		m.mdns.Stop()
	}
}

// GetAllSources returns a merged list of sources from all discovery mechanisms.
func (m *Manager) GetAllSources() []RemoteSource {
	sources := m.sap.GetSources()
	if m.mdnsEnabled && m.mdns != nil {
		sources = append(sources, m.mdns.GetSources()...)
	}
	return sources
}
