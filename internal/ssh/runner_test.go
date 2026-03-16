package ssh

import (
	"context"
	"testing"
)

func TestFakeRunner_RecordsCalls(t *testing.T) {
	fake := &FakeRunner{}
	ctx := context.Background()

	_ = fake.Run(ctx, "ssh", "-MNf", "myserver")
	_, _ = fake.Output(ctx, "ssh", "-O", "check", "myserver")

	if len(fake.Calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(fake.Calls))
	}

	if fake.Calls[0].Name != "ssh" {
		t.Errorf("call 0 name = %q, want %q", fake.Calls[0].Name, "ssh")
	}
	if fake.Calls[1].String() != "ssh -O check myserver" {
		t.Errorf("call 1 = %q, want %q", fake.Calls[1].String(), "ssh -O check myserver")
	}
}
