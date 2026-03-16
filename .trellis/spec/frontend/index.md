# Frontend Development Guidelines Index

> **Status: Not applicable for MVP.**
>
> The sshmux MVP is a pure Go CLI tool. There is no frontend in this phase.
> A SwiftUI/AppKit menu bar wrapper is planned as a future optional phase.

---

## Current Phase (MVP: Go CLI)

All development guidelines are in `../backend/`. There is no frontend code to write.

The CLI is the only user interface. Output conventions:
- Command results → `fmt.Fprintf(os.Stdout, ...)`
- Errors → `fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)`
- Diagnostics → `slog.Info/Warn/Error` (shown with `--debug` flag or on Warn+)

---

## Future Phase (SwiftUI menu bar)

When the menu bar UI begins, this index will be updated with:

| File | Description |
|------|-------------|
| `directory-structure.md` | SwiftUI project layout |
| `component-guidelines.md` | View / ViewModel patterns |
| `state-management.md` | CLI shell-out → ViewModel → View data flow |
| `quality-guidelines.md` | SwiftLint, accessibility, testing |

### Planned Architecture

```
sshmux CLI binary (source of truth)
        ↑  shell-out (Process + JSON output)
SwiftUI ViewModel  (ObservableObject, polls CLI)
        ↑  @Published
SwiftUI Views  (menu bar popover, host list, toggles)
```

The CLI binary remains the single source of truth for all proxy state.
The UI never writes state directly — it always calls the CLI.

---

**Language**: All documentation must be written in **English**.
