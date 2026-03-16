package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/liuxuan/sshmux/internal/ssh"
)

// SOCKS manages SOCKS proxy via SSH DynamicForward.
type SOCKS struct {
	runner ssh.Runner
}

// NewSOCKS creates a SOCKS manager with the given Runner.
func NewSOCKS(r ssh.Runner) *SOCKS {
	return &SOCKS{runner: r}
}

func sshControlPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "cm-%C"), nil
}

// Enable adds a DynamicForward on the given port via the ControlMaster socket.
func (s *SOCKS) Enable(ctx context.Context, host string, port int) error {
	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("socks enable %s: %w", host, err)
	}
	slog.Debug("enabling socks forward", "host", host, "port", port)
	if err := s.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		"-O", "forward",
		"-D", fmt.Sprintf("127.0.0.1:%d", port),
		host,
	); err != nil {
		return fmt.Errorf("socks enable %s port %d: %w", host, port, err)
	}
	slog.Info("socks enabled", "host", host, "port", port)
	return nil
}

// Disable removes the DynamicForward on the given port.
func (s *SOCKS) Disable(ctx context.Context, host string, port int) error {
	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("socks disable %s: %w", host, err)
	}
	slog.Debug("disabling socks forward", "host", host, "port", port)
	if err := s.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		"-O", "cancel",
		"-D", fmt.Sprintf("127.0.0.1:%d", port),
		host,
	); err != nil {
		return fmt.Errorf("socks disable %s port %d: %w", host, port, err)
	}
	slog.Info("socks disabled", "host", host, "port", port)
	return nil
}

// SetPort changes the SOCKS port using the safe start-new, verify, stop-old pattern.
func (s *SOCKS) SetPort(ctx context.Context, host string, oldPort, newPort int) error {
	slog.Debug("switching socks port", "host", host, "old_port", oldPort, "new_port", newPort)

	// 1. Start new forward
	if err := s.Enable(ctx, host, newPort); err != nil {
		return fmt.Errorf("socks set-port %s: start new: %w", host, err)
	}

	// 2. Cancel old forward (best-effort)
	if err := s.Disable(ctx, host, oldPort); err != nil {
		slog.Warn("socks set-port: failed to cancel old forward", "host", host, "port", oldPort, "err", err)
	}

	slog.Info("socks port changed", "host", host, "old_port", oldPort, "new_port", newPort)
	return nil
}
