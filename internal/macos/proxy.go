package macos

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/liuxuan/sshmux/internal/ssh"
)

// Proxy manages macOS system proxy settings via networksetup.
type Proxy struct {
	runner ssh.Runner
}

// NewProxy creates a macOS Proxy manager with the given Runner.
func NewProxy(r ssh.Runner) *Proxy {
	return &Proxy{runner: r}
}

// SetSOCKS enables the SOCKS Firewall Proxy on the given network service.
func (p *Proxy) SetSOCKS(ctx context.Context, service string, host string, port int) error {
	slog.Debug("setting macOS SOCKS proxy", "service", service, "host", host, "port", port)
	if err := p.runner.Run(ctx, "networksetup",
		"-setSOCKSFirewallProxy", service, host, fmt.Sprintf("%d", port),
	); err != nil {
		return fmt.Errorf("set SOCKS proxy on %s: %w", service, err)
	}
	if err := p.runner.Run(ctx, "networksetup",
		"-setSOCKSFirewallProxyState", service, "on",
	); err != nil {
		return fmt.Errorf("enable SOCKS proxy on %s: %w", service, err)
	}
	slog.Info("macOS SOCKS proxy set", "service", service, "port", port)
	return nil
}

// SetWebProxy enables the Web Proxy (HTTP) on the given network service.
func (p *Proxy) SetWebProxy(ctx context.Context, service string, host string, port int) error {
	slog.Debug("setting macOS web proxy", "service", service, "host", host, "port", port)
	if err := p.runner.Run(ctx, "networksetup",
		"-setwebproxy", service, host, fmt.Sprintf("%d", port),
	); err != nil {
		return fmt.Errorf("set web proxy on %s: %w", service, err)
	}
	if err := p.runner.Run(ctx, "networksetup",
		"-setwebproxystate", service, "on",
	); err != nil {
		return fmt.Errorf("enable web proxy on %s: %w", service, err)
	}
	slog.Info("macOS web proxy set", "service", service, "port", port)
	return nil
}

// SetSecureWebProxy enables the Secure Web Proxy (HTTPS) on the given network service.
func (p *Proxy) SetSecureWebProxy(ctx context.Context, service string, host string, port int) error {
	slog.Debug("setting macOS secure web proxy", "service", service, "host", host, "port", port)
	if err := p.runner.Run(ctx, "networksetup",
		"-setsecurewebproxy", service, host, fmt.Sprintf("%d", port),
	); err != nil {
		return fmt.Errorf("set secure web proxy on %s: %w", service, err)
	}
	if err := p.runner.Run(ctx, "networksetup",
		"-setsecurewebproxystate", service, "on",
	); err != nil {
		return fmt.Errorf("enable secure web proxy on %s: %w", service, err)
	}
	slog.Info("macOS secure web proxy set", "service", service, "port", port)
	return nil
}

// ClearAll disables all proxy settings on the given network service.
func (p *Proxy) ClearAll(ctx context.Context, service string) error {
	slog.Debug("clearing all macOS proxies", "service", service)

	var firstErr error
	if err := p.runner.Run(ctx, "networksetup",
		"-setSOCKSFirewallProxyState", service, "off",
	); err != nil {
		firstErr = fmt.Errorf("disable SOCKS proxy on %s: %w", service, err)
		slog.Warn("failed to disable SOCKS proxy", "service", service, "err", err)
	}

	if err := p.runner.Run(ctx, "networksetup",
		"-setwebproxystate", service, "off",
	); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("disable web proxy on %s: %w", service, err)
		}
		slog.Warn("failed to disable web proxy", "service", service, "err", err)
	}

	if err := p.runner.Run(ctx, "networksetup",
		"-setsecurewebproxystate", service, "off",
	); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("disable secure web proxy on %s: %w", service, err)
		}
		slog.Warn("failed to disable secure web proxy", "service", service, "err", err)
	}

	if firstErr != nil {
		return firstErr
	}
	slog.Info("macOS proxies cleared", "service", service)
	return nil
}
