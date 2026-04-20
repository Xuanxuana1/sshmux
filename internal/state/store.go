package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// stateDir returns the base state directory (~/.sshmux).
func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".sshmux"), nil
}

// hostsDir returns the per-host state directory (~/.sshmux/hosts).
func hostsDir() (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", fmt.Errorf("hosts dir: %w", err)
	}
	return filepath.Join(base, "hosts"), nil
}

// statePath returns the full file path for a host alias state file.
func statePath(alias string) (string, error) {
	dir, err := hostsDir()
	if err != nil {
		return "", fmt.Errorf("state path %s: %w", alias, err)
	}
	return filepath.Join(dir, alias+".json"), nil
}

// Load reads the HostState for the given alias.
// Returns (nil, nil) if the file does not exist.
func Load(alias string) (*HostState, error) {
	path, err := statePath(alias)
	if err != nil {
		return nil, fmt.Errorf("load state %s: %w", alias, err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state %s: %w", alias, err)
	}
	var s HostState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", alias, err)
	}
	slog.Debug("state loaded", "host", alias, "path", path)
	return &s, nil
}

// Save persists a HostState using atomic write (temp file + rename).
func Save(s *HostState) error {
	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state %s: %w", s.HostAlias, err)
	}

	dir, err := hostsDir()
	if err != nil {
		return fmt.Errorf("save state %s: %w", s.HostAlias, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	path, err := statePath(s.HostAlias)
	if err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("save state %s: %w", s.HostAlias, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename state file: %w", err)
	}
	slog.Debug("state saved", "host", s.HostAlias, "path", path)
	return nil
}

// ListHosts returns all saved HostState entries.
func ListHosts() ([]*HostState, error) {
	dir, err := hostsDir()
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list state dir: %w", err)
	}

	var hosts []*HostState
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		alias := e.Name()[:len(e.Name())-len(".json")]
		s, err := Load(alias)
		if err != nil {
			slog.Warn("skip corrupt state", "host", alias, "err", err)
			continue
		}
		if s != nil {
			hosts = append(hosts, s)
		}
	}
	return hosts, nil
}

// globalConfigPath returns the path for the global config file.
func globalConfigPath() (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", fmt.Errorf("global config path: %w", err)
	}
	return filepath.Join(base, "global.json"), nil
}

// LoadGlobalConfig reads the global config.
// Returns defaults if the file does not exist.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := globalConfigPath()
	if err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultGlobalConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}
	cfg := DefaultGlobalConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	// Fill in zero values with defaults.
	if cfg.SocksPort == 0 {
		cfg.SocksPort = 7897
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 7897
	}
	if cfg.RemoteSource == "" {
		cfg.RemoteSource = RemoteSourceSSHMux
	}
	if cfg.ExternalHTTPAddr == "" {
		cfg.ExternalHTTPAddr = fmt.Sprintf("127.0.0.1:%d", cfg.HTTPPort)
	}
	if cfg.ExternalSOCKSAddr == "" {
		cfg.ExternalSOCKSAddr = fmt.Sprintf("127.0.0.1:%d", cfg.SocksPort)
	}
	return cfg, nil
}

// SaveGlobalConfig persists the global config using atomic write.
func SaveGlobalConfig(cfg *GlobalConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}
	base, err := stateDir()
	if err != nil {
		return fmt.Errorf("save global config: %w", err)
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	tmp, err := os.CreateTemp(base, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()
	path, err := globalConfigPath()
	if err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("save global config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename global config: %w", err)
	}
	return nil
}

// terminalProxyPath returns the path for the global terminal-proxy config.
func terminalProxyPath() (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", fmt.Errorf("terminal-proxy path: %w", err)
	}
	return filepath.Join(base, "terminal-proxy.json"), nil
}

// LoadTerminalProxy reads the global terminal-proxy config.
// Returns a zero-value config (disabled) if the file does not exist.
func LoadTerminalProxy() (*TerminalProxyConfig, error) {
	path, err := terminalProxyPath()
	if err != nil {
		return nil, fmt.Errorf("load terminal-proxy: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &TerminalProxyConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read terminal-proxy config: %w", err)
	}
	var cfg TerminalProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse terminal-proxy config: %w", err)
	}
	return &cfg, nil
}

// SaveTerminalProxy persists the terminal-proxy config using atomic write.
func SaveTerminalProxy(cfg *TerminalProxyConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal terminal-proxy config: %w", err)
	}

	base, err := stateDir()
	if err != nil {
		return fmt.Errorf("save terminal-proxy: %w", err)
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	tmp, err := os.CreateTemp(base, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	path, err := terminalProxyPath()
	if err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("save terminal-proxy: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename terminal-proxy config: %w", err)
	}
	slog.Debug("terminal-proxy config saved", "path", path)
	return nil
}
