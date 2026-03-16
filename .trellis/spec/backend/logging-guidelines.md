# Logging

> Structured logging conventions for sshmux.

---

## Library

Use Go standard **`log/slog`** (Go 1.21+). No external logging dependency.

```go
import "log/slog"
```

---

## Initialization in `main.go`

```go
func initLogger(debug bool) {
    var handler slog.Handler
    opts := &slog.HandlerOptions{AddSource: debug}
    if debug {
        opts.Level = slog.LevelDebug
        handler = slog.NewTextHandler(os.Stderr, opts)
    } else {
        opts.Level = slog.LevelWarn
        handler = slog.NewTextHandler(os.Stderr, opts)
    }
    slog.SetDefault(slog.New(handler))
}
```

Normal mode: only `Warn` and above are shown (clean CLI output).
`--debug` flag: all levels, including source location.

---

## Log Levels

| Level | When to use | Example |
|-------|-------------|---------|
| `Debug` | Internal steps, command args, file paths | `"running ssh command" args=["-MNf", "myserver"]` |
| `Info` | State transitions visible to an operator | `"socks enabled" host=myserver port=1087` |
| `Warn` | Recoverable failure; user action may help | `"macOS proxy sync failed, local proxy still active"` |
| `Error` | Operation failed, user must take action | `"privoxy failed to start" err="port 8118 in use"` |

---

## Structured Field Conventions

Always include `"host"` when the log event relates to a specific SSH host:

```go
// ✅ Good
slog.Info("socks enabled",   "host", host, "port", port)
slog.Warn("macos sync fail", "host", host, "err", err)
slog.Error("connect failed", "host", host, "err", err)

// ❌ Bad — no context
slog.Info("socks enabled")
```

Standard field names:

| Field | Type | Usage |
|-------|------|-------|
| `host` | string | SSH host alias |
| `port` | int | proxy port number |
| `err` | error | the Go error value |
| `path` | string | file system path |
| `pid` | int | child process PID (Privoxy) |

---

## CLI Output vs Logs

- `fmt.Fprintf(os.Stdout, ...)` — user-facing command output (success results, table listings)
- `slog.*` — internal diagnostics (only visible with `--debug` or on `Warn`/`Error`)

Never mix: do not `slog.Info` things that are part of the normal command output.

```go
// ✅ Good — success output goes to stdout
fmt.Printf("SOCKS proxy enabled on 127.0.0.1:%d\n", port)

// ❌ Bad — normal output buried in logs
slog.Info("SOCKS proxy enabled", "port", port)
```

---

## What to Log

| Event | Level |
|-------|-------|
| SSH master connect / disconnect | `Info` |
| SOCKS on / off / port change | `Info` |
| HTTP proxy on / off / port change | `Info` |
| macOS system proxy updated | `Info` |
| macOS system proxy update failed (proxy still alive) | `Warn` |
| External command being run (with args) | `Debug` |
| State file read / written | `Debug` |
| Port verification check | `Debug` |
| Any `error` return from `internal/` packages | `Error` (in cmd layer) |

---

## What NOT to Log

- **Identity file contents** — path is fine, never the key content
- **SSH usernames/passwords** — log the host alias only
- **Full proxy.env contents** — log "proxy.env updated" not the values
- **Stack traces in user-facing output** — use `--debug` only
