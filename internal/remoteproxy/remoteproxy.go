package remoteproxy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
	"github.com/liuxuan/sshmux/internal/termproxy"
)

// RemoteProxy manages SSH reverse port forwarding for proxy access on remote hosts.
type RemoteProxy struct {
	runner ssh.Runner
}

// NewRemoteProxy creates a RemoteProxy manager with the given Runner.
func NewRemoteProxy(r ssh.Runner) *RemoteProxy {
	return &RemoteProxy{runner: r}
}

func sshControlPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "cm-%C"), nil
}

// Enable sets up reverse port forwarding and writes proxy.env on the remote host.
func (rp *RemoteProxy) Enable(ctx context.Context, host string, httpAddr, socksAddr string) error {
	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("remote-proxy enable %s: %w", host, err)
	}

	// Determine which forwards to set up
	forwards := uniqueForwards(httpAddr, socksAddr)

	for _, fwd := range forwards {
		slog.Debug("setting up reverse forward", "host", host, "remote_port", fwd.remotePort, "local_addr", fwd.localAddr)
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			"-O", "forward",
			"-R", fmt.Sprintf("%s:%s", fwd.remotePort, fwd.localAddr),
			host,
		); err != nil {
			return fmt.Errorf("remote-proxy enable %s: forward %s: %w", host, fwd.remotePort, err)
		}
	}

	// Write proxy.env on remote
	envContent := BuildRemoteEnvContent(httpAddr, socksAddr)
	writeCmd := fmt.Sprintf("mkdir -p ~/.sshmux && cat > ~/.sshmux/proxy.env << 'SSHMUX_EOF'\n%sSSHMUX_EOF", envContent)

	slog.Debug("writing remote proxy.env", "host", host)
	if err := rp.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, writeCmd,
	); err != nil {
		return fmt.Errorf("remote-proxy enable %s: write proxy.env: %w", host, err)
	}

	// Patch remote shell profiles (.bashrc and .zshrc, idempotent).
	// Uses Python via heredoc stdin to avoid shell quoting issues.
	// Handles both fresh installs and upgrade from the old single-line format.
	// The if/else block actively unsets proxy vars when proxy.env is absent,
	// so inherited env vars (e.g. from VSCode server forks) are cleared correctly.
	patchCmd := `python3 - << 'PYEOF'
import os
OLD_LINE = '[ -f "$HOME/.sshmux/proxy.env" ] && source "$HOME/.sshmux/proxy.env"'
NEW_BLOCK = '''# sshmux: terminal proxy
if [ -f "$HOME/.sshmux/proxy.env" ]; then
  source "$HOME/.sshmux/proxy.env"
else
  unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY no_proxy NO_PROXY
fi'''
for rcfile in ['~/.bashrc', '~/.zshrc']:
    p = os.path.expanduser(rcfile)
    if not os.path.exists(p):
        continue
    c = open(p).read()
    old_block = '# sshmux: terminal proxy\n' + OLD_LINE
    if old_block in c:
        open(p, 'w').write(c.replace(old_block, NEW_BLOCK))
    elif '# sshmux: terminal proxy' not in c:
        open(p, 'a').write('\n' + NEW_BLOCK + '\n')
PYEOF`
	if err := rp.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, patchCmd,
	); err != nil {
		slog.Warn("failed to patch remote shell profiles", "host", host, "err", err)
	}

	slog.Info("remote-proxy enabled", "host", host, "http_addr", httpAddr, "socks_addr", socksAddr)
	return nil
}

// Disable removes the reverse port forwarding and deletes remote proxy.env.
func (rp *RemoteProxy) Disable(ctx context.Context, host string, httpAddr, socksAddr string) error {
	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("remote-proxy disable %s: %w", host, err)
	}

	// Cancel forwards
	forwards := uniqueForwards(httpAddr, socksAddr)
	for _, fwd := range forwards {
		slog.Debug("canceling reverse forward", "host", host, "remote_port", fwd.remotePort, "local_addr", fwd.localAddr)
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			"-O", "cancel",
			"-R", fmt.Sprintf("%s:%s", fwd.remotePort, fwd.localAddr),
			host,
		); err != nil {
			slog.Warn("failed to cancel reverse forward", "host", host, "port", fwd.remotePort, "err", err)
		}
	}

	// Delete remote proxy.env
	if err := rp.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, "rm -f ~/.sshmux/proxy.env",
	); err != nil {
		slog.Warn("failed to delete remote proxy.env", "host", host, "err", err)
	}

	slog.Info("remote-proxy disabled", "host", host)
	return nil
}

// SetPort changes the remote proxy port using safe switch pattern.
func (rp *RemoteProxy) SetPort(ctx context.Context, host string, oldHTTPAddr, oldSOCKSAddr, newHTTPAddr, newSOCKSAddr string) error {
	slog.Debug("switching remote-proxy ports", "host", host)

	// Enable new forwards first
	if err := rp.Enable(ctx, host, newHTTPAddr, newSOCKSAddr); err != nil {
		return fmt.Errorf("remote-proxy set-port %s: %w", host, err)
	}

	// Cancel old forwards (best-effort)
	cp, err := sshControlPath()
	if err != nil {
		return nil // New forwards are up, old cleanup is best-effort
	}

	oldForwards := uniqueForwards(oldHTTPAddr, oldSOCKSAddr)
	for _, fwd := range oldForwards {
		_ = rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			"-O", "cancel",
			"-R", fmt.Sprintf("%s:%s", fwd.remotePort, fwd.localAddr),
			host,
		)
	}

	return nil
}

// ResolveAddrs resolves the effective http and socks addresses,
// falling back to terminal-proxy config if not explicitly provided.
func ResolveAddrs(httpAddr, socksAddr string) (string, string) {
	if httpAddr != "" || socksAddr != "" {
		return httpAddr, socksAddr
	}
	cfg, err := state.LoadTerminalProxy()
	if err != nil {
		return httpAddr, socksAddr
	}
	if httpAddr == "" {
		httpAddr = cfg.HTTPAddr
	}
	if socksAddr == "" {
		socksAddr = cfg.SOCKSAddr
	}
	return httpAddr, socksAddr
}

type forward struct {
	remotePort string
	localAddr  string
}

// uniqueForwards deduplicates forwards when http and socks share the same address.
func uniqueForwards(httpAddr, socksAddr string) []forward {
	seen := make(map[string]bool)
	var forwards []forward

	addrs := []string{httpAddr, socksAddr}
	for _, addr := range addrs {
		if addr == "" {
			continue
		}
		// The remote port is the port part of the address,
		// and the local address is the full addr
		key := addr
		if seen[key] {
			continue
		}
		seen[key] = true

		// Extract port from addr (host:port)
		port := extractPort(addr)
		forwards = append(forwards, forward{
			remotePort: port,
			localAddr:  addr,
		})
	}

	return forwards
}

// extractPort extracts the port from a host:port string.
func extractPort(addr string) string {
	parts := strings.SplitN(addr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return addr
}

// BuildRemoteEnvContent generates the remote proxy.env content.
// On the remote, the proxy addresses point to 127.0.0.1:<port> since
// the SSH reverse forward maps the remote port to the local address.
func BuildRemoteEnvContent(httpAddr, socksAddr string) string {
	// Reuse the termproxy builder, but with remote-appropriate addresses.
	// On the remote side, the proxy is accessible at 127.0.0.1:<remote_port>
	// where remote_port is the port from the address.
	remoteHTTP := ""
	if httpAddr != "" {
		remoteHTTP = "127.0.0.1:" + extractPort(httpAddr)
	}
	remoteSOCKS := ""
	if socksAddr != "" {
		remoteSOCKS = "127.0.0.1:" + extractPort(socksAddr)
	}
	return termproxy.BuildEnvContent(remoteHTTP, remoteSOCKS)
}
