package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
)

type checkOnlyRunner struct {
	calls [][]string
}

func (r *checkOnlyRunner) Run(_ context.Context, name string, args ...string) error {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if containsArg(args, "-O") && containsArg(args, "check") {
		return fmt.Errorf("run ssh: Control socket connect(/tmp/cm-dead): Connection refused: exit status 255")
	}
	return nil
}

func (r *checkOnlyRunner) Output(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}

func (r *checkOnlyRunner) RunWithInput(_ context.Context, _ string, _ string, _ ...string) error {
	return nil
}

func TestToggleRemoteProxyRefreshesDeadSSHState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runner := &checkOnlyRunner{}
	m := model{
		hosts: []state.HostState{
			{
				HostAlias:       "a800_lx",
				MasterConnected: true,
			},
		},
		runner: runner,
	}

	updatedAny, _ := m.toggleRemoteProxy()
	updated, ok := updatedAny.(model)
	if !ok {
		t.Fatalf("toggleRemoteProxy returned %T, want model", updatedAny)
	}

	if updated.hosts[0].MasterConnected {
		t.Fatal("MasterConnected stayed true after live SSH check failed")
	}
	if !strings.Contains(updated.statusMsg, "press [c] to reconnect") {
		t.Fatalf("unexpected status message: %q", updated.statusMsg)
	}
	if !updated.statusErr {
		t.Fatal("statusErr = false, want true")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner recorded %d calls, want 1", len(runner.calls))
	}
	if !containsArg(runner.calls[0], "check") {
		t.Fatalf("first call = %v, want ssh -O check only", runner.calls[0])
	}
}

func TestToggleSSH_ExternalSourceDoesNotStartLocalProxy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runner := &ssh.FakeRunner{}
	m := model{
		hosts: []state.HostState{
			{HostAlias: "gpu-box"},
		},
		runner:    runner,
		termProxy: &state.TerminalProxyConfig{},
		globalCfg: &state.GlobalConfig{
			SocksPort:         7897,
			HTTPPort:          7897,
			RemoteSource:      state.RemoteSourceExternal,
			ExternalHTTPAddr:  "127.0.0.1:7897",
			ExternalSOCKSAddr: "127.0.0.1:7897",
		},
	}

	updatedAny, _ := m.toggleSSH()
	updated := updatedAny.(model)

	if !updated.hosts[0].MasterConnected {
		t.Fatal("MasterConnected = false, want true")
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("runner recorded %d calls, want 1", len(runner.Calls))
	}
	if runner.Calls[0].Name != "ssh" || !containsArg(runner.Calls[0].Args, "-MNf") {
		t.Fatalf("first call = %v, want ssh master connect only", runner.Calls[0])
	}
}

func TestToggleRemoteProxy_ExternalSourceUsesConfiguredAddr(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runner := &ssh.FakeRunner{}
	m := model{
		hosts: []state.HostState{
			{HostAlias: "gpu-box", MasterConnected: true},
		},
		runner:    runner,
		termProxy: &state.TerminalProxyConfig{},
		globalCfg: &state.GlobalConfig{
			SocksPort:         7897,
			HTTPPort:          7897,
			RemoteSource:      state.RemoteSourceExternal,
			ExternalHTTPAddr:  "127.0.0.1:7897",
			ExternalSOCKSAddr: "127.0.0.1:7897",
		},
	}

	updatedAny, _ := m.toggleRemoteProxy()
	updated := updatedAny.(model)

	if !updated.hosts[0].RemoteProxyEnabled {
		t.Fatal("RemoteProxyEnabled = false, want true")
	}

	foundForward := false
	for _, call := range runner.Calls {
		if call.Name != "ssh" {
			continue
		}
		args := strings.Join(call.Args, " ")
		if strings.Contains(args, "-O forward") && strings.Contains(args, "-R 7897:") {
			foundForward = true
			break
		}
	}
	if !foundForward {
		t.Fatalf("expected external remote-proxy forward, got calls: %v", runner.Calls)
	}
}

func TestConfirmRemoteSource_ExternalStopsRunningLocalProxies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runner := &ssh.FakeRunner{}
	m := model{
		hosts: []state.HostState{
			{
				HostAlias:    "gpu-box",
				SocksEnabled: true,
				SocksPort:    7897,
				HTTPEnabled:  true,
				HTTPPort:     7897,
			},
		},
		runner: runner,
		globalCfg: &state.GlobalConfig{
			SocksPort:         7897,
			HTTPPort:          7897,
			RemoteSource:      state.RemoteSourceSSHMux,
			ExternalHTTPAddr:  "127.0.0.1:7897",
			ExternalSOCKSAddr: "127.0.0.1:7897",
		},
		remoteSourceMode: state.RemoteSourceExternal,
		remoteSourceInput: [2]string{
			"127.0.0.1:7897",
			"127.0.0.1:7897",
		},
	}

	updatedAny, _ := m.confirmRemoteSource()
	updated := updatedAny.(model)

	if updated.hosts[0].SocksEnabled || updated.hosts[0].HTTPEnabled {
		t.Fatalf("local proxies still enabled after switching external: %+v", updated.hosts[0])
	}
	if updated.globalCfg.RemoteSource != state.RemoteSourceExternal {
		t.Fatalf("RemoteSource = %q, want %q", updated.globalCfg.RemoteSource, state.RemoteSourceExternal)
	}
	if !strings.Contains(updated.statusMsg, "stopped 1 local proxy set") {
		t.Fatalf("unexpected status message: %q", updated.statusMsg)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

var _ tea.Model = model{}
