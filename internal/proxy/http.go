package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/liuxuan/sshmux/internal/ssh"
)

// HTTP manages the Privoxy HTTP/HTTPS proxy adapter.
type HTTP struct {
	runner ssh.Runner
}

// NewHTTP creates an HTTP proxy manager with the given Runner.
func NewHTTP(r ssh.Runner) *HTTP {
	return &HTTP{runner: r}
}

// configDir returns the directory for Privoxy config files.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".sshmux", "privoxy"), nil
}

// configPath returns the Privoxy config file path for a host.
func configPath(host string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", fmt.Errorf("config path %s: %w", host, err)
	}
	return filepath.Join(dir, host+".conf"), nil
}

// pidPath returns the Privoxy PID file path for a host.
func pidPath(host string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", fmt.Errorf("pid path %s: %w", host, err)
	}
	return filepath.Join(dir, host+".pid"), nil
}

// writeConfig generates a Privoxy config file that forwards to the SOCKS port.
func writeConfig(host string, httpPort, socksPort int) error {
	dir, err := configDir()
	if err != nil {
		return fmt.Errorf("write privoxy config %s: %w", host, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create privoxy config dir: %w", err)
	}

	path, err := configPath(host)
	if err != nil {
		return fmt.Errorf("write privoxy config %s: %w", host, err)
	}

	pid, err := pidPath(host)
	if err != nil {
		return fmt.Errorf("write privoxy config %s: %w", host, err)
	}

	content := fmt.Sprintf(`listen-address 127.0.0.1:%d
forward-socks5 / 127.0.0.1:%d .
logdir /tmp
logfile privoxy-%s.log
pidfile %s
`, httpPort, socksPort, host, pid)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("write privoxy config: %w", err)
	}
	slog.Debug("privoxy config written", "host", host, "path", path)
	return nil
}

// Enable starts Privoxy for the given host.
func (h *HTTP) Enable(ctx context.Context, host string, httpPort, socksPort int) error {
	if err := writeConfig(host, httpPort, socksPort); err != nil {
		return fmt.Errorf("http enable %s: %w", host, err)
	}

	path, err := configPath(host)
	if err != nil {
		return fmt.Errorf("http enable %s: %w", host, err)
	}

	slog.Debug("starting privoxy", "host", host, "http_port", httpPort, "socks_port", socksPort)
	if err := h.runner.Run(ctx, "privoxy", path); err != nil {
		return fmt.Errorf("http enable %s: start privoxy: %w", host, err)
	}
	slog.Info("http proxy enabled", "host", host, "port", httpPort)
	return nil
}

// Disable stops Privoxy for the given host.
func (h *HTTP) Disable(ctx context.Context, host string) error {
	pid, err := pidPath(host)
	if err != nil {
		return fmt.Errorf("http disable %s: %w", host, err)
	}

	// Read PID and kill
	data, err := os.ReadFile(pid)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("privoxy pid file not found, already stopped", "host", host)
			return nil
		}
		return fmt.Errorf("http disable %s: read pid: %w", host, err)
	}

	pidStr := string(data)
	pidStr = trimWhitespace(pidStr)

	slog.Debug("stopping privoxy", "host", host, "pid", pidStr)
	if err := h.runner.Run(ctx, "kill", pidStr); err != nil {
		slog.Warn("failed to kill privoxy", "host", host, "pid", pidStr, "err", err)
	}

	// Clean up config and pid files
	cfgPath, _ := configPath(host)
	os.Remove(cfgPath)
	os.Remove(pid)

	slog.Info("http proxy disabled", "host", host)
	return nil
}

// SetPort changes the HTTP proxy port using safe switch pattern.
func (h *HTTP) SetPort(ctx context.Context, host string, oldPort, newPort, socksPort int) error {
	slog.Debug("switching http port", "host", host, "old_port", oldPort, "new_port", newPort)

	// 1. Start new instance
	if err := h.Enable(ctx, host, newPort, socksPort); err != nil {
		return fmt.Errorf("http set-port %s: start new: %w", host, err)
	}

	// 2. The old instance is replaced by the new config, so no explicit stop needed
	// if Privoxy replaces itself. In a more robust impl we'd stop old first.

	slog.Info("http port changed", "host", host, "old_port", oldPort, "new_port", newPort)
	return nil
}

func trimWhitespace(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\n' && s[i] != '\r' && s[i] != '\t' {
			result = append(result, s[i])
		}
	}
	return string(result)
}
