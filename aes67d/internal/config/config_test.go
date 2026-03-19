package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigHasSensibleValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.RTSPPort != 8854 {
		t.Errorf("RTSPPort = %d, want 8854", cfg.RTSPPort)
	}
	if cfg.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cfg.SampleRate)
	}
	if cfg.RTPPort != 5004 {
		t.Errorf("RTPPort = %d, want 5004", cfg.RTPPort)
	}
	if cfg.SAPMcastAddr != "239.255.255.255" {
		t.Errorf("SAPMcastAddr = %q, want 239.255.255.255", cfg.SAPMcastAddr)
	}
	if cfg.TICFrameSize != 48 {
		t.Errorf("TICFrameSize = %d, want 48", cfg.TICFrameSize)
	}
	if cfg.MaxTICFrameSize != 1024 {
		t.Errorf("MaxTICFrameSize = %d, want 1024", cfg.MaxTICFrameSize)
	}
	if !cfg.MDNSEnabled {
		t.Error("MDNSEnabled should be true by default")
	}
	if !cfg.AutoSinksUpdate {
		t.Error("AutoSinksUpdate should be true by default")
	}
	if cfg.StreamerEnabled {
		t.Error("StreamerEnabled should be false by default")
	}
}

func TestLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.conf")

	original := DefaultConfig()
	original.HTTPPort = 9090
	original.InterfaceName = "eth0"
	original.MDNSEnabled = false

	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", loaded.HTTPPort)
	}
	if loaded.InterfaceName != "eth0" {
		t.Errorf("InterfaceName = %q, want eth0", loaded.InterfaceName)
	}
	if loaded.MDNSEnabled {
		t.Error("MDNSEnabled should be false after roundtrip")
	}
	// Fields not set should retain defaults
	if loaded.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", loaded.SampleRate)
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/daemon.conf")
	if err == nil {
		t.Fatal("expected error loading nonexistent file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.conf")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error loading invalid JSON")
	}
}

func TestLoadActualDaemonConf(t *testing.T) {
	// Load the actual daemon.conf from the project to verify compatibility.
	daemonConf := filepath.Join("..", "..", "..", "daemon", "daemon.conf")
	if _, err := os.Stat(daemonConf); os.IsNotExist(err) {
		t.Skipf("daemon.conf not found at %s, skipping compatibility test", daemonConf)
	}

	cfg, err := Load(daemonConf)
	if err != nil {
		t.Fatalf("Load daemon.conf: %v", err)
	}

	// Verify values match the known daemon.conf contents.
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.RTSPPort != 8854 {
		t.Errorf("RTSPPort = %d, want 8854", cfg.RTSPPort)
	}
	if cfg.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", cfg.SampleRate)
	}
	if cfg.InterfaceName != "lo" {
		t.Errorf("InterfaceName = %q, want lo", cfg.InterfaceName)
	}
	if cfg.RTPMcastBase != "239.1.0.1" {
		t.Errorf("RTPMcastBase = %q, want 239.1.0.1", cfg.RTPMcastBase)
	}
	if cfg.PTPDomain != 0 {
		t.Errorf("PTPDomain = %d, want 0", cfg.PTPDomain)
	}
	if cfg.PTPDSCP != 48 {
		t.Errorf("PTPDSCP = %d, want 48", cfg.PTPDSCP)
	}
	if !cfg.MDNSEnabled {
		t.Error("MDNSEnabled should be true")
	}
	if !cfg.AutoSinksUpdate {
		t.Error("AutoSinksUpdate should be true")
	}
	if cfg.StreamerEnabled {
		t.Error("StreamerEnabled should be false")
	}
	if cfg.SAPInterval != 30 {
		t.Errorf("SAPInterval = %d, want 30", cfg.SAPInterval)
	}
}
