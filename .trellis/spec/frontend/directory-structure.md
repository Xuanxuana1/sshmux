# Directory Structure (Frontend)

> **Status: Not applicable for MVP.**
>
> The MVP of sshmux is a pure Go CLI tool. There is no frontend in this phase.
> This file will be filled when the optional SwiftUI/AppKit menu bar wrapper is implemented.

---

## Planned Future Structure (SwiftUI menu bar)

When the macOS menu bar UI phase begins, the expected layout is:

```
sshmux-ui/          (separate target or package)
├── App/
│   └── sshmuxApp.swift       # @main NSApplicationDelegate / MenuBarExtra
├── Views/
│   ├── HostListView.swift    # List of imported SSH hosts
│   ├── HostRowView.swift     # Per-host status + toggle controls
│   └── SettingsView.swift    # App-level settings
├── ViewModels/
│   └── HostViewModel.swift   # ObservableObject wrapping CLI state
├── Services/
│   └── SSHMuxCLI.swift       # Shell out to sshmux CLI binary
└── Resources/
    └── Assets.xcassets
```

The UI communicates with the CLI binary via `Process` (shell-out), not via an embedded framework.

---

## Reference

Update this file with actual structure when SwiftUI development begins.
