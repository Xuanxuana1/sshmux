package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_SimpleHost(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	content := `Host myserver
  HostName 10.0.0.1
  User admin
  Port 2222
  IdentityFile ~/.ssh/id_rsa

Host devbox
  HostName 192.168.1.100
  User developer
`

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	entries, err := ParseFile(configFile)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	tests := []struct {
		name     string
		idx      int
		alias    string
		hostname string
		user     string
		port     string
	}{
		{"first host", 0, "myserver", "10.0.0.1", "admin", "2222"},
		{"second host", 1, "devbox", "192.168.1.100", "developer", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := entries[tt.idx]
			if e.Alias != tt.alias {
				t.Errorf("Alias = %q, want %q", e.Alias, tt.alias)
			}
			if e.Hostname != tt.hostname {
				t.Errorf("Hostname = %q, want %q", e.Hostname, tt.hostname)
			}
			if e.User != tt.user {
				t.Errorf("User = %q, want %q", e.User, tt.user)
			}
			if e.Port != tt.port {
				t.Errorf("Port = %q, want %q", e.Port, tt.port)
			}
		})
	}
}

func TestParseFile_WildcardSkipped(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	content := `Host *
  ServerAliveInterval 60
  ServerAliveCountMax 3

Host myserver
  HostName 10.0.0.1

Host *.internal
  ProxyJump bastion
`

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	entries, err := ParseFile(configFile)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (wildcards should be skipped)", len(entries))
	}
	if entries[0].Alias != "myserver" {
		t.Errorf("Alias = %q, want %q", entries[0].Alias, "myserver")
	}
}

func TestParseFile_IncludeDirective(t *testing.T) {
	dir := t.TempDir()

	// Create included file
	includedFile := filepath.Join(dir, "included")
	includedContent := `Host remote-server
  HostName 172.16.0.50
  User deploy
`
	if err := os.WriteFile(includedFile, []byte(includedContent), 0644); err != nil {
		t.Fatalf("write included: %v", err)
	}

	// Create main config with Include
	mainFile := filepath.Join(dir, "config")
	mainContent := `Host local-server
  HostName 127.0.0.1

Include ` + includedFile + `
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	entries, err := ParseFile(mainFile)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// Verify both hosts are present
	aliases := make(map[string]bool)
	for _, e := range entries {
		aliases[e.Alias] = true
	}
	if !aliases["local-server"] {
		t.Error("missing local-server from main config")
	}
	if !aliases["remote-server"] {
		t.Error("missing remote-server from included config")
	}
}

func TestParseFile_ProxyJumpAndProxyCommand(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	content := `Host jump-target
  HostName 10.0.0.5
  ProxyJump bastion

Host tunnel-host
  HostName 10.0.0.6
  ProxyCommand ssh -W %h:%p bastion
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	entries, err := ParseFile(configFile)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	if entries[0].ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q, want %q", entries[0].ProxyJump, "bastion")
	}
	if entries[1].ProxyCommand != "ssh -W %h:%p bastion" {
		t.Errorf("ProxyCommand = %q, want %q", entries[1].ProxyCommand, "ssh -W %h:%p bastion")
	}
}

func TestParseFile_IncludeGlob(t *testing.T) {
	dir := t.TempDir()
	confDir := filepath.Join(dir, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("mkdir conf.d: %v", err)
	}

	// Create two included files
	for _, name := range []string{"a.conf", "b.conf"} {
		content := "Host " + name[:1] + "-server\n  HostName 10.0.0.1\n"
		if err := os.WriteFile(filepath.Join(confDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Create main config with glob include
	mainFile := filepath.Join(dir, "config")
	mainContent := "Include " + filepath.Join(confDir, "*.conf") + "\n"
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	entries, err := ParseFile(mainFile)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestSplitKeyValue(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
	}{
		{"Host myserver", "Host", "myserver"},
		{"HostName 10.0.0.1", "HostName", "10.0.0.1"},
		{"Port=2222", "Port", "2222"},
		{"  IdentityFile ~/.ssh/id_rsa", "IdentityFile", "~/.ssh/id_rsa"},
		{"ProxyCommand ssh -W %h:%p bastion", "ProxyCommand", "ssh -W %h:%p bastion"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			key, val := splitKeyValue(tt.input)
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
			if val != tt.wantVal {
				t.Errorf("val = %q, want %q", val, tt.wantVal)
			}
		})
	}
}

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		alias string
		want  bool
	}{
		{"myserver", false},
		{"*", true},
		{"*.internal", true},
		{"dev?box", true},
		{"server-01", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got := isWildcard(tt.alias)
			if got != tt.want {
				t.Errorf("isWildcard(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}
