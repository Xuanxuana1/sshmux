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

// SOCKS manages SOCKS proxy via SSH DynamicForward.
type SOCKS struct {
	runner ssh.Runner
}

// NewSOCKS creates a SOCKS manager with the given Runner.
func NewSOCKS(r ssh.Runner) *SOCKS {
	return &SOCKS{runner: r}
}

func socksPidDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".sshmux", "socks"), nil
}

func socksPidPath(host string) (string, error) {
	dir, err := socksPidDir()
	if err != nil {
		return "", fmt.Errorf("socks pid path %s: %w", host, err)
	}
	return filepath.Join(dir, host+".pid"), nil
}

func sshControlPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "cm-%C"), nil
}

// Enable starts a background SSH slave session through the ControlMaster with
// a DynamicForward on the given port.
//
// ssh -O forward -D does NOT support DynamicForward (only -L/-R). Instead we
// start a multiplexed slave connection with -D and manage its lifecycle via a
// PID file, mirroring the HTTP proxy pattern.
func (s *SOCKS) Enable(ctx context.Context, host string, port int) error {
	dir, err := socksPidDir()
	if err != nil {
		return fmt.Errorf("socks enable %s: %w", host, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("socks enable %s: create dir: %w", host, err)
	}

	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("socks enable %s: %w", host, err)
	}

	pidFile, err := socksPidPath(host)
	if err != nil {
		return fmt.Errorf("socks enable %s: %w", host, err)
	}

	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "ControlPath="+cp,
		"-o", "ControlMaster=no",
		"-D", fmt.Sprintf("127.0.0.1:%d", port),
		"-NnT",
		host,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	slog.Debug("enabling socks forward", "host", host, "port", port)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("socks enable %s port %d: %w", host, port, err)
	}

	pidStr := fmt.Sprintf("%d", cmd.Process.Pid)
	if err := os.WriteFile(pidFile, []byte(pidStr), 0600); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("socks enable %s: write pid: %w", host, err)
	}
	cmd.Process.Release()

	slog.Info("socks enabled", "host", host, "port", port)
	return nil
}

// Disable kills the background SOCKS slave process for the given host.
func (s *SOCKS) Disable(ctx context.Context, host string, port int) error {
	pidFile, err := socksPidPath(host)
	if err != nil {
		return fmt.Errorf("socks disable %s: %w", host, err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("socks pid file not found, already stopped", "host", host)
			return nil
		}
		return fmt.Errorf("socks disable %s: read pid: %w", host, err)
	}

	pidStr := trimWhitespace(string(data))
	slog.Debug("stopping socks forward", "host", host, "pid", pidStr)
	if err := s.runner.Run(ctx, "kill", pidStr); err != nil {
		slog.Warn("failed to kill socks process", "host", host, "pid", pidStr, "err", err)
	}

	os.Remove(pidFile)
	slog.Info("socks disabled", "host", host, "port", port)
	return nil
}

// SetPort stops the old SOCKS forward and starts a new one on the new port.
func (s *SOCKS) SetPort(ctx context.Context, host string, oldPort, newPort int) error {
	slog.Debug("switching socks port", "host", host, "old_port", oldPort, "new_port", newPort)
	if err := s.Disable(ctx, host, oldPort); err != nil {
		slog.Warn("socks set-port: disable old", "host", host, "err", err)
	}
	return s.Enable(ctx, host, newPort)
}
