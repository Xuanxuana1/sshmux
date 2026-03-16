[中文](./README.md) | **English**

# sshmux

**A macOS SSH proxy manager** with an interactive TUI — manage all your SSH connections and proxy settings without typing a single command.

---

## Installation

### Option A — Pre-built binary (no Go required)

```bash
curl -fsSL https://raw.githubusercontent.com/liuxuan/sshmux/main/install.sh | bash
```

Detects your architecture (Intel / Apple Silicon) automatically, downloads the binary from the latest GitHub Release, installs it to `~/bin/sshmux`, and imports your SSH hosts.

Make sure `~/bin` is in your `PATH` (add to `~/.zshrc` if needed):

```bash
export PATH="$HOME/bin:$PATH"
```

### Option B — Build from source (requires Go 1.21+)

```bash
git clone https://github.com/liuxuan/sshmux.git
cd sshmux
make install
```

**Uninstall**:

```bash
make uninstall
```

---

## Quick Start

```bash
sshmux
```

That's it — the interactive UI opens immediately.

---

## Interface

```
  sshmux -- SSH Proxy Manager

  +-------------------+-------------+---------+--------+
  | Host              | SSH         | Sync    | RPx    |
  +-------------------+-------------+---------+--------+
> | my-server         | * online    | *       | x      |
  | dev-box           | o offline   | x       | x      |
  +-------------------+-------------+---------+--------+

  [c] SSH  [m] macOS sync  [r] remote-proxy  [p] ports  [i] import
  ------------------------------------------------------------------------
  Proxy Ports  SOCKS :7897  HTTP :7897   [p] edit ports
  Terminal Proxy  ON  http=127.0.0.1:7897  socks=127.0.0.1:7897   [t] toggle
  [↑/↓] or [j/k] to navigate   [q] quit
```

Once you establish an SSH connection, **SOCKS5 and HTTP proxies start automatically** on port 7897 — no manual toggle needed.

### Column Reference

| Column | Meaning |
|--------|---------|
| **Host** | Alias from `~/.ssh/config` |
| **SSH** | Master connection status. `* online` = connected, `o offline` = disconnected |
| **Sync** | macOS system proxy sync. `*` = SOCKS proxy synced to System Settings |
| **RPx** | Remote Proxy. Forwards your local proxy into the remote server's environment |

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Navigate the host list |
| `c` | Toggle SSH connection. Proxy starts automatically on connect |
| `m` | Toggle macOS system proxy sync (requires SSH connection) |
| `r` | Toggle Remote Proxy (requires SSH connection + Terminal Proxy on) |
| `t` | Toggle Terminal Proxy (injects `http_proxy` / `https_proxy` into shell) |
| `p` | Edit proxy ports (Tab to switch field, Enter to confirm) |
| `i` | Import hosts from `~/.ssh/config` |
| `q` or `Ctrl+C` | Quit |

---

## Common Workflows

### Route traffic through a remote server

1. Run `sshmux`, select a host with `↑↓`
2. Press `c` to connect — SOCKS/HTTP proxies start automatically
3. Press `m` to sync to macOS System Settings
4. Press `q` to quit — proxies keep running in the background

### Set proxy for the current terminal session

1. Configure the Terminal Proxy address once:
   ```bash
   sshmux terminal-proxy on --http 127.0.0.1:7897 --socks 127.0.0.1:7897
   ```
2. Run `sshmux` and press `t` to enable Terminal Proxy
3. All `curl`, `git`, `npm`, etc. in that terminal now use the proxy

### Forward your local proxy to a remote server

1. Connect via SSH (press `c`)
2. Enable Terminal Proxy (press `t`)
3. Press `r` to enable Remote Proxy — `http_proxy` / `https_proxy` are set automatically on login

### Team collaboration — different proxy ports across machines

In shared server environments, teammates often run local proxies on different ports (7897, 1080, etc.). sshmux keeps proxy ports as a **single global setting**, so you only configure it once for all hosts:

1. Press `p` to edit ports, enter your local port, press Enter — all hosts update instantly
2. Need internet access temporarily? Press `m` to enable macOS system proxy, do your work, then press `m` again to switch back to the internal network
3. Need proxy in just one terminal without affecting other apps? Use `t` to toggle Terminal Proxy — it only injects env vars into the current shell session

---

## First Run

The host list is empty on first launch. Press `i` to import all hosts from `~/.ssh/config`.

---

## State Persistence

All state is saved under `~/.sshmux/`. The UI restores your settings on every launch.

---

## CLI Subcommands

For scripting or automation:

```bash
sshmux hosts                                           # List all hosts
sshmux connect <alias>                                 # Connect (proxy starts automatically)
sshmux disconnect <alias>                              # Disconnect
sshmux sync enable <alias>                             # Sync to macOS system proxy
sshmux sync disable <alias>                            # Disable system proxy sync
sshmux terminal-proxy on --http <addr> --socks <addr>  # Enable Terminal Proxy
sshmux terminal-proxy off                              # Disable Terminal Proxy
sshmux remote-proxy enable <alias>                     # Enable Remote Proxy
sshmux remote-proxy disable <alias>                    # Disable Remote Proxy
sshmux import-hosts                                    # Import from ~/.ssh/config
```

---

## Requirements

- macOS (uses `networksetup` for system proxy management)
- OpenSSH (pre-installed on macOS)
