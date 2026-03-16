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
		wantParts []string // substrings expected in the ssh forward calls
	}{
		{
			name:      "http only",
			httpAddr:  "127.0.0.1:7897",
			socksAddr: "",
			wantParts: []string{"-R", "7897:127.0.0.1:7897"},
		},
		{
			name:      "socks only",
			httpAddr:  "",
			socksAddr: "127.0.0.1:1080",
			wantParts: []string{"-R", "1080:127.0.0.1:1080"},
		},
		{
			name:      "same address deduplicates",
			httpAddr:  "127.0.0.1:7897",
			socksAddr: "127.0.0.1:7897",
			wantParts: []string{"-R", "7897:127.0.0.1:7897"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &ssh.FakeRunner{}
			rp := NewRemoteProxy(fake)

			_ = rp.Enable(context.Background(), "testhost", tt.httpAddr, tt.socksAddr)

			// Find forward calls
			foundForward := false
			for _, c := range fake.Calls {
				if c.Name == "ssh" {
					argsStr := strings.Join(c.Args, " ")
					allFound := true
					for _, part := range tt.wantParts {
						if !strings.Contains(argsStr, part) {
							allFound = false
						}
					}
					if allFound && strings.Contains(argsStr, "-O forward") {
						foundForward = true
					}
				}
			}

			if !foundForward {
				t.Errorf("expected ssh forward call with %v, got calls: %v", tt.wantParts, fake.Calls)
			}
		})
	}
}

func TestDisable_CancelCommandArgs(t *testing.T) {
	fake := &ssh.FakeRunner{}
	rp := NewRemoteProxy(fake)

	_ = rp.Disable(context.Background(), "testhost", "127.0.0.1:7897", "")

	foundCancel := false
	for _, c := range fake.Calls {
		if c.Name == "ssh" {
			argsStr := strings.Join(c.Args, " ")
			if strings.Contains(argsStr, "-O cancel") && strings.Contains(argsStr, "-R 7897:127.0.0.1:7897") {
				foundCancel = true
			}
		}
	}

	if !foundCancel {
		t.Errorf("expected ssh cancel call, got calls: %v", fake.Calls)
	}
}

func TestBuildRemoteEnvContent_HTTPAndSOCKS(t *testing.T) {
	content := BuildRemoteEnvContent("127.0.0.1:7897", "127.0.0.1:1080")

	expected := []string{
		`export http_proxy="http://127.0.0.1:7897"`,
		`export https_proxy="http://127.0.0.1:7897"`,
		`export all_proxy="socks5://127.0.0.1:1080"`,
		`export no_proxy="localhost,127.0.0.1"`,
	}

	for _, line := range expected {
		if !strings.Contains(content, line) {
			t.Errorf("content missing line: %s\ncontent: %s", line, content)
		}
	}
}

func TestBuildRemoteEnvContent_SameAddr(t *testing.T) {
	content := BuildRemoteEnvContent("127.0.0.1:7897", "127.0.0.1:7897")

	if !strings.Contains(content, `export http_proxy="http://127.0.0.1:7897"`) {
		t.Error("missing http_proxy")
	}
	if !strings.Contains(content, `export all_proxy="socks5://127.0.0.1:7897"`) {
		t.Error("missing all_proxy")
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
		{"different addresses", "127.0.0.1:7897", "127.0.0.1:1080", 2},
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

func TestEnable_SameAddrSingleForward(t *testing.T) {
	fake := &ssh.FakeRunner{}
	rp := NewRemoteProxy(fake)

	_ = rp.Enable(context.Background(), "testhost", "127.0.0.1:7897", "127.0.0.1:7897")

	// Count forward calls (should be exactly 1 since addresses are the same)
	forwardCount := 0
	for _, c := range fake.Calls {
		if c.Name == "ssh" {
			argsStr := strings.Join(c.Args, " ")
			if strings.Contains(argsStr, "-O forward") {
				forwardCount++
			}
		}
	}

	if forwardCount != 1 {
		t.Errorf("expected 1 forward call (dedup), got %d", forwardCount)
	}
}
