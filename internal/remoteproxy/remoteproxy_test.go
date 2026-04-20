package remoteproxy

import (
	"context"
	"strings"
	"testing"

	"github.com/liuxuan/sshmux/internal/ssh"
)

func TestEnable_ForwardCommandArgs(t *testing.T) {
	tests := []struct {
		name      string
		httpAddr  string
		socksAddr string
		wantPorts []string
	}{
		{
			name:      "http only",
			httpAddr:  "127.0.0.1:7897",
			socksAddr: "",
			wantPorts: []string{"7897"},
		},
		{
			name:      "socks only",
			httpAddr:  "",
			socksAddr: "127.0.0.1:1080",
			wantPorts: []string{"1080"},
		},
		{
			name:      "same port deduplicates",
			httpAddr:  "127.0.0.1:7897",
			socksAddr: "localhost:7897",
			wantPorts: []string{"7897"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &ssh.FakeRunner{}
			rp := NewRemoteProxy(fake)

			_, err := rp.Enable(context.Background(), "testhost", Options{
				HTTPAddr:  tt.httpAddr,
				SOCKSAddr: tt.socksAddr,
			})
			if err != nil {
				t.Fatalf("Enable returned error: %v", err)
			}

			foundPorts := make(map[string]bool)
			for _, c := range fake.Calls {
				if c.Name != "ssh" || !contains(c.Args, "-O") || !contains(c.Args, "forward") {
					continue
				}
				for i, arg := range c.Args {
					if arg == "-R" && i+1 < len(c.Args) {
						parts := strings.SplitN(c.Args[i+1], ":", 2)
						if len(parts) >= 1 {
							foundPorts[parts[0]] = true
						}
					}
				}
			}

			for _, port := range tt.wantPorts {
				if !foundPorts[port] {
					t.Errorf("expected forward for port %s, got ports: %v", port, foundPorts)
				}
			}
		})
	}
}

func TestEnable_DetectsDockerGatewayAndStartsRelay(t *testing.T) {
	fake := &ssh.FakeRunner{OutputData: []byte("172.17.0.1\n")}
	rp := NewRemoteProxy(fake)

	activation, err := rp.Enable(context.Background(), "testhost", Options{
		HTTPAddr:      "127.0.0.1:7897",
		SOCKSAddr:     "127.0.0.1:7897",
		DockerGateway: true,
	})
	if err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if activation.BindAddress != "172.17.0.1" {
		t.Fatalf("BindAddress = %q, want %q", activation.BindAddress, "172.17.0.1")
	}
	if activation.ExposedHTTPAddr != "172.17.0.1:7897" {
		t.Fatalf("ExposedHTTPAddr = %q, want %q", activation.ExposedHTTPAddr, "172.17.0.1:7897")
	}

	foundRelayStart := false
	for _, c := range fake.Calls {
		if c.Name != "ssh" || len(c.Args) == 0 {
			continue
		}
		cmd := c.Args[len(c.Args)-1]
		if strings.Contains(cmd, "remote-proxy-relay-7897.pid") && strings.Contains(cmd, `bind = "172.17.0.1"`) {
			foundRelayStart = true
			break
		}
	}
	if !foundRelayStart {
		t.Fatalf("expected relay start command, got calls: %v", fake.Calls)
	}
}

func TestEnable_LoopbackOnlySkipsDockerGatewayAndRelay(t *testing.T) {
	fake := &ssh.FakeRunner{OutputData: []byte("172.17.0.1\n")}
	rp := NewRemoteProxy(fake)

	activation, err := rp.Enable(context.Background(), "testhost", Options{
		HTTPAddr:     "127.0.0.1:7897",
		LoopbackOnly: true,
	})
	if err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if activation.BindAddress != "" {
		t.Fatalf("BindAddress = %q, want empty", activation.BindAddress)
	}

	for _, c := range fake.Calls {
		if c.Name != "ssh" || len(c.Args) == 0 {
			continue
		}
		cmd := c.Args[len(c.Args)-1]
		if strings.Contains(cmd, "docker0") || strings.Contains(cmd, "remote-proxy-relay-") {
			t.Fatalf("unexpected docker gateway or relay command: %q", cmd)
		}
	}
}

func TestDisable_CancelCommandArgs(t *testing.T) {
	fake := &ssh.FakeRunner{}
	rp := NewRemoteProxy(fake)

	err := rp.Disable(context.Background(), "testhost", Activation{
		HTTPAddr: "127.0.0.1:7897",
	})
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}

	foundCancel := false
	for _, c := range fake.Calls {
		if c.Name != "ssh" {
			continue
		}
		argsStr := strings.Join(c.Args, " ")
		if strings.Contains(argsStr, "-O cancel") && strings.Contains(argsStr, "-R 7897:") {
			foundCancel = true
		}
	}

	if !foundCancel {
		t.Errorf("expected ssh cancel call for port 7897, got calls: %v", fake.Calls)
	}
}

func TestDisable_StopsRelayWhenBindAddressSet(t *testing.T) {
	fake := &ssh.FakeRunner{}
	rp := NewRemoteProxy(fake)

	err := rp.Disable(context.Background(), "testhost", Activation{
		HTTPAddr:    "127.0.0.1:7897",
		BindAddress: "172.17.0.1",
	})
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}

	foundRelayStop := false
	for _, c := range fake.Calls {
		if c.Name != "ssh" || len(c.Args) == 0 {
			continue
		}
		cmd := c.Args[len(c.Args)-1]
		if strings.Contains(cmd, "remote-proxy-relay-7897.pid") && strings.Contains(cmd, "os.kill(pid, signal.SIGTERM)") {
			foundRelayStop = true
			break
		}
	}
	if !foundRelayStop {
		t.Fatalf("expected relay stop command, got calls: %v", fake.Calls)
	}
}

func TestBuildRemoteEnvContent_HTTPAndSOCKS(t *testing.T) {
	content := BuildRemoteEnvContent("127.0.0.1:7897", "127.0.0.1:1080")

	expected := []string{
		`export http_proxy="http://127.0.0.1:7897"`,
		`export https_proxy="http://127.0.0.1:7897"`,
		`export all_proxy="socks5h://127.0.0.1:1080"`,
		`export no_proxy="localhost,127.0.0.1"`,
	}

	for _, line := range expected {
		if !strings.Contains(content, line) {
			t.Errorf("content missing line: %s\ncontent: %s", line, content)
		}
	}
}

func TestBuildRemoteEnvContent_SocksOnlyUsesHTTPFallback(t *testing.T) {
	content := BuildRemoteEnvContent("", "127.0.0.1:1080")

	if !strings.Contains(content, `export http_proxy="http://127.0.0.1:1080"`) {
		t.Fatalf("expected HTTP fallback to SOCKS port, got: %s", content)
	}
	if !strings.Contains(content, `export all_proxy="socks5h://127.0.0.1:1080"`) {
		t.Fatalf("expected socks5h all_proxy, got: %s", content)
	}
}

func TestUniqueForwards_Deduplication(t *testing.T) {
	tests := []struct {
		name      string
		httpAddr  string
		socksAddr string
		wantCount int
	}{
		{"both empty", "", "", 0},
		{"http only", "127.0.0.1:7897", "", 1},
		{"socks only", "", "127.0.0.1:1080", 1},
		{"same address", "127.0.0.1:7897", "127.0.0.1:7897", 1},
		{"same port different host", "127.0.0.1:7897", "localhost:7897", 1},
		{"different ports", "127.0.0.1:7897", "127.0.0.1:1080", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forwards := uniqueForwards(tt.httpAddr, tt.socksAddr)
			if len(forwards) != tt.wantCount {
				t.Errorf("got %d forwards, want %d", len(forwards), tt.wantCount)
			}
		})
	}
}

func TestExtractPort(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"127.0.0.1:7897", "7897"},
		{"127.0.0.1:1080", "1080"},
		{"0.0.0.0:8080", "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := extractPort(tt.addr)
			if got != tt.want {
				t.Errorf("extractPort(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestEnable_PatchesShellProfilesEvenWhenMissing(t *testing.T) {
	fake := &ssh.FakeRunner{}
	rp := NewRemoteProxy(fake)

	_, err := rp.Enable(context.Background(), "testhost", Options{
		HTTPAddr: "127.0.0.1:7897",
	})
	if err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}

	foundPatch := false
	for _, c := range fake.Calls {
		if c.Name != "ssh" || len(c.Args) < 3 {
			continue
		}
		cmd := c.Args[len(c.Args)-1]
		if strings.Contains(cmd, "open(p, 'a').close()") {
			foundPatch = true
			break
		}
	}

	if !foundPatch {
		t.Fatalf("expected remote patch command to create missing rc files, got calls: %v", fake.Calls)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
