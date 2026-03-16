# State Management (Frontend)

> **Status: Not applicable for MVP.**
>
> The MVP of sshmux is a pure Go CLI tool. Frontend state management does not apply.

---

## CLI State Model (Current MVP)

In the CLI, state is managed via JSON files on disk. See `../backend/database-guidelines.md` for the full `HostState` schema.

The CLI reads state from disk on every command invocation — there is no in-memory state between invocations. This is intentional: the tool is stateless between runs.

---

## Future (SwiftUI menu bar)

When the menu bar UI is added, the state flow will be:

```
sshmux CLI (source of truth — JSON files on disk)
     ↑ poll / shell-out
SwiftUI ViewModel (ObservableObject)
     ↑ @Published
SwiftUI Views (renders current state)
```

The CLI binary remains the single source of truth. The UI reads state by calling `sshmux status <host>` and parses the output (or a JSON output flag).
