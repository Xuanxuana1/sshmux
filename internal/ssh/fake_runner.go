package ssh

import (
	"context"
	"fmt"
	"strings"
)

// FakeRunner records calls for testing and returns canned results.
type FakeRunner struct {
	Calls      []FakeCall
	RunErr     error
	OutputData []byte
	OutputErr  error
}

// FakeCall records a single call to Run, Output, or RunWithInput.
type FakeCall struct {
	Name  string
	Args  []string
	Input string
}

func (f *FakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.Calls = append(f.Calls, FakeCall{Name: name, Args: args})
	return f.RunErr
}

func (f *FakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, FakeCall{Name: name, Args: args})
	return f.OutputData, f.OutputErr
}

func (f *FakeRunner) RunWithInput(_ context.Context, input string, name string, args ...string) error {
	f.Calls = append(f.Calls, FakeCall{Name: name, Args: args, Input: input})
	return f.RunErr
}

// String returns a string representation of a call for assertions.
func (c FakeCall) String() string {
	return fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
}
