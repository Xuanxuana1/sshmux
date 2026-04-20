package ssh

import (
	"context"
	"fmt"
	"os"
	"testing"
)

type sequenceRunner struct {
	runErrs  []error
	runCalls int
}

func (r *sequenceRunner) Run(_ context.Context, _ string, _ ...string) error {
	r.runCalls++
	if len(r.runErrs) == 0 {
		return nil
	}
	err := r.runErrs[0]
	r.runErrs = r.runErrs[1:]
	return err
}

func (r *sequenceRunner) Output(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}

func (r *sequenceRunner) RunWithInput(_ context.Context, _ string, _ string, _ ...string) error {
	return nil
}

func TestMasterIsConnectedRemovesStaleControlSocket(t *testing.T) {
	stalePath := createTempSocketPlaceholder(t)
	runner := &sequenceRunner{
		runErrs: []error{
			fmt.Errorf("run ssh: Control socket connect(%s): Connection refused: exit status 255", stalePath),
		},
	}

	master := NewMaster(runner)
	connected, err := master.IsConnected(context.Background(), "stale-host")
	if err != nil {
		t.Fatalf("IsConnected returned error: %v", err)
	}
	if connected {
		t.Fatal("IsConnected returned true for a stale control socket")
	}
	if _, statErr := os.Stat(stalePath); !os.IsNotExist(statErr) {
		t.Fatalf("stale control socket was not removed, stat err=%v", statErr)
	}
}

func TestMasterConnectRetriesAfterRemovingStaleControlSocket(t *testing.T) {
	stalePath := createTempSocketPlaceholder(t)
	runner := &sequenceRunner{
		runErrs: []error{
			fmt.Errorf("run ssh: Control socket connect(%s): Connection refused: exit status 255", stalePath),
			nil,
		},
	}

	master := NewMaster(runner)
	if err := master.Connect(context.Background(), "stale-host"); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if runner.runCalls != 2 {
		t.Fatalf("Connect called runner %d times, want 2", runner.runCalls)
	}
	if _, statErr := os.Stat(stalePath); !os.IsNotExist(statErr) {
		t.Fatalf("stale control socket was not removed, stat err=%v", statErr)
	}
}

func createTempSocketPlaceholder(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/cm-stale"
	if err := os.WriteFile(path, []byte("stale"), 0600); err != nil {
		t.Fatalf("write stale control socket placeholder: %v", err)
	}
	return path
}
