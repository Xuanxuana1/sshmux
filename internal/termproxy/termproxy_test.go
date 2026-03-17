package termproxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEnvContent_AllAddrs(t *testing.T) {
	content := BuildEnvContent("127.0.0.1:7897", "127.0.0.1:7897")

	expected := []string{
		`export http_proxy="http://127.0.0.1:7897"`,
		`export https_proxy="http://127.0.0.1:7897"`,
		`export HTTP_PROXY="http://127.0.0.1:7897"`,
		`export HTTPS_PROXY="http://127.0.0.1:7897"`,
		`export all_proxy="socks5://127.0.0.1:7897"`,
		`export ALL_PROXY="socks5://127.0.0.1:7897"`,
		`export no_proxy="localhost,127.0.0.1"`,
		`export NO_PROXY="localhost,127.0.0.1"`,
	}

	for _, line := range expected {
		if !strings.Contains(content, line) {
			t.Errorf("content missing line: %s", line)
		}
	}
}

func TestBuildEnvContent_HTTPOnly(t *testing.T) {
	content := BuildEnvContent("127.0.0.1:8080", "")

	if !strings.Contains(content, `export http_proxy="http://127.0.0.1:8080"`) {
		t.Error("missing http_proxy line")
	}
	if strings.Contains(content, "all_proxy") {
		t.Error("should not contain all_proxy when socks addr is empty")
	}
	if !strings.Contains(content, "no_proxy") {
		t.Error("missing no_proxy line")
	}
}

func TestBuildEnvContent_SOCKSOnly(t *testing.T) {
	content := BuildEnvContent("", "127.0.0.1:1080")

	if strings.Contains(content, "http_proxy") {
		t.Error("should not contain http_proxy when http addr is empty")
	}
	if !strings.Contains(content, `export all_proxy="socks5://127.0.0.1:1080"`) {
		t.Error("missing all_proxy line")
	}
	if !strings.Contains(content, "no_proxy") {
		t.Error("missing no_proxy line")
	}
}

func TestBuildEnvContent_DifferentPorts(t *testing.T) {
	content := BuildEnvContent("127.0.0.1:8080", "127.0.0.1:1080")

	if !strings.Contains(content, `export http_proxy="http://127.0.0.1:8080"`) {
		t.Error("missing http_proxy with port 8080")
	}
	if !strings.Contains(content, `export all_proxy="socks5://127.0.0.1:1080"`) {
		t.Error("missing all_proxy with port 1080")
	}
}

func TestPatchProfile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".zshrc")

	// Create an empty profile
	if err := os.WriteFile(profile, []byte("# existing content\n"), 0644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	// Patch once
	if err := PatchProfileAt(profile); err != nil {
		t.Fatalf("first PatchProfileAt: %v", err)
	}

	// Read and count marker lines
	data1, _ := os.ReadFile(profile)
	count1 := strings.Count(string(data1), markerLine)

	// Patch again
	if err := PatchProfileAt(profile); err != nil {
		t.Fatalf("second PatchProfileAt: %v", err)
	}

	data2, _ := os.ReadFile(profile)
	count2 := strings.Count(string(data2), markerLine)

	if count1 != 1 {
		t.Errorf("after first patch: marker count = %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("after second patch: marker count = %d, want 1 (idempotent)", count2)
	}

	// Verify the source line is present
	if !strings.Contains(string(data2), sourceLine) {
		t.Error("source line not found in profile")
	}
}

func TestPatchProfile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".zshrc")

	// File does not exist
	if err := PatchProfileAt(profile); err != nil {
		t.Fatalf("PatchProfileAt: %v", err)
	}

	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}

	if !strings.Contains(string(data), markerLine) {
		t.Error("marker line not found")
	}
	if !strings.Contains(string(data), sourceLine) {
		t.Error("source line not found")
	}
}

func TestPatchProfile_UpgradesOldFormat(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".zshrc")

	// Write profile with old single-line format
	oldContent := "# existing\n" + markerLine + "\n" + oldSourceLine + "\n"
	if err := os.WriteFile(profile, []byte(oldContent), 0644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	if err := PatchProfileAt(profile); err != nil {
		t.Fatalf("PatchProfileAt: %v", err)
	}

	data, _ := os.ReadFile(profile)
	content := string(data)

	if strings.Contains(content, oldSourceLine) {
		t.Error("old single-line format should have been replaced")
	}
	if !strings.Contains(content, "unset http_proxy") {
		t.Error("new if/else block with unset not found")
	}
	if strings.Count(content, markerLine) != 1 {
		t.Errorf("marker count = %d, want 1", strings.Count(content, markerLine))
	}
	if !strings.HasPrefix(content, "# existing\n") {
		t.Error("content before marker was modified")
	}
}

func TestPatchProfile_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, ".zshrc")

	existing := "export PATH=\"/usr/local/bin:$PATH\"\nalias ll='ls -la'\n"
	if err := os.WriteFile(profile, []byte(existing), 0644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	if err := PatchProfileAt(profile); err != nil {
		t.Fatalf("PatchProfileAt: %v", err)
	}

	data, _ := os.ReadFile(profile)
	content := string(data)

	if !strings.HasPrefix(content, existing) {
		t.Error("existing content was modified")
	}
	if !strings.Contains(content, markerLine) {
		t.Error("marker line not appended")
	}
}
