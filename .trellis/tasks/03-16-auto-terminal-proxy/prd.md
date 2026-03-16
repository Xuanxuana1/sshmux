# Auto Terminal Proxy

## Goal

为 sshmux 增加一个全局"终端代理"开关。用户配置好本地代理地址（HTTP 和 SOCKS 可以分别指定），开启后所有新终端（Terminal.app、VS Code 集成终端）自动继承这些代理环境变量；关闭后删除 proxy.env，新终端不再有代理变量。与 SSH host 连接状态完全解耦。

---

## Requirements

- `sshmux terminal-proxy on [--http 127.0.0.1:7897] [--socks 127.0.0.1:7897]` — 开启并写入配置
- `sshmux terminal-proxy off` — 关闭并删除 `~/.sshmux/proxy.env`
- `sshmux terminal-proxy status` — 显示当前开关状态和配置的地址
- 首次运行 `on` 时，检测并向 `~/.zshrc` / `~/.bash_profile` 追加 source 行（幂等）
- 配置持久化到 `~/.sshmux/terminal-proxy.json`，下次 `on` 无需重复指定地址
- HTTP 和 SOCKS 地址可以相同也可以不同；任一可省略（省略则不写对应 env var）

---

## proxy.env 内容

根据用户配置动态生成，只写用户指定的部分：

```bash
# 若 --http 已配置
export http_proxy="http://127.0.0.1:7897"
export https_proxy="http://127.0.0.1:7897"
export HTTP_PROXY="http://127.0.0.1:7897"
export HTTPS_PROXY="http://127.0.0.1:7897"

# 若 --socks 已配置
export all_proxy="socks5://127.0.0.1:7897"
export ALL_PROXY="socks5://127.0.0.1:7897"

# 始终写入
export no_proxy="localhost,127.0.0.1"
export NO_PROXY="localhost,127.0.0.1"
```

---

## Shell Profile 修改

在 `~/.zshrc`（和 `~/.bash_profile`）中追加（幂等，已存在则不重复写）：

```bash
# sshmux: terminal proxy
[ -f "$HOME/.sshmux/proxy.env" ] && source "$HOME/.sshmux/proxy.env"
```

---

## 配置文件 `~/.sshmux/terminal-proxy.json`

```json
{
  "enabled": true,
  "http_addr": "127.0.0.1:7897",
  "socks_addr": "127.0.0.1:7897"
}
```

字段为空字符串表示未配置该类型代理。

---

## Acceptance Criteria

- [ ] `sshmux terminal-proxy on --http 127.0.0.1:7897 --socks 127.0.0.1:7897` → proxy.env 生成，含全部 env var
- [ ] `sshmux terminal-proxy on --http 127.0.0.1:8080 --socks 127.0.0.1:1080` → HTTP 和 SOCKS 用不同端口
- [ ] `sshmux terminal-proxy on --http 127.0.0.1:7897`（不带 --socks）→ 只生成 HTTP_PROXY 类 var，无 ALL_PROXY
- [ ] 新开终端 `echo $http_proxy` → `http://127.0.0.1:7897`
- [ ] `sshmux terminal-proxy off` → proxy.env 被删除，新开终端无代理 env
- [ ] 重复运行 `on` → `~/.zshrc` 中 source 行不重复追加
- [ ] `~/.zshrc` 不存在时自动创建
- [ ] `sshmux terminal-proxy status` → 打印当前状态和地址

---

## Definition of Done

- 单元测试：proxy.env 生成逻辑、shell profile 幂等追加、on/off 状态切换
- `go fmt / vet / test / golangci-lint` 通过
- `sshmux --help` 中 terminal-proxy 子命令有描述

---

## Out of Scope

- 已开启的终端会话自动更新（需 IPC，超出 MVP）
- Fish shell 支持（语法不同：`set -x`，后续 enhancement）
- 与 SSH host 连接状态的联动（本功能完全解耦）

---

## Decision (ADR-lite)

**Context**: 代理地址的来源和开关的粒度

**Decision**:
1. 全局 Mac 级开关，与 SSH host 连接完全解耦
2. 用户手动配置本地代理地址（可来自任何代理工具：Clash/Surge/sshmux 自身等）
3. HTTP 和 SOCKS 可分别指定不同地址
4. 开启时写 proxy.env，关闭时删除（Option B: 跟随开关状态，不跟随代理运行状态）

**Consequences**:
- 实现极简：sshmux 不需要感知任何代理工具的运行状态
- 用户灵活：可以和 sshmux 的 SOCKS/HTTP proxy 配合，也可以和第三方代理工具（Clash/Surge）配合
- 一次配置，重复 `on/off` 无需再次指定地址

---

## Technical Notes

- 配置文件：`~/.sshmux/terminal-proxy.json`（全局，非 per-host）
- env 文件：`~/.sshmux/proxy.env`
- Shell profile 目标：`~/.zshrc`（zsh，macOS 默认）+ `~/.bash_profile`（bash）
- 追加标记行：`# sshmux: terminal proxy`（用于幂等检测）
- 参考：nvm/pyenv 的 shell profile 注入模式
