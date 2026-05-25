# Claude Token Monitor

[![Claude Plugin](https://img.shields.io/badge/Claude_Code-Plugin-blueviolet)](https://github.com/young1lin/claude-token-monitor)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/young1lin/claude-token-monitor)](https://github.com/young1lin/claude-token-monitor/releases)
[![Coverage](https://codecov.io/gh/young1lin/claude-token-monitor/branch/main/graph/badge.svg)](https://codecov.io/gh/young1lin/claude-token-monitor)
[![Go Report Card](https://goreportcard.com/badge/github.com/young1lin/claude-token-monitor)](https://goreportcard.com/report/github.com/young1lin/claude-token-monitor)
[![Test](https://github.com/young1lin/claude-token-monitor/actions/workflows/test.yml/badge.svg)](https://github.com/young1lin/claude-token-monitor/actions/workflows/test.yml)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-blue)](https://github.com/young1lin/claude-token-monitor/releases)
[![Downloads](https://img.shields.io/github/downloads/young1lin/claude-token-monitor/total)](https://github.com/young1lin/claude-token-monitor/releases)

Claude Code 实时 Token 使用状态栏插件。

![](./images/claude-code-monitor-team.png)

## 安装

```bash
/plugin marketplace add young1lin/claude-token-monitor
/plugin install claude-token-monitor@claude-token-monitor
/reload-plugins
/claude-token-monitor:setup
```

## 配置

在项目中创建 `.claude/statusline.yml`（`.yml` 优先，也兼容 `.yaml`；放到 `~/.claude/` 下即为全局配置）：

```yaml
display:
  singleLine: false  # 单行模式
  hide:              # 隐藏项
    - claude-version
    - memory-files

format:
  progressBar: braille  # "braille" 或 "ascii"
  timeFormat: 24h       # "12h" 或 "24h"
  compact: false

# 网络配置（v0.2.1+）
# 仅作用于对 api.anthropic.com 的 OAuth usage 请求，
# 其他 HTTP 流量永远不走代理；HTTP_PROXY / HTTPS_PROXY 也会被忽略。
# 优先级：--proxy CLI 参数 > STATUSLINE_CLAUDE_PROXY 环境变量 > 本字段
network:
  # 留空 = 直连。支持 http / https / socks5（socks5h 亦可）。
  # 用户名/密码直接写在 URL 用户信息部分（需 URL-encode）：
  #   http://alice:p%40ss@127.0.0.1:7890
  #   socks5://bob:secret@127.0.0.1:1080
  claudeAPIProxy: ""

# 缓存配置（v0.2.1+）
cache:
  # usage/quota 响应的成功缓存秒数。默认 90s ≈ 每小时最多 40 次请求。
  # 失败缓存 (15s) 与 429 退避 (60→120→240s, 上限 5min) 不可配置。
  # 429 响应带 Retry-After 时优先遵守服务端返回值。
  usageTTLSeconds: 90

content:
  composers:
    - name: my-token
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"
  use:
    token: my-token
```

> 不想手写？运行 `/claude-token-monitor:setup`，里面有交互式代理向导（启用？→ 协议 → host:port → 是否鉴权 → 用户名/密码），会自动写入 `.claude/statusline.yml`。该文件已加入 `.gitignore`，凭据不会进仓库。

### GLM / Z.ai 配额显示

当 `ANTHROPIC_BASE_URL` 指向 `api.z.ai`、`open.bigmodel.cn` 或 `dev.bigmodel.cn` 时，订阅配额行会改用 GLM monitor quota 接口。输出会显示套餐标签（`[Max]` / `[Pro]` / `[Lite]`）、5h / 7d token 窗口，以及 GLM Coding Plan 的 MCP 月度调用量（如 `MCP 380/4k`）。Anthropic 账号没有 MCP 配额，不会显示 MCP 段。

GLM 缓存按 `provider + ANTHROPIC_AUTH_TOKEN` 指纹分文件保存；同一机器上切换 Pro / Lite 或不同 Z.ai / 智谱账号时，不会互相复用旧 quota。遇到 429 会优先遵守服务端 `Retry-After`。

![](./images/claude-code-monitor-glm.png)

## 扩展开发

在 `internal/statusline/content/` 中创建新的收集器：

```go
type MyCollector struct {
    *content.BaseCollector
}

func (c *MyCollector) Collect(input, summary) (string, error) {
    return "my data", nil
}
```

在 `main.go` 中注册，并在 `layout/grid.go` 中添加到布局。

## 工作原理

状态栏插件采用**无状态 stdin/stdout** 执行模型。Claude Code 每次刷新时启动插件子进程，通过 stdin 写入 JSON 数据，从 stdout 读取格式化后的状态文本。

```
+-------------------+          +--------------------+          +------------------+
|                   |  spawn   |                    |  exit 0  |                  |
|    Claude Code    +--------->|   statusline.exe   +--------->|   Process Ends   |
|   (main process)  |          |  (child process)   |          |   (cleanup)      |
|                   |          |                    |          |                  |
+--------+----------+          +----+----------+----+          +------------------+
         |                          |          |
         |  stdin (JSON)            |          |  stdout (text)
         v                          |          v
+-------------------+          +----+----------+----+
| {                 |          | Parsed output:     |
|   "cwd": "...",   |          |                    |
|   "model": {...}, |   --->   | [Model] [===---]   |
|   "context_window"|          |  75K/200K (37.5%)  |
|   ...             |          |                    |
| }                 |          +--------------------+
+-------------------+
```

### Execution Flow

```
Claude Code                          statusline.exe
    |                                      |
    |  1. Spawn process                    |
    +------------------------------------->|
    |                                      |
    |  2. Write JSON to stdin              |
    +------------------------------------->|
    |                                      |
    |                            3. Parse JSON input
    |                            4. Collect data:
    |                               - Token usage
    |                               - Git branch & status
    |                               - Tool calls (from transcript)
    |                               - Agent info
    |                               - TODO progress
    |                            5. Format output string
    |                                      |
    |  6. Read stdout                      |
    |<-------------------------------------+
    |                                      |
    |  7. Display in status bar    8. Exit |
    |                                      X
```

### Input (stdin)

Claude Code 通过 stdin 发送 JSON 数据：

```json
{
  "cwd": "C:\\Project",
  "model": {
    "display_name": "Claude Sonnet 4.5",
    "id": "claude-sonnet-4-5-20250514"
  },
  "context_window": {
    "context_window_size": 200000,
    "current_usage": {
      "input_tokens": 93,
      "output_tokens": 68,
      "cache_read_input_tokens": 103040
    }
  },
  "transcript_path": "/home/user/.claude/projects/.../session.jsonl",
  "workspace": {
    "current_dir": "C:\\Project",
    "project_dir": "C:\\Project"
  }
}
```

### Output (stdout)

插件向 stdout 输出一行或多行纯文本（可包含 ANSI 颜色代码）。默认是 4 行 grid 布局，单元从左到右、从上到下依次为：项目目录、模型 + token 进度条、Claude Code 版本号、Git 分支、CLAUDE.md / rules 计数、本次会话花费与 I/O token、当前时间、订阅配额（5h / 7d 重置倒计时）、进程常驻内存、工具调用记录。

```
📁 claude-token-monitor | [Opus 4.7 (1M context) [░░░░░░░░░░] 59.6K/1000K (6.0%)] | v2.1.143
🌿 main                 | 📦 2 CLAUDE.md + 2 rules                                | 💰 $0.53 · I:60.6K O:78
🕐 2026-05-17 13:27     | 📊 [Team] 52% 5h ↻ 1h25m · 17% 7d ↻ 6d14h              | 💾 294.0 MB
✓ Read(9) ✓ Grep(5) ✓ Glob(2) ✖ Bash(1)
```

| 字段 | 含义 |
|------|------|
| `📁 claude-token-monitor` | 当前工作目录名 |
| `[Opus 4.7 (1M context) [░░░░░░░░░░] 59.6K/1000K (6.0%)]` | 模型 + 上下文 token 进度条 |
| `v2.1.143` | Claude Code 版本 |
| `🌿 main` | Git 分支（带 `+新增 ~修改 -删除` 时显示文件改动统计） |
| `📦 2 CLAUDE.md + 2 rules` | 当前作用域命中的 CLAUDE.md 与规则文件数 |
| `💰 $0.53 · I:60.6K O:78` | 当前会话累计费用、输入 / 输出 token |
| `🕐 2026-05-17 13:27` | 当前日期时间（`format.timeFormat` 控制 12/24h） |
| `📊 [Team] 52% 5h ↻ 1h25m · 17% 7d ↻ 6d14h` | 订阅配额：套餐、5h / 7d 用量百分比、距离下次重置的倒计时；GLM/Z.ai 账号会额外显示 MCP 月度调用量 |
| `💾 294.0 MB` | 当前 statusline 进程的常驻内存 |
| `✓ Read(9) ✓ Grep(5) ✖ Bash(1)` | 本会话工具调用次数，`✓` 成功 / `✖` 失败 |

#### 订阅配额展示对比

不同套餐 / provider 在配额行的展示差异：

**Anthropic Team** —— `[Team] 6% 5h ↻ 4h51m · 18% 7d ↻ 6d10h`

![](./images/claude-code-monitor-team.png)

**Anthropic Pro** —— `[Pro] 4% 5h ↻ 4h49m · 56% 7d ↻ 11h49m`

![](./images/claude-code-monitor-pro.png)

**GLM Coding Plan（Max）** —— `[Max] 1% 5h ↻ 3h23m · MCP 42/4k`，唯一带 MCP 月度调用量

![](./images/claude-code-monitor-glm.png)

### Why Hot Reload Works

由于插件**每次刷新都重新启动**，重新编译二进制文件后立即生效——无需重启 Claude Code。

```
  Time ─────────────────────────────────────────────────>

  v1.0 on disk          go build (v2.0)       v2.0 on disk
  ─────────────────────────┬──────────────────────────────
                           |
  Refresh #1               |          Refresh #2
  spawns v1.0              |          spawns v2.0
  ┌──────┐                 |          ┌──────┐
  │ v1.0 │ -> output       |          │ v2.0 │ -> new output
  └──────┘                 |          └──────┘
```

### Design Principles

1. **Stateless** — 没有常驻进程、IPC 或 socket，每次刷新独立运行。
2. **Fast** — 冷启动 30–50ms，热缓存命中 10–20ms；transcript 只读尾部。
3. **Safe** — 插件崩溃不会影响 Claude Code，只是不显示状态文本。
4. **Cross-platform** — 单个 Go 二进制，零外部依赖。
5. **Stable grid** — 默认输出使用固定列宽的 4 行 grid。这样每一格的语义位置保持稳定：模型、Git、花费、配额、内存等信息不会因为某个字段内容变长/变短而整体漂移，扫一眼就能知道每个位置显示的是什么。

> 终端建议：Windows 上推荐使用 Windows 11 自带的 Windows Terminal。它对 ANSI 颜色、emoji 和方块字符的宽度处理更一致，grid 对齐效果最好；旧版 cmd / PowerShell 或不同内嵌终端可能会因为字符宽度算法不同而出现轻微错位。

> 注：为了渲染订阅配额行，插件会向对应 provider 的 usage/quota 接口发送请求（成功响应默认 90s 缓存，失败 15s 缓存，遇 429 时优先遵守 `Retry-After`，否则使用 60→120→240s 指数退避，封顶 5min）。如果你在企业网或者跨境访问 `api.anthropic.com` 受限，请配置上面的 `network.claudeAPIProxy`。

### Debugging with `--debug`

使用 `--debug` 参数查看 Claude Code 发送给插件的确切 JSON 数据：

```bash
# In your Claude Code settings, temporarily add --debug:
"command": "C:\\\\path\\\\to\\\\statusline.exe --debug"
```

启用 `--debug` 后，插件会将原始 JSON 输入写入二进制文件所在目录的 `statusline.debug` 文件：

```
+-------------------+       +--------------------+       +-------------------+
|                   | stdin  |                    | file  |                   |
|    Claude Code    +------->|  statusline.exe    +------>| statusline.debug  |
|                   | (JSON) |  --debug           |       | (raw JSON, 最近 20 条) |
+-------------------+       +--------+-----------+       +-------------------+
                                      |
                                      | stdout (正常输出不受影响)
                                      v
                             +--------------------+
                             | [Model] [===---]   |
                             |  75K/200K (37.5%)  |
                             +--------------------+
```

调试文件保留最近 **20 条**记录（最多 40 行），新记录从顶部追加。每条 = **1 行时间戳 + 1 行原始 JSON**（不格式化、无分隔符），用户家目录会被替换为 `~` 以避免泄漏个人信息：

```
2026-05-17 13:27:01
{"session_id":"...","transcript_path":"~\\.claude\\projects\\...","cwd":"C:\\Project","model":{"display_name":"Opus 4.7 (1M context)","id":"claude-opus-4-7"},"context_window":{...}}
2026-05-17 13:26:45
{"session_id":"...","transcript_path":"~\\.claude\\projects\\...","cwd":"C:\\Project",...}
```

用途：
- 验证 Claude Code 实际提供的字段
- 检查 token 值是否与 `/context` 命令显示一致
- 诊断状态栏显示异常数据时的解析问题

## 更新

### 更新插件（命令和技能）

通过 marketplace 安装的用户，更新到最新版本：

```bash
/plugin update claude-token-monitor@claude-token-monitor
```

或通过 CLI：

```bash
claude plugin update claude-token-monitor@claude-token-monitor
```

**更新内容：**
- `/setup` 命令
- `/commit-push` 命令
- `/release-github` 命令
- 插件包含的其他技能或代理

**插件缓存位置：**

| 平台 | 路径 |
|------|------|
| Windows | `C:/Users/<用户名>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<版本>/` |
| macOS | `/Users/<用户名>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<版本>/` |
| Linux | `/home/<用户名>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<版本>/` |

### 更新 Statusline 二进制文件

`/setup` 命令会自动处理二进制文件更新：

1. 执行 `/setup` 或 `/claude-token-monitor:setup`
2. 检查本地版本与 GitHub 最新发布版本
3. 如有新版本，自动下载并安装更新

### 手动更新二进制文件

如需手动更新：

```bash
# Check current version
~/.claude/statusline --version

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_windows_amd64.zip" -OutFile "$env:TEMP\statusline.zip"
Expand-Archive -Path "$env:TEMP\statusline.zip" -DestinationPath "$env:USERPROFILE\.claude\" -Force
Remove-Item "$env:TEMP\statusline.zip"

# macOS
curl -L "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_darwin_$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/').tar.gz" | tar -xz -C "$HOME/.claude/"

# Linux
curl -L "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_linux_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xz -C "$HOME/.claude/"
```

### 启用自动更新

启用启动时自动更新插件：

1. 执行 `/plugin`
2. 进入 **Marketplaces** 标签页
3. 选择 `claude-token-monitor` marketplace
4. 启用 **Auto-update**

或通过 CLI：

```bash
claude plugin marketplace update claude-token-monitor --auto-update true
```

---

[English Documentation](./README.en-US.md)
