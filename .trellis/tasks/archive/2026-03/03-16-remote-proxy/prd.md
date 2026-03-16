# Remote Server Proxy Forward

## Goal

远端服务器无法直接访问外网，但 Mac 本地开着代理（如 Clash/Surge，端口 7897）。
通过 SSH 反向端口转发，把 Mac 的代理端口映射到远端服务器的本地端口，让远端服务器
的程序能通过 Mac 的代理访问外网。支持热切换（不断开 SSH 即可开关、换端口）。

---

## Requirements

### 命令

```bash
sshmux remote-proxy on  <host> [--http 127.0.0.1:7897] [--socks 127.0.0.1:7897]
sshmux remote-proxy off <host>
sshmux remote-proxy status <host>
```

- `--http` / `--socks` 默认沿用 `terminal-proxy` 的配置（若存在），也可单独指定
- 配置持久化到远端 host 的 `HostState`（`remote_proxy_enabled`, `remote_proxy_http_addr`, `remote_proxy_socks_addr`）

### on 的行为（分两步）

**Step 1 — SSH 反向端口转发**

```bash
# HTTP 代理（若配置了 --http）
ssh -O forward -R 7897:127.0.0.1:7897 <host>

# SOCKS 代理（若配置了 --socks，且端口不同则另起一条）
ssh -O forward -R 7897:127.0.0.1:7897 <host>
```

若 HTTP 和 SOCKS 是同一个地址（如都是 127.0.0.1:7897），只建一条转发即可。

**Step 2 — 写入远端 proxy.env**

通过 SSH exec 在远端写入 `~/.sshmux/proxy.env`，并在远端 `~/.zshrc` / `~/.bash_profile` 追加 source 行（幂等）：

```bash
ssh <host> "mkdir -p ~/.sshmux && cat > ~/.sshmux/proxy.env" <<'EOF'
export http_proxy="http://127.0.0.1:7897"
export https_proxy="http://127.0.0.1:7897"
export HTTP_PROXY="http://127.0.0.1:7897"
export HTTPS_PROXY="http://127.0.0.1:7897"
export all_proxy="socks5://127.0.0.1:7897"
export ALL_PROXY="socks5://127.0.0.1:7897"
export no_proxy="localhost,127.0.0.1"
export NO_PROXY="localhost,127.0.0.1"
EOF

ssh <host> "grep -q 'sshmux: terminal proxy' ~/.zshrc 2>/dev/null || \
  echo '[ -f ~/.sshmux/proxy.env ] && source ~/.sshmux/proxy.env' >> ~/.zshrc"
```

### off 的行为

```bash
# 撤销端口转发（不断 SSH）
ssh -O cancel -R 7897:127.0.0.1:7897 <host>

# 删除远端 proxy.env
ssh <host> "rm -f ~/.sshmux/proxy.env"
```

### 代理端口变化（热切换）

port 变更时，遵循 prp.md 的安全切换原则：start new → verify → stop old

```bash
ssh -O forward -R <newPort>:127.0.0.1:<newLocalPort> <host>
# verify: ssh <host> "nc -z 127.0.0.1 <newPort>"
# update remote proxy.env（覆盖写）
ssh -O cancel  -R <oldPort>:127.0.0.1:<oldLocalPort> <host>
```

---

## HostState 新增字段

```go
RemoteProxyEnabled   bool   `json:"remote_proxy_enabled"`
RemoteProxyHTTPAddr  string `json:"remote_proxy_http_addr,omitempty"`  // "127.0.0.1:7897"
RemoteProxySOCKSAddr string `json:"remote_proxy_socks_addr,omitempty"` // "127.0.0.1:7897"
```

---

## Acceptance Criteria

- [ ] `sshmux remote-proxy on myserver --http 127.0.0.1:7897` → 远端 `curl -x http://127.0.0.1:7897 https://google.com` 成功
- [ ] `sshmux remote-proxy off myserver` → 远端端口转发消失，`curl` 超时
- [ ] SSH 连接保持，不重连，ControlMaster socket 不变
- [ ] `sshmux remote-proxy on myserver --http 127.0.0.1:7898`（端口变更）→ 新端口生效，旧端口转发撤销
- [ ] 远端新开 shell 自动有 `http_proxy` 环境变量（via ~/.sshmux/proxy.env source）
- [ ] `sshmux remote-proxy status myserver` → 打印当前状态和转发地址
- [ ] ControlMaster 不存在时，返回错误提示"请先运行 sshmux connect myserver"

---

## Definition of Done

- 单元测试：端口转发命令生成、proxy.env 内容生成、热切换的 cancel→forward 顺序
- `go fmt / vet / test / golangci-lint` 通过
- `sshmux --help` 中 remote-proxy 子命令有描述

---

## Limitations（明确告知用户）

- **已开启的远端 shell 不会自动热更新 env var**（SSH 无法注入运行中的进程）
  - 解决方法：远端 shell 手动运行 `source ~/.sshmux/proxy.env`，或新开终端
- **需要远端 sshd 允许 TCP 转发**（默认开启，`AllowTcpForwarding yes`）

---

## Out of Scope

- 远端已开启 shell 的 env var 热注入（需要 SSH agent forwarding 之外的 IPC 机制）
- 让远端局域网内其他机器也能用 Mac 代理（需 GatewayPorts=yes，安全风险较大）
- Fish shell 的远端 proxy.env 格式

---

## Decision (ADR-lite)

**Context**: 热切换机制选型

**Decision**: 使用 SSH ControlMaster 的 `-O forward` / `-O cancel`，而不是重新建立 SSH 连接

**Consequences**:
- 完全满足"切换不需要重连"需求
- 与 sshmux 现有 ControlMaster 架构（Layer 1）自然融合
- 端口变更时遵循 prp.md 的 start-new → verify → stop-old 安全模式
