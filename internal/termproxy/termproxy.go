package termproxy

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/liuxuan/sshmux/internal/state"
)

const (
	markerLine    = "# sshmux: terminal proxy"
	oldSourceLine = `[ -f "$HOME/.sshmux/proxy.env" ] && source "$HOME/.sshmux/proxy.env"`
	sourceLine    = `if [ -f "$HOME/.sshmux/proxy.env" ]; then
  source "$HOME/.sshmux/proxy.env"
else
  unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY no_proxy NO_PROXY
fi`
)

// Enable turns on terminal-proxy: saves config, writes proxy.env, patches shell profiles.
func Enable(cfg *state.TerminalProxyConfig) error {
	if cfg.HTTPAddr == "" && cfg.SOCKSAddr == "" {
		return fmt.Errorf("terminal-proxy enable: at least one of --http or --socks must be specified")
	}

	cfg.Enabled = true
	if err := state.SaveTerminalProxy(cfg); err != nil {
		return fmt.Errorf("terminal-proxy enable: %w", err)
	}

	if err := WriteEnvFile(cfg); err != nil {
		return fmt.Errorf("terminal-proxy enable: %w", err)
	}

	if err := PatchProfile(); err != nil {
		return fmt.Errorf("terminal-proxy enable: %w", err)
	}

	slog.Info("terminal-proxy enabled", "http_addr", cfg.HTTPAddr, "socks_addr", cfg.SOCKSAddr)
	return nil
}

// Disable turns off terminal-proxy: removes proxy.env, saves disabled config.
func Disable() error {
	cfg, err := state.LoadTerminalProxy()
	if err != nil {
		return fmt.Errorf("terminal-proxy disable: %w", err)
	}

	cfg.Enabled = false
	if err := state.SaveTerminalProxy(cfg); err != nil {
		return fmt.Errorf("terminal-proxy disable: %w", err)
	}

	envPath, err := envFilePath()
	if err != nil {
		return fmt.Errorf("terminal-proxy disable: %w", err)
	}

	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("terminal-proxy disable: remove proxy.env: %w", err)
	}

	slog.Info("terminal-proxy disabled")
	return nil
}

// envFilePath returns ~/.sshmux/proxy.env.
func envFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".sshmux", "proxy.env"), nil
}

// WriteEnvFile generates the proxy.env file from the config.
func WriteEnvFile(cfg *state.TerminalProxyConfig) error {
	content := BuildEnvContent(cfg.HTTPAddr, cfg.SOCKSAddr)

	envPath, err := envFilePath()
	if err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	dir := filepath.Dir(envPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create dir for proxy.env: %w", err)
	}

	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write proxy.env: %w", err)
	}

	slog.Debug("proxy.env written", "path", envPath)
	return nil
}

// BuildEnvContent generates the content of proxy.env given http and socks addresses.
func BuildEnvContent(httpAddr, socksAddr string) string {
	var b strings.Builder

	effectiveHTTP := httpAddr
	if effectiveHTTP == "" && socksAddr != "" {
		effectiveHTTP = socksAddr
	}

	if effectiveHTTP != "" {
		fmt.Fprintf(&b, "export http_proxy=\"http://%s\"\n", effectiveHTTP)
		fmt.Fprintf(&b, "export https_proxy=\"http://%s\"\n", effectiveHTTP)
		fmt.Fprintf(&b, "export HTTP_PROXY=\"http://%s\"\n", effectiveHTTP)
		fmt.Fprintf(&b, "export HTTPS_PROXY=\"http://%s\"\n", effectiveHTTP)
	}

	if socksAddr != "" {
		fmt.Fprintf(&b, "export all_proxy=\"socks5h://%s\"\n", socksAddr)
		fmt.Fprintf(&b, "export ALL_PROXY=\"socks5h://%s\"\n", socksAddr)
	}

	b.WriteString("export no_proxy=\"localhost,127.0.0.1\"\n")
	b.WriteString("export NO_PROXY=\"localhost,127.0.0.1\"\n")

	return b.String()
}

// PatchProfile appends the source line to ~/.zshrc and ~/.bash_profile (idempotent).
func PatchProfile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	profiles := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bash_profile"),
	}

	for _, profile := range profiles {
		if err := patchSingleProfile(profile); err != nil {
			return fmt.Errorf("patch %s: %w", profile, err)
		}
	}
	return nil
}

// patchSingleProfile appends the source block to a single profile file if not present.
// If the old single-line format is detected, it is upgraded to the if/else format in-place.
func patchSingleProfile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read profile: %w", err)
	}

	content := string(data)
	newBlock := markerLine + "\n" + sourceLine + "\n"
	oldBlock := markerLine + "\n" + oldSourceLine + "\n"

	if strings.Contains(content, markerLine) {
		if strings.Contains(content, oldSourceLine) {
			// Upgrade: old single-line format → new if/else format.
			// This ensures inherited env vars are actively unset when proxy.env is absent.
			newContent := strings.Replace(content, oldBlock, newBlock, 1)
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("upgrade profile: %w", err)
			}
			slog.Debug("profile upgraded to unset format", "path", path)
		} else {
			slog.Debug("profile already patched", "path", path)
		}
		return nil
	}

	// Not patched yet — append.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open profile for append: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + newBlock); err != nil {
		return fmt.Errorf("append to profile: %w", err)
	}

	slog.Debug("profile patched", "path", path)
	return nil
}

// PatchProfileAt patches a specific file path (for testing).
func PatchProfileAt(path string) error {
	return patchSingleProfile(path)
}
