# Backend Development Guidelines Index

> **Tech Stack**: Go CLI + JSON file state + macOS system integration (networksetup, ssh, privoxy)

---

## Documentation Files

| File | Description | When to Read |
|------|-------------|--------------|
| [directory-structure.md](./directory-structure.md) | Go project layout, package responsibilities | Starting any new package or feature |
| [database-guidelines.md](./database-guidelines.md) | JSON state file: HostState schema, atomic writes | Any state read/write operation |
| [error-handling.md](./error-handling.md) | Error wrapping, categories, partial failure pattern | Implementing any operation that can fail |
| [logging-guidelines.md](./logging-guidelines.md) | log/slog setup, levels, field conventions | Adding log statements |
| [quality-guidelines.md](./quality-guidelines.md) | Forbidden patterns, testing, naming, dependency policy | Before every commit |

---

## Quick Navigation

### Project Structure

| Task | File |
|------|------|
| Where does new code go? | [directory-structure.md](./directory-structure.md) |
| How are external binaries called? | [directory-structure.md](./directory-structure.md) — Runner interface |
| Where is state stored on disk? | [directory-structure.md](./directory-structure.md) — State Directory |

### State Management

| Task | File |
|------|------|
| HostState struct definition | [database-guidelines.md](./database-guidelines.md) |
| Atomic write pattern | [database-guidelines.md](./database-guidelines.md) |
| Load missing host (nil, not error) | [database-guidelines.md](./database-guidelines.md) |

### Error Handling

| Task | File |
|------|------|
| Wrap errors with context | [error-handling.md](./error-handling.md) |
| Sentinel error types | [error-handling.md](./error-handling.md) |
| Port-switch safety pattern | [error-handling.md](./error-handling.md) |
| Partial failure (proxy up, macOS sync failed) | [error-handling.md](./error-handling.md) |

### Logging

| Task | File |
|------|------|
| Which log level to use? | [logging-guidelines.md](./logging-guidelines.md) |
| CLI output vs internal logs | [logging-guidelines.md](./logging-guidelines.md) |
| What NOT to log (secrets, keys) | [logging-guidelines.md](./logging-guidelines.md) |

### Quality & Testing

| Task | File |
|------|------|
| Pre-commit checklist | [quality-guidelines.md](./quality-guidelines.md) |
| Table-driven test pattern | [quality-guidelines.md](./quality-guidelines.md) |
| Interface injection for testability | [quality-guidelines.md](./quality-guidelines.md) |
| Adding a new dependency | [quality-guidelines.md](./quality-guidelines.md) — Dependency Policy |

---

## Core Rules Summary

| Rule | Reference |
|------|-----------|
| **`cmd/` is thin** — no business logic, only flag parsing + output | [directory-structure.md](./directory-structure.md) |
| **All external binaries through `Runner` interface** | [directory-structure.md](./directory-structure.md) |
| **State writes are atomic** (temp file + rename) | [database-guidelines.md](./database-guidelines.md) |
| **Load missing state = nil, not error** | [database-guidelines.md](./database-guidelines.md) |
| **Always wrap errors** with `fmt.Errorf("ctx: %w", err)` | [error-handling.md](./error-handling.md) |
| **No `os.Exit` / `log.Fatal` outside `cmd/`** | [error-handling.md](./error-handling.md) |
| **Port switch: start new → verify → switch → stop old** | [error-handling.md](./error-handling.md) |
| **`slog.Info/Warn/Error` for diagnostics; `fmt.Print` for command output** | [logging-guidelines.md](./logging-guidelines.md) |
| **Always include `"host"` field in logs** | [logging-guidelines.md](./logging-guidelines.md) |
| **`go fmt / vet / test / golangci-lint` before every commit** | [quality-guidelines.md](./quality-guidelines.md) |
| **Minimize dependencies** — stdlib first | [quality-guidelines.md](./quality-guidelines.md) |

---

## Reference Files

| Item | Location |
|------|----------|
| Entry point | `cmd/sshmux/main.go` |
| HostState type | `internal/state/types.go` |
| Runner interface | `internal/ssh/runner.go` |
| State directory | `~/.sshmux/hosts/` |
| Terminal proxy env file | `~/.sshmux/proxy.env` |

---

**Language**: All documentation must be written in **English**.
