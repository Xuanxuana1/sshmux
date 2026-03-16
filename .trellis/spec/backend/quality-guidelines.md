# Quality Guidelines

> Code quality standards for sshmux.

---

## Before Every Commit

```bash
go fmt ./...          # format all files
go vet ./...          # catch common mistakes
go test ./...         # run all tests
golangci-lint run     # lint (install: brew install golangci-lint)
```

All four must pass with zero errors before committing.

---

## Forbidden Patterns

| Pattern | Why forbidden | Alternative |
|---------|--------------|-------------|
| `interface{}` / `any` without immediate type assert | Loses type safety | Define a concrete struct or interface |
| Ignoring errors: `_, _ = fn()` | Silent failures | Handle or explicitly document why ignored |
| `os.Exit` outside `cmd/` | Bypasses deferred cleanup | Return `error` up the call chain |
| `log.Fatal` outside `main()` | Same as above | Return `error` |
| `panic()` for expected conditions | Port busy, file missing are expected | Return typed error |
| Hardcoded paths like `/Users/foo/.ssh/config` | Not portable | Use `os.UserHomeDir()` |
| Global mutable state | Causes test flakiness | Pass dependencies explicitly |

---

## Required Patterns

### Interface Injection for External Binaries

All shell-outs (`ssh`, `networksetup`, `privoxy`) go through an injectable interface:

```go
// internal/ssh/runner.go
type Runner interface {
    Run(name string, args ...string) error
    Output(name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) error {
    return exec.Command(name, args...).Run()
}
```

Tests inject a `FakeRunner` that records calls and returns canned output.

### Explicit `context.Context` Propagation

Any function that runs an external process accepts a `ctx context.Context` as its first parameter:

```go
func (m *Master) Connect(ctx context.Context, host string) error
```

This allows callers (and tests) to cancel long-running operations.

### `os.UserHomeDir()` for Home Directory

```go
// ✅
home, err := os.UserHomeDir()

// ❌
home := "/Users/liuxuan"
```

---

## Testing Conventions

### Table-Driven Tests

Use table-driven tests for parser logic, state serialization, and command dispatch:

```go
func TestParseSSHConfig(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        wantHost string
        wantErr  bool
    }{
        {"simple host", "Host myserver\n  HostName 10.0.0.1\n", "myserver", false},
        {"wildcard skipped", "Host *\n  ServerAliveInterval 60\n", "", false},
        {"malformed", "Host\n", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### Test File Location

Test files live next to the source they test:

```
internal/config/parser.go
internal/config/parser_test.go   ← same package
```

### Minimum Coverage Targets

| Package | What to test |
|---------|-------------|
| `internal/state` | Atomic write, load-missing-returns-nil, round-trip JSON |
| `internal/config` | Host parsing, Include resolution, wildcard exclusion |
| `internal/proxy` | port-switch safety (start new → verify → stop old), env file generation |
| `cmd/sshmux` | Flag parsing, error-to-exit-code mapping |

---

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| Exported types | `PascalCase` | `HostState`, `Master` |
| Unexported helpers | `camelCase` | `statePath`, `verifyListener` |
| Interfaces | noun describing capability | `Runner`, `ProxyManager` |
| Test helpers | `test` prefix (unexported) | `testStateDir()` |
| Constants | `PascalCase` for exported, `camelCase` for unexported | `DefaultSOCKSPort` |
| Error variables | `ErrXxx` | `ErrNotConnected` |

---

## Dependency Policy

Minimize external dependencies. For MVP:

| Need | Allowed dependency |
|------|--------------------|
| CLI framework | `github.com/spf13/cobra` |
| JSON | stdlib `encoding/json` |
| Logging | stdlib `log/slog` |
| SSH config parse | stdlib only (`bufio`, `strings`) |
| Testing | stdlib `testing` |

Before adding any new dependency, ask: can this be done with stdlib in < 50 lines? If yes, do it without the dependency.

---

## Code Review Checklist

- [ ] No `os.Exit` / `log.Fatal` outside `cmd/`
- [ ] All errors wrapped with `fmt.Errorf("context: %w", err)`
- [ ] External binaries called through `Runner` interface
- [ ] State writes are atomic (temp file + rename)
- [ ] New commands have a table-driven test for the happy path
- [ ] `go fmt`, `go vet`, `go test`, `golangci-lint` all pass
