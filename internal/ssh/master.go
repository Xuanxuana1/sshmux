package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	slog.Debug("connecting ssh master", "host", host, "control_path", cp)
	if err := m.runner.Run(ctx, "ssh",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath="+cp,
		"-o", "ControlPersist=yes",
		"-MNf", host,
	); err != nil {
		return fmt.Errorf("connect %s: %w", host, err)
	}
	slog.Info("ssh master connected", "host", host)
	return nil
}

// Disconnect stops the SSH master connection.
func (m *Master) Disconnect(ctx context.Context, host string) error {
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
		slog.Debug("ssh master not connected", "host", host)
		return false, nil
	}
	return true, nil
}
