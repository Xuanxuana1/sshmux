package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/liuxuan/sshmux/internal/ssh"
)

// HTTP manages the built-in HTTP→SOCKS5 proxy adapter.
type HTTP struct {
	runner ssh.Runner
}

// NewHTTP creates an HTTP proxy manager with the given Runner.
func NewHTTP(r ssh.Runner) *HTTP {
	return &HTTP{runner: r}
}

// configDir returns the directory for HTTP proxy PID files.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".sshmux", "httpproxy"), nil
}

// pidPath returns the PID file path for a host's HTTP proxy process.
func pidPath(host string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", fmt.Errorf("pid path %s: %w", host, err)
	}
	return filepath.Join(dir, host+".pid"), nil
}

// Enable starts the built-in HTTP→SOCKS5 proxy for the given host.
// It spawns sshmux itself as a detached background process running the
// "_proxy-server" subcommand, so the proxy survives TUI exit.
func (h *HTTP) Enable(ctx context.Context, host string, httpPort, socksPort int) error {
	dir, err := configDir()
	if err != nil {
		return fmt.Errorf("http enable %s: %w", host, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("http enable %s: create dir: %w", host, err)
	}

	pidFile, err := pidPath(host)
	if err != nil {
		return fmt.Errorf("http enable %s: %w", host, err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("http enable %s: find executable: %w", host, err)
	}

	cmd := exec.Command(self, "_proxy-server",
		"--http-port", fmt.Sprintf("%d", httpPort),
		"--socks-port", fmt.Sprintf("%d", socksPort),
	)
	// Detach from the current session so the proxy keeps running after TUI exits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	slog.Debug("starting http proxy server", "host", host, "http_port", httpPort, "socks_port", socksPort)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("http enable %s: start proxy: %w", host, err)
	}

	// Parent writes the PID file; the child only needs to serve.
	pidStr := fmt.Sprintf("%d", cmd.Process.Pid)
	if err := os.WriteFile(pidFile, []byte(pidStr), 0600); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("http enable %s: write pid: %w", host, err)
	}
	cmd.Process.Release()

	slog.Info("http proxy enabled", "host", host, "port", httpPort)
	return nil
}

// Disable stops the HTTP proxy for the given host.
func (h *HTTP) Disable(ctx context.Context, host string) error {
	pidFile, err := pidPath(host)
	if err != nil {
		return fmt.Errorf("http disable %s: %w", host, err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("http proxy pid file not found, already stopped", "host", host)
			return nil
		}
		return fmt.Errorf("http disable %s: read pid: %w", host, err)
	}

	pidStr := trimWhitespace(string(data))
	slog.Debug("stopping http proxy", "host", host, "pid", pidStr)
	if err := h.runner.Run(ctx, "kill", pidStr); err != nil {
		slog.Warn("failed to kill http proxy", "host", host, "pid", pidStr, "err", err)
	}

	os.Remove(pidFile)
	slog.Info("http proxy disabled", "host", host)
	return nil
}

// SetPort stops the old proxy and starts a new one on the new port.
func (h *HTTP) SetPort(ctx context.Context, host string, oldPort, newPort, socksPort int) error {
	slog.Debug("switching http port", "host", host, "old_port", oldPort, "new_port", newPort)
	if err := h.Disable(ctx, host); err != nil {
		slog.Warn("set-port: disable old http proxy", "host", host, "err", err)
	}
	return h.Enable(ctx, host, newPort, socksPort)
}

func trimWhitespace(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			result = append(result, b)
		}
	}
	return string(result)
}
