# PRP: macOS local SSH proxy manager with SSH host import

## Goal
Build a local-only macOS utility that:
1. keeps a persistent SSH master connection to a selected server,
2. exposes a local SOCKS proxy over that SSH connection,
3. exposes local HTTP/HTTPS proxy endpoints by adapting to the SOCKS proxy,
4. can switch proxy on/off and change proxy ports without reconnecting the SSH master,
5. can sync macOS system proxy settings,
6. can auto-import SSH hosts from OpenSSH config files, including the config path used by VS Code Remote-SSH,
7. can set proxy environment variables for new local terminal sessions (terminal-proxy),
8. can forward a local proxy to a remote server via SSH reverse port forwarding, with hot-switch (remote-proxy).

## Core technical decision
Use OpenSSH for:
- SSH connection
- connection multiplexing
- SOCKS proxy via DynamicForward

Use a local HTTP proxy adapter for:
- HTTP proxy
- HTTPS CONNECT proxy
- forwarding to the local SOCKS endpoint

Recommended adapter for MVP:
- Privoxy

## Important boundary
Do not attempt to change the transport path of an already-established SSH master connection.
It is acceptable to hot-switch:
- SOCKS on/off
- SOCKS port
- HTTP/HTTPS adapter port
- macOS system proxy target
- remote-proxy on/off and port (via ControlMaster -O forward/cancel)
It is not acceptable to promise hot-switching between direct-connect SSH and ProxyCommand-based SSH without reconnecting.

## User-visible model
Each host profile has:
- imported SSH alias
- connection status
- SOCKS status and port
- HTTP/HTTPS status and port
- macOS sync status
- remote-proxy status and forwarded address
- source config file path

Global (Mac-level) settings:
- terminal-proxy: on/off, http_addr, socks_addr

## Host import requirements
Support importing from:
- user-selected path
- ~/.ssh/config by default
- VS Code Remote-SSH config path if configured
- files referenced by Include directives

Parser rules:
- parse Host blocks
- import concrete host aliases
- skip or separately categorize wildcard/template Host entries
- resolve effective display fields:
  - Host
  - HostName
  - User
  - Port
  - IdentityFile
  - ProxyJump
  - ProxyCommand

## Runtime architecture
### Layer 1: SSH master
Start a persistent master with:
- ControlMaster auto
- ControlPath ~/.ssh/cm-%C
- ControlPersist yes

Command:
- connect host => ssh -MNf <host>

### Layer 2: SOCKS proxy
Enable SOCKS by adding:
- DynamicForward 127.0.0.1:<socksPort>

Disable SOCKS by canceling that forward.

Port change flow:
1. add new SOCKS forward
2. verify new listener
3. update dependent HTTP adapter if needed
4. update macOS SOCKS proxy if sync enabled
5. remove old SOCKS forward

### Layer 3: HTTP/HTTPS proxy adapter
Run Privoxy locally on:
- 127.0.0.1:<httpPort>

Configure Privoxy to forward all traffic to:
- 127.0.0.1:<socksPort>

HTTP and HTTPS system proxies should both point to the same local adapter port.

Port change flow:
1. generate updated adapter config
2. start/reload adapter on new port and/or upstream socks port
3. health-check adapter
4. update macOS WebProxy and SecureWebProxy
5. retire old adapter instance

### Layer 4: macOS system proxy sync
Support:
- SOCKS Firewall Proxy
- Web Proxy
- Secure Web Proxy

Modes:
- off
- socks-only
- http-https-only
- full-sync

### Layer 5: terminal-proxy (Mac-local terminal env)
Global toggle. When enabled, new terminal sessions on the Mac (Terminal.app, VS Code
integrated terminal) automatically inherit proxy environment variables.

Mechanism:
- User configures local proxy address(es): --http addr and/or --socks addr
  (e.g. 127.0.0.1:7897 from Clash/Surge, or sshmux's own ports — user's choice)
- HTTP and SOCKS addresses can differ or be the same
- sshmux writes ~/.sshmux/proxy.env with the configured values
- sshmux appends a source line to ~/.zshrc and ~/.bash_profile (idempotent)
- off: deletes proxy.env; new terminals get no proxy env vars
- Completely decoupled from SSH host connections

proxy.env content (only sections with configured addresses are written):
```
export http_proxy="http://<http_addr>"
export https_proxy="http://<http_addr>"
export HTTP_PROXY="http://<http_addr>"
export HTTPS_PROXY="http://<http_addr>"
export all_proxy="socks5://<socks_addr>"
export ALL_PROXY="socks5://<socks_addr>"
export no_proxy="localhost,127.0.0.1"
export NO_PROXY="localhost,127.0.0.1"
```

Limitation: already-open terminal sessions are not updated. User must open a new
terminal or manually run `source ~/.sshmux/proxy.env`.

### Layer 6: remote-proxy (forward Mac proxy to remote server)
Per-host toggle. The remote server has no direct internet access; Mac has a local proxy
(e.g. Clash on 127.0.0.1:7897). This layer forwards Mac's proxy to the remote server
via SSH reverse port forwarding so remote programs can access the internet.

Mechanism:
- Uses SSH ControlMaster -O forward / -O cancel — hot-switch without SSH reconnect
- on: ssh -O forward -R <port>:127.0.0.1:<localPort> <host>
       + write proxy.env on remote server (via ssh exec)
       + append source line to remote ~/.zshrc (idempotent, via ssh exec)
- off: ssh -O cancel -R <port>:127.0.0.1:<localPort> <host>
        + delete remote proxy.env (via ssh exec)
- port change: start new forward → verify remote listener → update remote proxy.env → cancel old forward

Default proxy address: inherits from terminal-proxy config if set; can be overridden per host.

Limitation: already-open remote shell sessions are not updated automatically.
User must run `source ~/.sshmux/proxy.env` on the remote or open a new remote terminal.
Requires AllowTcpForwarding=yes on remote sshd (default on most servers).

## Commands
- sshmux import-hosts [--config <path>]
- sshmux hosts list
- sshmux connect <host>
- sshmux disconnect <host>
- sshmux status <host>

- sshmux socks on <host> --port 1087
- sshmux socks off <host>
- sshmux socks set-port <host> --port 1088

- sshmux http on <host> --port 8118
- sshmux http off <host>
- sshmux http set-port <host> --port 8119

- sshmux sync on <host> --service "Wi-Fi" --mode full
- sshmux sync off <host> --service "Wi-Fi"

- sshmux terminal-proxy on [--http 127.0.0.1:7897] [--socks 127.0.0.1:7897]
- sshmux terminal-proxy off
- sshmux terminal-proxy status

- sshmux remote-proxy on <host> [--http 127.0.0.1:7897] [--socks 127.0.0.1:7897]
- sshmux remote-proxy off <host>
- sshmux remote-proxy status <host>

## State model

### Per-host JSON (~/.sshmux/hosts/<alias>.json)
- host_alias
- source_config_path
- hostname
- user
- port
- identity_file
- proxy_jump
- proxy_command
- master_connected
- socks_enabled
- socks_port
- http_enabled
- http_port
- mac_sync_enabled
- mac_sync_mode
- mac_network_service
- remote_proxy_enabled
- remote_proxy_http_addr    (e.g. "127.0.0.1:7897", empty = not configured)
- remote_proxy_socks_addr   (e.g. "127.0.0.1:7897", empty = not configured)
- updated_at
- last_error

### Global JSON (~/.sshmux/terminal-proxy.json)
- enabled
- http_addr     (e.g. "127.0.0.1:7897", empty = not configured)
- socks_addr    (e.g. "127.0.0.1:7897", empty = not configured)

## Acceptance criteria
1. Existing SSH shell remains alive while SOCKS is enabled or disabled.
2. Existing SSH shell remains alive while SOCKS port changes.
3. Existing SSH shell remains alive while HTTP/HTTPS adapter port changes.
4. macOS system SOCKS/Web/Secure Web proxy can be switched on/off from the tool.
5. Imported hosts from ~/.ssh/config are shown correctly.
6. Include-based SSH config trees are imported correctly.
7. Wildcard/template Host entries do not pollute the normal host list.
8. If VS Code uses a custom SSH config path, the tool can import from it.
9. If the remote server disallows forwarding, the user gets a clear error.
10. After `terminal-proxy on`, new local terminal inherits http_proxy / ALL_PROXY.
11. After `terminal-proxy off`, new local terminal has no proxy env vars.
12. After `remote-proxy on <host>`, remote server can reach internet via Mac proxy.
13. After `remote-proxy off <host>`, remote port forward is removed without SSH reconnect.
14. Changing proxy address via `remote-proxy on <host> --http newAddr` hot-switches the
    remote forward without disconnecting SSH.

## Error handling
- If SSH master is absent, proxy-on auto-connects.
- If SOCKS port is occupied, fail without disturbing current active proxy.
- If HTTP adapter start fails, keep current working configuration.
- During any port switch, always use "start new -> verify -> switch system settings -> stop old".
- If macOS proxy update fails, keep the local proxy listeners alive and report partial failure.
- If remote-proxy on fails (AllowTcpForwarding disabled, port occupied on remote), report
  error and leave existing configuration unchanged.
- terminal-proxy: if ~/.zshrc is not writable, report error and abort (do not write proxy.env).

## Suggested implementation
MVP:
- Go CLI
- shell out to system ssh
- shell out to networksetup
- manage Privoxy child process and config file
- terminal-proxy: write ~/.sshmux/proxy.env, patch ~/.zshrc / ~/.bash_profile
- remote-proxy: ssh -O forward/cancel via existing ControlMaster
- optional menu bar wrapper later with SwiftUI/AppKit
