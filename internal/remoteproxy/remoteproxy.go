package remoteproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
	"github.com/liuxuan/sshmux/internal/termproxy"
)

// Options configures how remote-proxy exposes the forwarded proxy on the
// remote machine. The SSH reverse forward always remains loopback-bound for the
// remote shell; Docker-friendly access is provided by an extra relay when a
// non-loopback bind address is resolved.
type Options struct {
	HTTPAddr      string
	SOCKSAddr     string
	BindAddress   string
	DockerGateway bool
	LoopbackOnly  bool
}

// Activation captures the effective remote-proxy endpoints after bind-address
// resolution. HTTPAddr and SOCKSAddr are the local source addresses on macOS;
// Exposed*Addr are the remote endpoints that Docker containers can use.
type Activation struct {
	HTTPAddr         string
	SOCKSAddr        string
	BindAddress      string
	ExposedHTTPAddr  string
	ExposedSOCKSAddr string
}

// ShellHTTPAddr returns the loopback endpoint used in the remote shell env.
func (a Activation) ShellHTTPAddr() string {
	return remoteLoopbackAddr(effectiveHTTPAddr(a.HTTPAddr, a.SOCKSAddr))
}

// ShellSOCKSAddr returns the loopback SOCKS endpoint used in the remote shell env.
func (a Activation) ShellSOCKSAddr() string {
	return remoteLoopbackAddr(a.SOCKSAddr)
}

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

// Enable sets up reverse port forwarding, writes proxy.env on the remote host,
// and, when requested, starts a Docker-friendly relay bound to a host-reachable
// address such as docker0.
func (rp *RemoteProxy) Enable(ctx context.Context, host string, opts Options) (Activation, error) {
	cp, err := sshControlPath()
	if err != nil {
		return Activation{}, fmt.Errorf("remote-proxy enable %s: %w", host, err)
	}

	activation, err := rp.resolveActivation(ctx, host, cp, opts)
	if err != nil {
		return Activation{}, fmt.Errorf("remote-proxy enable %s: %w", host, err)
	}

	forwards := uniqueForwards(activation.HTTPAddr, activation.SOCKSAddr)
	for _, fwd := range forwards {
		slog.Debug("setting up reverse forward", "host", host, "remote_port", fwd.remotePort, "local_addr", fwd.localAddr)
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			"-O", "forward",
			"-R", fmt.Sprintf("%s:%s", fwd.remotePort, fwd.localAddr),
			host,
		); err != nil {
			return Activation{}, fmt.Errorf("remote-proxy enable %s: forward %s: %w", host, fwd.remotePort, err)
		}
	}

	if err := rp.writeRemoteEnv(ctx, host, cp, activation.HTTPAddr, activation.SOCKSAddr); err != nil {
		return Activation{}, fmt.Errorf("remote-proxy enable %s: %w", host, err)
	}

	if activation.BindAddress != "" {
		if err := rp.ensureRemoteRelays(ctx, host, cp, activation.BindAddress, uniquePortsFromForwards(forwards)); err != nil {
			_ = rp.cancelForwards(ctx, host, cp, forwards)
			_ = rp.deleteRemoteEnv(ctx, host, cp)
			return Activation{}, fmt.Errorf("remote-proxy enable %s: docker relay: %w", host, err)
		}
	}

	slog.Info(
		"remote-proxy enabled",
		"host", host,
		"http_addr", activation.HTTPAddr,
		"socks_addr", activation.SOCKSAddr,
		"bind_address", activation.BindAddress,
	)
	return activation, nil
}

// WriteEnvOnly writes proxy.env and patches shell profiles on the remote host
// without setting up a new port forward. Use when a port forward is already
// active on the same physical server via another SSH session.
func (rp *RemoteProxy) WriteEnvOnly(ctx context.Context, host string, opts Options) (Activation, error) {
	cp, err := sshControlPath()
	if err != nil {
		return Activation{}, fmt.Errorf("remote-proxy write-env %s: %w", host, err)
	}
	activation, err := rp.resolveActivation(ctx, host, cp, opts)
	if err != nil {
		return Activation{}, fmt.Errorf("remote-proxy write-env %s: %w", host, err)
	}
	if err := rp.writeRemoteEnv(ctx, host, cp, activation.HTTPAddr, activation.SOCKSAddr); err != nil {
		return Activation{}, fmt.Errorf("remote-proxy write-env %s: %w", host, err)
	}
	slog.Info(
		"remote-proxy env written (shared tunnel)",
		"host", host,
		"http_addr", activation.HTTPAddr,
		"socks_addr", activation.SOCKSAddr,
		"bind_address", activation.BindAddress,
	)
	return activation, nil
}

// Disable removes the reverse port forwarding, stops any Docker-friendly relay,
// and deletes remote proxy.env.
func (rp *RemoteProxy) Disable(ctx context.Context, host string, activation Activation) error {
	cp, err := sshControlPath()
	if err != nil {
		return fmt.Errorf("remote-proxy disable %s: %w", host, err)
	}

	forwards := uniqueForwards(activation.HTTPAddr, activation.SOCKSAddr)
	if err := rp.cancelForwards(ctx, host, cp, forwards); err != nil {
		slog.Warn("failed to cancel one or more reverse forwards", "host", host, "err", err)
	}

	if activation.BindAddress != "" {
		if err := rp.stopRemoteRelays(ctx, host, cp, uniquePortsFromForwards(forwards)); err != nil {
			slog.Warn("failed to stop one or more docker relays", "host", host, "err", err)
		}
	}

	if err := rp.deleteRemoteEnv(ctx, host, cp); err != nil {
		slog.Warn("failed to delete remote proxy.env", "host", host, "err", err)
	}

	slog.Info("remote-proxy disabled", "host", host)
	return nil
}

// SetPort changes the remote proxy configuration using a safe switch pattern.
func (rp *RemoteProxy) SetPort(ctx context.Context, host string, old Activation, opts Options) (Activation, error) {
	slog.Debug("switching remote-proxy configuration", "host", host)

	newActivation, err := rp.Enable(ctx, host, opts)
	if err != nil {
		return Activation{}, fmt.Errorf("remote-proxy set-port %s: %w", host, err)
	}

	cp, err := sshControlPath()
	if err != nil {
		return newActivation, nil
	}

	oldForwards := uniqueForwards(old.HTTPAddr, old.SOCKSAddr)
	newForwards := uniqueForwards(newActivation.HTTPAddr, newActivation.SOCKSAddr)
	if err := rp.cancelForwards(ctx, host, cp, subtractForwards(oldForwards, newForwards)); err != nil {
		slog.Warn("failed to clean up old reverse forwards", "host", host, "err", err)
	}

	if old.BindAddress != "" && newActivation.BindAddress == "" {
		if err := rp.stopRemoteRelays(ctx, host, cp, uniquePortsFromForwards(oldForwards)); err != nil {
			slog.Warn("failed to stop old docker relays", "host", host, "err", err)
		}
	}
	if old.BindAddress != "" && newActivation.BindAddress != "" {
		oldPorts := uniquePortsFromForwards(oldForwards)
		newPorts := uniquePortsFromForwards(newForwards)
		if err := rp.stopRemoteRelays(ctx, host, cp, subtractPorts(oldPorts, newPorts)); err != nil {
			slog.Warn("failed to stop stale docker relays", "host", host, "err", err)
		}
	}

	return newActivation, nil
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

func (rp *RemoteProxy) resolveActivation(ctx context.Context, host, cp string, opts Options) (Activation, error) {
	if err := validateOptions(opts); err != nil {
		return Activation{}, err
	}

	effectiveHTTP := effectiveHTTPAddr(opts.HTTPAddr, opts.SOCKSAddr)
	activation := Activation{
		HTTPAddr:         opts.HTTPAddr,
		SOCKSAddr:        opts.SOCKSAddr,
		ExposedHTTPAddr:  remoteLoopbackAddr(effectiveHTTP),
		ExposedSOCKSAddr: remoteLoopbackAddr(opts.SOCKSAddr),
	}

	if opts.LoopbackOnly {
		return activation, nil
	}

	bindAddr := opts.BindAddress
	if bindAddr == "" && opts.DockerGateway {
		detected, err := rp.detectDockerGateway(ctx, host, cp)
		if err != nil {
			return Activation{}, fmt.Errorf("detect docker gateway: %w", err)
		}
		bindAddr = detected
	}
	if bindAddr == "" {
		return activation, nil
	}

	activation.BindAddress = bindAddr
	if effectiveHTTP != "" {
		activation.ExposedHTTPAddr = net.JoinHostPort(bindAddr, extractPort(effectiveHTTP))
	}
	if opts.SOCKSAddr != "" {
		activation.ExposedSOCKSAddr = net.JoinHostPort(bindAddr, extractPort(opts.SOCKSAddr))
	}
	return activation, nil
}

// writeRemoteEnv writes proxy.env and patches remote shell profiles.
func (rp *RemoteProxy) writeRemoteEnv(ctx context.Context, host, cp, httpAddr, socksAddr string) error {
	envContent := BuildRemoteEnvContent(httpAddr, socksAddr)
	writeCmd := fmt.Sprintf("mkdir -p ~/.sshmux && cat > ~/.sshmux/proxy.env << 'SSHMUX_EOF'\n%sSSHMUX_EOF", envContent)

	slog.Debug("writing remote proxy.env", "host", host)
	if err := rp.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, writeCmd,
	); err != nil {
		return fmt.Errorf("write proxy.env: %w", err)
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
        open(p, 'a').close()
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

	return nil
}

func (rp *RemoteProxy) deleteRemoteEnv(ctx context.Context, host, cp string) error {
	return rp.runner.Run(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, "rm -f ~/.sshmux/proxy.env",
	)
}

func (rp *RemoteProxy) cancelForwards(ctx context.Context, host, cp string, forwards []forward) error {
	var firstErr error
	for _, fwd := range forwards {
		slog.Debug("canceling reverse forward", "host", host, "remote_port", fwd.remotePort, "local_addr", fwd.localAddr)
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			"-O", "cancel",
			"-R", fmt.Sprintf("%s:%s", fwd.remotePort, fwd.localAddr),
			host,
		); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Warn("failed to cancel reverse forward", "host", host, "port", fwd.remotePort, "err", err)
		}
	}
	return firstErr
}

func (rp *RemoteProxy) detectDockerGateway(ctx context.Context, host, cp string) (string, error) {
	cmd := `sh -lc 'ip -o -4 addr show docker0 2>/dev/null | awk "{print \$4}" | cut -d/ -f1 | head -n1 || true'`
	out, err := rp.runner.Output(ctx, "ssh",
		"-o", "ControlPath="+cp,
		host, cmd,
	)
	if err != nil {
		return "", err
	}
	addr := strings.TrimSpace(string(out))
	if addr == "" {
		return "", nil
	}
	if net.ParseIP(addr) == nil || net.ParseIP(addr).To4() == nil {
		return "", fmt.Errorf("invalid docker gateway %q", addr)
	}
	return addr, nil
}

func (rp *RemoteProxy) ensureRemoteRelays(ctx context.Context, host, cp, bindAddr string, ports []string) error {
	for _, port := range ports {
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			host, buildRelayStartCommand(bindAddr, port),
		); err != nil {
			return fmt.Errorf("port %s: %w", port, err)
		}
	}
	return nil
}

func (rp *RemoteProxy) stopRemoteRelays(ctx context.Context, host, cp string, ports []string) error {
	var firstErr error
	for _, port := range ports {
		if err := rp.runner.Run(ctx, "ssh",
			"-o", "ControlPath="+cp,
			host, buildRelayStopCommand(port),
		); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func buildRelayStartCommand(bindAddr, port string) string {
	return fmt.Sprintf(`python3 - << 'PYEOF'
import os
import signal
import socket
import subprocess
import sys
import time

bind = %q
port = int(%q)
target_host = "127.0.0.1"
target_port = int(%q)
base = os.path.expanduser("~/.sshmux")
pid_path = os.path.join(base, "remote-proxy-relay-%s.pid")
meta_path = os.path.join(base, "remote-proxy-relay-%s.meta")
meta = f"{bind}|{port}|{target_host}:{target_port}"

def alive(pid):
    if not pid:
        return False
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    return True

os.makedirs(base, exist_ok=True)
current_pid = None
if os.path.exists(pid_path):
    try:
        current_pid = int(open(pid_path).read().strip())
    except Exception:
        current_pid = None

current_meta = ""
if os.path.exists(meta_path):
    try:
        current_meta = open(meta_path).read().strip()
    except Exception:
        current_meta = ""

if current_pid and alive(current_pid) and current_meta == meta:
    sys.exit(0)

if current_pid and alive(current_pid):
    try:
        os.kill(current_pid, signal.SIGTERM)
    except OSError:
        pass
    for _ in range(20):
        if not alive(current_pid):
            break
        time.sleep(0.1)

server = r"""
import socket
import sys
import threading

bind_host = sys.argv[1]
bind_port = int(sys.argv[2])
target_host = "127.0.0.1"
target_port = int(sys.argv[3])

listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
listener.bind((bind_host, bind_port))
listener.listen(128)

def pipe(src, dst):
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            dst.sendall(data)
    except OSError:
        pass
    finally:
        for sock in (src, dst):
            try:
                sock.close()
            except OSError:
                pass

while True:
    client, _ = listener.accept()
    upstream = socket.create_connection((target_host, target_port))
    threading.Thread(target=pipe, args=(client, upstream), daemon=True).start()
    threading.Thread(target=pipe, args=(upstream, client), daemon=True).start()
"""

proc = subprocess.Popen(
    [sys.executable, "-u", "-c", server, bind, str(port), str(target_port)],
    stdin=subprocess.DEVNULL,
    stdout=subprocess.DEVNULL,
    stderr=subprocess.DEVNULL,
    start_new_session=True,
)

verify_host = bind if bind != "0.0.0.0" else "127.0.0.1"
for _ in range(20):
    if proc.poll() is not None:
        raise SystemExit("relay exited early")
    probe = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    probe.settimeout(0.2)
    try:
        probe.connect((verify_host, port))
        probe.close()
        break
    except OSError:
        time.sleep(0.1)
    finally:
        try:
            probe.close()
        except OSError:
            pass
else:
    try:
        proc.terminate()
    except OSError:
        pass
    raise SystemExit("relay did not become ready")

with open(pid_path, "w") as f:
    f.write(str(proc.pid))
with open(meta_path, "w") as f:
    f.write(meta)
PYEOF`, bindAddr, port, port, port, port)
}

func buildRelayStopCommand(port string) string {
	return fmt.Sprintf(`python3 - << 'PYEOF'
import os
import signal
import time

base = os.path.expanduser("~/.sshmux")
pid_path = os.path.join(base, "remote-proxy-relay-%s.pid")
meta_path = os.path.join(base, "remote-proxy-relay-%s.meta")

pid = None
if os.path.exists(pid_path):
    try:
        pid = int(open(pid_path).read().strip())
    except Exception:
        pid = None

if pid:
    try:
        os.kill(pid, signal.SIGTERM)
    except OSError:
        pid = None

if pid:
    for _ in range(20):
        try:
            os.kill(pid, 0)
            time.sleep(0.1)
        except OSError:
            break

for path in (pid_path, meta_path):
    try:
        os.remove(path)
    except FileNotFoundError:
        pass
PYEOF`, port, port)
}

func validateOptions(opts Options) error {
	if opts.LoopbackOnly && opts.BindAddress != "" {
		return fmt.Errorf("--bind-address and --loopback-only cannot be used together")
	}
	if opts.LoopbackOnly && opts.DockerGateway {
		return fmt.Errorf("--docker-gateway and --loopback-only cannot be used together")
	}
	if opts.BindAddress == "" {
		return nil
	}
	ip := net.ParseIP(opts.BindAddress)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid IPv4 bind address %q", opts.BindAddress)
	}
	return nil
}

func effectiveHTTPAddr(httpAddr, socksAddr string) string {
	if httpAddr != "" {
		return httpAddr
	}
	return socksAddr
}

func remoteLoopbackAddr(addr string) string {
	if addr == "" {
		return ""
	}
	return "127.0.0.1:" + extractPort(addr)
}

// uniqueForwards deduplicates forwards when http and socks share the same port.
// It auto-detects whether the proxy is listening on 127.0.0.1 or localhost.
func uniqueForwards(httpAddr, socksAddr string) []forward {
	seenPorts := make(map[string]bool)
	var forwards []forward

	for _, addr := range []string{effectiveHTTPAddr(httpAddr, socksAddr), socksAddr} {
		if addr == "" {
			continue
		}
		port := extractPort(addr)
		if port == "" || seenPorts[port] {
			continue
		}
		seenPorts[port] = true
		forwards = append(forwards, forward{
			remotePort: port,
			localAddr:  detectLocalAddr(port),
		})
	}

	return forwards
}

// detectLocalAddr detects whether the proxy is listening on 127.0.0.1 or localhost.
// It tries 127.0.0.1 first, then falls back to localhost.
func detectLocalAddr(port string) string {
	if isPortListening("127.0.0.1", port) {
		return "127.0.0.1:" + port
	}
	if isPortListening("localhost", port) {
		return "localhost:" + port
	}
	slog.Debug("proxy port not detected on 127.0.0.1 or localhost", "port", port)
	return "localhost:" + port
}

// isPortListening checks if a port is listening on the given host.
func isPortListening(host, port string) bool {
	addr := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// extractPort extracts the port from a host:port string.
func extractPort(addr string) string {
	parts := strings.SplitN(addr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return addr
}

func uniquePortsFromForwards(forwards []forward) []string {
	seen := make(map[string]bool)
	var ports []string
	for _, fwd := range forwards {
		if fwd.remotePort == "" || seen[fwd.remotePort] {
			continue
		}
		seen[fwd.remotePort] = true
		ports = append(ports, fwd.remotePort)
	}
	return ports
}

func subtractForwards(oldForwards, newForwards []forward) []forward {
	keep := make(map[string]bool)
	for _, fwd := range newForwards {
		keep[fwd.remotePort+"|"+fwd.localAddr] = true
	}
	var result []forward
	for _, fwd := range oldForwards {
		if keep[fwd.remotePort+"|"+fwd.localAddr] {
			continue
		}
		result = append(result, fwd)
	}
	return result
}

func subtractPorts(oldPorts, newPorts []string) []string {
	keep := make(map[string]bool)
	for _, port := range newPorts {
		keep[port] = true
	}
	var result []string
	for _, port := range oldPorts {
		if keep[port] {
			continue
		}
		result = append(result, port)
	}
	return result
}

// BuildRemoteEnvContent generates the remote proxy.env content.
// On the remote, the proxy addresses point to 127.0.0.1:<port> since
// the SSH reverse forward maps the remote port to the local address.
func BuildRemoteEnvContent(httpAddr, socksAddr string) string {
	remoteHTTP := ""
	if effective := effectiveHTTPAddr(httpAddr, socksAddr); effective != "" {
		remoteHTTP = remoteLoopbackAddr(effective)
	}
	remoteSOCKS := remoteLoopbackAddr(socksAddr)
	return termproxy.BuildEnvContent(remoteHTTP, remoteSOCKS)
}
