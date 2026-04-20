**中文** | [English](./README_EN.md)

# sshmux

**macOS 本地 SSH 代理管理器**，让你用一个交互界面管理所有 SSH 连接和代理配置，彻底告别手敲命令。

---

## 安装

### 方式一：直接下载预编译二进制（无需 Go 环境）

```bash
curl -fsSL https://raw.githubusercontent.com/Xuanxuana1/sshmux/main/install.sh | bash
```

自动识别 Intel / Apple Silicon 架构，从 GitHub Releases 下载最新版本，安装到 `~/bin/sshmux`，并自动导入 SSH 主机。

> 安装完成后，`~/bin/sshmux` 是一个轻量 wrapper，真正的二进制位于 `~/.sshmux/bin/sshmux-real`。

确保 `~/bin` 在你的 `PATH` 里（如没有，在 `~/.zshrc` 加一行）：

```bash
export PATH="$HOME/bin:$PATH"
```

### 方式二：从源码编译（需要 Go 1.21+）

```bash
git clone https://github.com/Xuanxuana1/sshmux.git
cd sshmux
make install
```

**卸载**：

```bash
make uninstall
```

---

## 快速上手

```bash
sshmux
```

直接运行即可打开交互界面。

---

## 界面说明

启动后你会看到一张主机列表表格，每一行是一台服务器：

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

建立 SSH 连接后，SOCKS5（端口 7897）和 HTTP 代理（端口 7897）**自动启用**，无需手动开关。

### 列含义

| 列 | 含义 |
|----|------|
| **Host** | SSH 配置里的别名（来自 `~/.ssh/config`） |
| **SSH** | SSH 主连接状态。`* online` 表示已连接，`o offline` 表示未连接 |
| **Sync** | macOS 系统代理同步状态。`*` 表示已将代理同步到系统网络设置 |
| **RPx** | Remote Proxy 状态。将本机代理转发到远端服务器的环境变量中 |

---

## 键盘操作

| 按键 | 功能 |
|------|------|
| `↑` / `↓` 或 `k` / `j` | 上下移动光标，选择主机 |
| `c` | 切换所选主机的 SSH 连接（建立 / 断开）。`sshmux` 模式下会自动启动本地代理，`external` 模式下只建立 SSH |
| `m` | 切换 macOS 系统代理同步（需要先建立 SSH 连接；External 模式下不可用） |
| `r` | 切换 Remote Proxy（需要先建立 SSH 连接） |
| `u` | 编辑 Remote Source：`sshmux` 或外部本地代理（如龙猫云 `127.0.0.1:7897`） |
| `t` | 切换 Terminal Proxy（为当前终端会话注入 `http_proxy` / `https_proxy`） |
| `p` | 编辑代理端口（SOCKS + HTTP 全局共用，Tab 切换字段，Enter 确认） |
| `i` | 从 `~/.ssh/config` 导入所有主机 |
| `q` 或 `Ctrl+C` | 退出 |

---

## 典型使用流程

### 场景一：让远端服务器访问外网

远程服务器（如 GPU 机器）本身没有外网，但你的 Mac 有代理。sshmux 通过 SSH 反向端口转发，把本机代理自动注入到服务器的环境变量里：

1. 运行 `sshmux`，按 `u` 把 Remote Source 切到 `external`，地址填龙猫云本地代理（如 `127.0.0.1:7897`）
2. 按 `c` 建立 SSH 连接，按 `r` 开启 Remote Proxy
3. SSH 登录服务器后，`http_proxy` / `https_proxy` 等环境变量自动生效，`pip install`、`wget`、`curl` 直接可用
4. 若远端启用了 Docker，`remote-proxy` 会默认自动探测 `docker0` 网关，并额外暴露一个容器可达入口；Docker build / run 可直接用远端主机网关地址访问代理

CLI 也支持显式控制容器入口：

- `sshmux remote-proxy on <alias> --bind-address 172.17.0.1`：手动指定远端绑定地址
- `sshmux remote-proxy on <alias> --loopback-only`：保留旧行为，只绑定远端 `127.0.0.1`

### 场景二：给本地终端临时配置代理

需要在当前终端窗口走代理，但不想改动系统全局设置：

1. 按 `t` 开启 Terminal Proxy（默认地址 `127.0.0.1:7897`，首次启动自动初始化）
2. 当前终端的 `curl`、`git`、`npm` 等命令自动走代理
3. 用完按 `t` 关闭，其他终端和应用完全不受影响

### 场景三：同步到 macOS 系统代理

需要浏览器、Slack 等 GUI 应用也走代理：

1. 按 `u` 确认 Remote Source 为 `sshmux`
2. 按 `c` 建立 SSH 连接
3. 按 `m` 同步到 macOS 系统网络设置，全局生效
4. 用完再按 `m` 关闭，一键切回直连

### 场景四：多人协作 — 端口不一致时快速对齐

团队共用同一批服务器，每个人本地代理端口往往不同（有人用 7897，有人用 1080）。sshmux 把端口设计为**全局一处修改**，无需逐台服务器调整：

1. 按 `p` 打开端口编辑，填入自己本地的端口，Enter 确认，所有主机立即生效，Terminal Proxy 地址同步更新
2. 需要临时外网时按 `m` 开启系统代理，用完关闭，不影响其他人

---

## 第一次使用

首次运行时主机列表是空的，按 `i` 从 `~/.ssh/config` 自动导入所有主机配置，导入后即可开始操作。

---

## 状态持久化

所有开关状态都保存在 `~/.sshmux/` 下。重新打开界面时，状态会从磁盘恢复，不会丢失。

---

## CLI 子命令

除了交互界面，sshmux 也支持命令行直接操作，适合脚本集成：

```bash
sshmux hosts                                           # 列出所有主机
sshmux connect <alias>                                 # 建立 SSH 连接（代理自动启动）
sshmux disconnect <alias>                              # 断开 SSH 连接
sshmux sync enable <alias>                             # 同步到 macOS 系统代理
sshmux sync disable <alias>                            # 关闭系统代理同步
sshmux terminal-proxy on --http <addr> --socks <addr>  # 开启 Terminal Proxy（自定义地址，默认 127.0.0.1:7897）
sshmux terminal-proxy off                              # 关闭 Terminal Proxy
sshmux remote-proxy on <alias>                         # 开启 Remote Proxy（默认自动探测 Docker gateway）
sshmux remote-proxy on <alias> --bind-address <ip>     # 手动指定容器可达绑定地址
sshmux remote-proxy on <alias> --loopback-only         # 只保留远端 127.0.0.1，不暴露给容器
sshmux remote-proxy off <alias>                        # 关闭 Remote Proxy
sshmux remote-proxy status <alias>                     # 查看 shell / container 端点
sshmux import-hosts                                    # 从 ~/.ssh/config 导入主机
```

---

## 系统要求

- macOS（依赖 `networksetup` 管理系统代理）
- OpenSSH（macOS 自带）
