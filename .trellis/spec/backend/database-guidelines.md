# State Management (JSON Files)

> sshmux has no traditional database. Per-host state is stored as individual JSON files.

---

## Storage Location

```
~/.sshmux/hosts/<host-alias>.json
```

One file per SSH host alias. The directory is created on first use.

---

## HostState Schema

```go
// internal/state/types.go

type MacSyncMode string

const (
    MacSyncOff          MacSyncMode = "off"
    MacSyncSOCKSOnly    MacSyncMode = "socks-only"
    MacSyncHTTPOnly     MacSyncMode = "http-https-only"
    MacSyncFull         MacSyncMode = "full"
)

type HostState struct {
    HostAlias           string      `json:"host_alias"`
    SourceConfigPath    string      `json:"source_config_path"`
    Hostname            string      `json:"hostname"`
    User                string      `json:"user"`
    Port                int         `json:"port"`
    IdentityFile        string      `json:"identity_file,omitempty"`
    ProxyJump           string      `json:"proxy_jump,omitempty"`
    ProxyCommand        string      `json:"proxy_command,omitempty"`

    MasterConnected     bool        `json:"master_connected"`

    SocksEnabled        bool        `json:"socks_enabled"`
    SocksPort           int         `json:"socks_port"`

    HTTPEnabled         bool        `json:"http_enabled"`
    HTTPPort            int         `json:"http_port"`

    MacSyncEnabled      bool        `json:"mac_sync_enabled"`
    MacSyncMode         MacSyncMode `json:"mac_sync_mode"`
    MacNetworkService   string      `json:"mac_network_service"`

    TerminalProxyEnabled bool       `json:"terminal_proxy_enabled"`

    UpdatedAt           time.Time   `json:"updated_at"`
    LastError           string      `json:"last_error,omitempty"`
}
```

---

## Read Pattern

```go
// internal/state/store.go

func Load(alias string) (*HostState, error) {
    path := statePath(alias)
    data, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil   // Not found = zero state, not an error
    }
    if err != nil {
        return nil, fmt.Errorf("read state %s: %w", alias, err)
    }
    var s HostState
    if err := json.Unmarshal(data, &s); err != nil {
        return nil, fmt.Errorf("parse state %s: %w", alias, err)
    }
    return &s, nil
}
```

---

## Write Pattern (Atomic)

Always use atomic writes: write to a temp file in the same directory, then `os.Rename`.

```go
func Save(s *HostState) error {
    s.UpdatedAt = time.Now()

    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal state %s: %w", s.HostAlias, err)
    }

    dir := stateDir()
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("create state dir: %w", err)
    }

    tmp, err := os.CreateTemp(dir, ".tmp-")
    if err != nil {
        return fmt.Errorf("create temp file: %w", err)
    }
    tmpName := tmp.Name()

    if _, err := tmp.Write(data); err != nil {
        tmp.Close()
        os.Remove(tmpName)
        return fmt.Errorf("write temp file: %w", err)
    }
    tmp.Close()

    // Atomic rename — safe on the same filesystem
    if err := os.Rename(tmpName, statePath(s.HostAlias)); err != nil {
        os.Remove(tmpName)
        return fmt.Errorf("rename state file: %w", err)
    }
    return nil
}
```

---

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| File name | `<host-alias>.json` | `myserver.json` |
| JSON keys | `snake_case` | `socks_port`, `master_connected` |
| Go struct fields | `PascalCase` with json tag | `SocksPort int \`json:"socks_port"\`` |
| Bool fields | plain bool, no `Is` prefix in JSON | `"socks_enabled": true` |

---

## Anti-Patterns

- **❌ Direct `os.WriteFile` to the state path** — non-atomic, file gets corrupted on crash
- **❌ Partial updates** — always rewrite the full `HostState` struct
- **❌ Storing runtime-derived facts as permanent state** — `master_connected` is refreshed by checking the ControlMaster socket, not persisted blindly
- **❌ Sharing a single state file across all hosts** — one file per alias keeps reads/writes independent
