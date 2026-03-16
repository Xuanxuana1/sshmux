# Error Handling

> Go-idiomatic error handling for sshmux.

---

## Core Rule: Always Return Errors

Business logic in `internal/` always returns `error`. Never call `os.Exit` or `log.Fatal` inside `internal/` packages. Only `cmd/` (the CLI layer) decides how to present and exit.

```go
// ✅ Good — internal package returns error
func (m *Master) Connect(host string) error {
    if err := m.runner.Run("ssh", "-MNf", host); err != nil {
        return fmt.Errorf("connect %s: %w", host, err)
    }
    return nil
}

// ❌ Bad — internal package exits the process
func (m *Master) Connect(host string) {
    if err := ...; err != nil {
        log.Fatalf("connect failed: %v", err)
    }
}
```

---

## Error Wrapping

Always add context when propagating an error up the call stack:

```go
// ✅ Good — context chain makes debugging fast
if err := state.Save(s); err != nil {
    return fmt.Errorf("socks on %s: %w", host, err)
}

// ❌ Bad — original error is swallowed
if err := state.Save(s); err != nil {
    return errors.New("failed to save")
}

// ❌ Bad — bare re-return loses context
if err := state.Save(s); err != nil {
    return err
}
```

---

## Error Categories

| Category | Sentinel / Type | Example message |
|----------|----------------|-----------------|
| SSH not connected | `ErrNotConnected` | `"SSH master not connected to myserver"` |
| Port occupied | `ErrPortOccupied` | `"port 1087 is already in use"` |
| Proxy adapter failed | `ErrAdapterFailed` | `"HTTP proxy adapter failed to start on :8118"` |
| SSH config parse error | `ErrConfigParse` | `"parse ~/.ssh/config: unexpected token on line 12"` |
| macOS proxy update failed | `ErrMacOSProxy` | `"networksetup -setSOCKSFirewall failed: exit status 1"` |

---

## Sentinel Errors

```go
// internal/proxy/errors.go
var (
    ErrNotConnected  = errors.New("SSH master not connected")
    ErrPortOccupied  = errors.New("port already in use")
    ErrAdapterFailed = errors.New("HTTP proxy adapter failed")
)
```

Use `errors.Is` in the CLI layer to map to user-friendly messages:

```go
// cmd/sshmux/socks.go
if err := proxy.SocksOn(host, port); err != nil {
    switch {
    case errors.Is(err, proxy.ErrNotConnected):
        fmt.Fprintf(os.Stderr, "Error: not connected to %s. Run: sshmux connect %s\n", host, host)
    case errors.Is(err, proxy.ErrPortOccupied):
        fmt.Fprintf(os.Stderr, "Error: port %d is already in use. Choose a different port.\n", port)
    default:
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    }
    os.Exit(1)
}
```

---

## Partial Failure Pattern

When an operation partially succeeds (e.g., proxy started but macOS sync failed), keep the local proxy alive and report the partial failure clearly:

```go
// ✅ Good — partial failure is explicit
type SocksOnResult struct {
    SocksStarted    bool
    MacSyncUpdated  bool
    MacSyncError    error   // non-nil = partial failure
}

func SocksOn(host string, port int) (*SocksOnResult, error) {
    if err := startSOCKS(host, port); err != nil {
        return nil, fmt.Errorf("start SOCKS: %w", err)  // Hard failure
    }
    res := &SocksOnResult{SocksStarted: true}
    if err := macos.SetSOCKSProxy(port); err != nil {
        res.MacSyncError = fmt.Errorf("macOS proxy sync: %w", err)  // Soft failure
    } else {
        res.MacSyncUpdated = true
    }
    return res, nil
}
```

---

## Port Switch Safety

From `prp.md`: during any port switch, always use "start new → verify → switch → stop old".
If any step fails, abort and keep the existing configuration:

```go
func SwitchSOCKSPort(host string, newPort int) error {
    // 1. Start new listener
    if err := addForward(host, newPort); err != nil {
        return fmt.Errorf("add new SOCKS forward: %w", err)
    }
    // 2. Verify
    if err := verifyListener(newPort); err != nil {
        removeForward(host, newPort) // best-effort cleanup
        return fmt.Errorf("verify new SOCKS port %d: %w", newPort, err)
    }
    // 3. Switch system settings
    // 4. Remove old (best-effort; old port being stuck is non-fatal)
    return nil
}
```

---

## Anti-Patterns

- **❌ `panic()` for expected failures** — port busy, SSH not running, file missing are all expected
- **❌ Swallowing errors** — `if err != nil { _ = err }` is never acceptable
- **❌ Exposing stack traces to the user** — internal errors are logged at DEBUG; user sees a clean message
- **❌ `log.Fatal` in `internal/`** — only `cmd/` may terminate the process
