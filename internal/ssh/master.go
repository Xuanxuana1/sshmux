package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Master manages SSH ControlMaster connections.
type Master struct {
	runner Runner
}

// NewMaster creates a Master with the given Runner implementation.
func NewMaster(r Runner) *Master {
	return &Master{runner: r}
}

// controlPath returns the SSH ControlPath for the given host.
func controlPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "cm-%C"), nil
}

// Connect starts a persistent SSH master connection.
func (m *Master) Connect(ctx context.Context, host string) error {
	cp, err := controlPath()
	if err != nil {
		return fmt.Errorf("connect %s: %w", host, err)
	}
	args := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + cp,
		"-o", "ControlPersist=yes",
		"-MNf", host,
	}
	slog.Debug("connecting ssh master", "host", host, "control_path", cp)
	if err := m.runner.Run(ctx, "ssh", args...); err != nil {
		if stalePath, ok := staleControlSocketPath(err); ok {
			slog.Warn("removing stale ssh control socket before retry", "host", host, "path", stalePath)
			if removeErr := os.Remove(stalePath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("connect %s: remove stale control socket %s: %w", host, stalePath, removeErr)
			}
			if retryErr := m.runner.Run(ctx, "ssh", args...); retryErr != nil {
				return fmt.Errorf("connect %s: %w", host, retryErr)
			}
		} else {
			return fmt.Errorf("connect %s: %w", host, err)
		}
	}
	slog.Info("ssh master connected", "host", host)
	return nil
}

// Disconnect stops the SSH master connection.
// If the control socket is already gone (e.g. dropped by network interruption),
// it returns nil — the connection is effectively already disconnected.
func (m *Master) Disconnect(ctx context.Context, host string) error {
	connected, err := m.IsConnected(ctx, host)
	if err != nil {
		return fmt.Errorf("disconnect %s: %w", host, err)
	}
	if !connected {
		slog.Debug("ssh master already disconnected, clearing stale state", "host", host)
		return nil
	}
	cp, err := controlPath()
	if err != nil {
		return fmt.Errorf("disconnect %s: %w", host, err)
	}
	slog.Debug("disconnecting ssh master", "host", host)
	if err := m.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		"-O", "exit", host,
	); err != nil {
		return fmt.Errorf("disconnect %s: %w", host, err)
	}
	slog.Info("ssh master disconnected", "host", host)
	return nil
}

// IsConnected checks whether the ControlMaster socket is active.
func (m *Master) IsConnected(ctx context.Context, host string) (bool, error) {
	cp, err := controlPath()
	if err != nil {
		return false, fmt.Errorf("check connection %s: %w", host, err)
	}
	err = m.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		"-O", "check", host,
	)
	if err != nil {
		if stalePath, ok := staleControlSocketPath(err); ok {
			slog.Warn("removing stale ssh control socket", "host", host, "path", stalePath)
			if removeErr := os.Remove(stalePath); removeErr != nil && !os.IsNotExist(removeErr) {
				return false, fmt.Errorf("check connection %s: remove stale control socket %s: %w", host, stalePath, removeErr)
			}
		}
		slog.Debug("ssh master not connected", "host", host)
		return false, nil
	}
	return true, nil
}

func staleControlSocketPath(err error) (string, bool) {
	msg := err.Error()
	if !strings.Contains(msg, "Connection refused") {
		return "", false
	}

	const prefix = "Control socket connect("
	start := strings.Index(msg, prefix)
	if start == -1 {
		return "", false
	}
	start += len(prefix)

	end := strings.Index(msg[start:], ")")
	if end == -1 {
		return "", false
	}

	path := strings.TrimSpace(msg[start : start+end])
	if path == "" {
		return "", false
	}
	return path, true
}
