package state

import (
	"os"
	"path/filepath"
	"testing"
)

func testStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Override home for testing
	t.Setenv("HOME", dir)
	return dir
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	testStateDir(t)

	s := &HostState{
		HostAlias:        "myserver",
		Hostname:         "10.0.0.1",
		User:             "admin",
		Port:             22,
		SocksEnabled:     true,
		SocksPort:        1087,
		HTTPEnabled:      true,
		HTTPPort:         8118,
		MacSyncMode:      MacSyncFull,
		SourceConfigPath: "/home/user/.ssh/config",
	}

	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load("myserver")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.HostAlias != "myserver" {
		t.Errorf("HostAlias = %q, want %q", loaded.HostAlias, "myserver")
	}
	if loaded.Hostname != "10.0.0.1" {
		t.Errorf("Hostname = %q, want %q", loaded.Hostname, "10.0.0.1")
	}
	if loaded.User != "admin" {
		t.Errorf("User = %q, want %q", loaded.User, "admin")
	}
	if loaded.SocksEnabled != true {
		t.Errorf("SocksEnabled = %v, want true", loaded.SocksEnabled)
	}
	if loaded.SocksPort != 1087 {
		t.Errorf("SocksPort = %d, want 1087", loaded.SocksPort)
	}
	if loaded.HTTPEnabled != true {
		t.Errorf("HTTPEnabled = %v, want true", loaded.HTTPEnabled)
	}
	if loaded.HTTPPort != 8118 {
		t.Errorf("HTTPPort = %d, want 8118", loaded.HTTPPort)
	}
	if loaded.MacSyncMode != MacSyncFull {
		t.Errorf("MacSyncMode = %q, want %q", loaded.MacSyncMode, MacSyncFull)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestLoad_MissingReturnsNil(t *testing.T) {
	testStateDir(t)

	loaded, err := Load("nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil for missing state, got %+v", loaded)
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	home := testStateDir(t)

	s := &HostState{
		HostAlias: "testhost",
		Hostname:  "192.168.1.1",
	}

	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists at expected path
	expectedPath := filepath.Join(home, ".sshmux", "hosts", "testhost.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected state file at %s: %v", expectedPath, err)
	}

	// Verify no temp files remain
	dir := filepath.Join(home, ".sshmux", "hosts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			t.Errorf("unexpected file in state dir: %s", e.Name())
		}
	}
}

func TestSaveAndLoadTerminalProxy(t *testing.T) {
	testStateDir(t)

	cfg := &TerminalProxyConfig{
		Enabled:   true,
		HTTPAddr:  "127.0.0.1:7897",
		SOCKSAddr: "127.0.0.1:1080",
	}

	if err := SaveTerminalProxy(cfg); err != nil {
		t.Fatalf("SaveTerminalProxy: %v", err)
	}

	loaded, err := LoadTerminalProxy()
	if err != nil {
		t.Fatalf("LoadTerminalProxy: %v", err)
	}

	if !loaded.Enabled {
		t.Error("expected Enabled=true")
	}
	if loaded.HTTPAddr != "127.0.0.1:7897" {
		t.Errorf("HTTPAddr = %q, want %q", loaded.HTTPAddr, "127.0.0.1:7897")
	}
	if loaded.SOCKSAddr != "127.0.0.1:1080" {
		t.Errorf("SOCKSAddr = %q, want %q", loaded.SOCKSAddr, "127.0.0.1:1080")
	}
}

func TestLoadTerminalProxy_MissingReturnsDefault(t *testing.T) {
	testStateDir(t)

	loaded, err := LoadTerminalProxy()
	if err != nil {
		t.Fatalf("LoadTerminalProxy: %v", err)
	}
	if loaded.Enabled {
		t.Error("expected Enabled=false for missing config")
	}
}

func TestListHosts(t *testing.T) {
	testStateDir(t)

	// Save two hosts
	hosts := []*HostState{
		{HostAlias: "server-a", Hostname: "10.0.0.1"},
		{HostAlias: "server-b", Hostname: "10.0.0.2"},
	}
	for _, h := range hosts {
		if err := Save(h); err != nil {
			t.Fatalf("Save %s: %v", h.HostAlias, err)
		}
	}

	listed, err := ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("ListHosts returned %d hosts, want 2", len(listed))
	}
}

func TestLoadGlobalConfig_DefaultsIncludeRemoteSource(t *testing.T) {
	testStateDir(t)

	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig: %v", err)
	}
	if cfg.RemoteSource != RemoteSourceSSHMux {
		t.Fatalf("RemoteSource = %q, want %q", cfg.RemoteSource, RemoteSourceSSHMux)
	}
	if cfg.ExternalHTTPAddr != "127.0.0.1:7897" {
		t.Fatalf("ExternalHTTPAddr = %q, want %q", cfg.ExternalHTTPAddr, "127.0.0.1:7897")
	}
	if cfg.ExternalSOCKSAddr != "127.0.0.1:7897" {
		t.Fatalf("ExternalSOCKSAddr = %q, want %q", cfg.ExternalSOCKSAddr, "127.0.0.1:7897")
	}
}
