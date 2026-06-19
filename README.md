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

## 新功能（v0.2.8）

### Stdin 快路径（避开 OAuth 限流）

Claude Code 2.1.x 起在 stdin 里直接给出 `rate_limits`（5h / 7d 配额）和 `version`。状态栏现在优先消费这两个字段：

- **Anthropic 配额**：不再调 `https://api.anthropic.com/api/oauth/usage`，省一次请求、彻底规避 429 退避。`[Max]` / `[Pro]` / `[Team]` 标签仍从 `.credentials.json` 读取
- **`v2.1.150`**：直接回显，省掉每次 fork 一次 `claude --version` 子进程
- 老版 CC 没发这些字段时自动降级到原 API 路径，GLM 路径不变（CC 不替 GLM 发 quota）

### 模式指示器（Mode Flags）

Token 单元格末尾会显示当前会话的运行时状态：

```
[Opus 4.7 (1M context) [██░░░] 282K/1M (28.2%)] 💭 xhigh
                                                  ↑↑↑↑↑↑↑↑
                                       thinking + 紫色 xhigh effort
```

| 标志 | 含义 | 显示条件 |
|---|---|---|
| `💭` | extended thinking | `thinking.enabled == true` |
| `⚡` | fast mode | `fast_mode == true` |
| `max`/`xhigh` 紫 / `high` 黄 / `medium` 青 / `low` 绿 | effort 档位 | CC 上报 `effort.level` 时（含 medium） |

thinking、fast、effort 都没有内容时该 chip 隐藏（旧版 CC 不发 `effort.level` 时不显示档位）。

> **v0.2.8**：时间格新增空闲提示 `⏰ Xm`（超过 5 分钟无 transcript 活动时显示，阈值可配置/可关闭，见下方配置）；额度重置倒计时最后一分钟改显示秒数（`↻ 56s` 而非 `<1m`）。

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
  idleWarnSeconds: 300  # 闲置多少秒后时间格显示 ⏰ 提示（默认 300=5 分钟）。0 关闭。
                        # 也可用 STATUSLINE_IDLE_WARN_SECONDS 环境变量覆盖（优先级更高）。

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

当 `ANTHROPIC_BASE_URL` 指向 `api.z.ai`、`open.bigmodel.cn` 或 `dev.bigmodel.cn` 时，订阅配额行会改用 GLM monitor quota 接口。输出会显示套餐标签（`[Max]` / `[Pro]` / `[Lite]`）、5h / 7d token 窗口，以及 GLM Coding Plan 的 MCP 月度调用量（如 `🧩 380/4k`，🧩 代表 MCP 这类可插拔工具）。Anthropic 账号没有 MCP 配额，不会显示 🧩 段。

GLM 缓存按 `provider + ANTHROPIC_AUTH_TOKEN` 指纹分文件保存；同一机器上切换 Pro / Lite 或不同 Z.ai / 智谱账号时，不会互相复用旧 quota。遇到 429 会优先遵守服务端 `Retry-After`。

![](./images/claude-code-monitor-glm.png)

## Git Worktree 支持

### 状态栏如何显示 worktree

当前工作目录是一个 **linked worktree**（而非主检出）时，Git 分支格的叶子图标 `🌿` 会自动换成树图标 `🌳`：

```
🌿 main                      ← 主检出
🌳 feat/git-worktree-icon    ← 你在某个 worktree 里
```

- `🌳` 后面显示的始终是该 worktree **检出的分支名**，不是目录名；worktree 的目录名已经由 `📁` 格展示，所以不重复。
- 检测方式：`git rev-parse --git-dir --git-common-dir` —— 主检出两者相等，linked worktree 的 git-dir 指向 `.git/worktrees/<name>` 而 common-dir 仍指向主 `.git`，两者不等即判定为 worktree。检测并入已有的并行 git 采集 + 5s 缓存，无额外子进程开销。

### 什么是 git worktree

一个仓库可以同时检出**多个工作目录**，每个目录在不同分支上、文件互相独立，但共享同一份 `.git` 历史。常用于「不打断当前分支的情况下，并行开另一个分支干活」。

> 核心约束：**同一个分支不能被两个 worktree 同时检出**，所以每个 worktree 都需要自己的分支。切 worktree = 切目录，没有「原地切换」。

### Claude Code 的 worktree 工具

Claude Code 内置两个工具，可以把**当前会话的工作目录**切进 / 切出 worktree：

| 工具 | 作用 | 关键参数 |
|------|------|---------|
| `EnterWorktree` | 新建一个 worktree 并把会话切进去；或切进一个**已存在**的 worktree | `name`（新建，分支建在 `.claude/worktrees/` 下）/ `path`（进入已有 worktree，须在 `git worktree list` 里）|
| `ExitWorktree` | 离开 worktree，会话切回原目录 | `action: keep`（保留 worktree 和分支）/ `action: remove`（删目录和分支；有未提交内容时需 `discard_changes: true`）|

行为要点：

- 新建 worktree 的基准分支由 `worktree.baseRef` 设置决定：`fresh`（默认，从 `origin/<默认分支>` 拉）或 `head`（从当前本地 HEAD 拉）。
- `ExitWorktree` **只清理由本会话 `EnterWorktree` 新建的 worktree**。对于你手动 `git worktree add` 创建、再用 `EnterWorktree path` 进去的 worktree，`ExitWorktree` 不会删它——只能用 `action: keep` 切回去，输出形如：

  ```
  Exiting worktree
  ⎿  Kept worktree (branch feat/git-worktree-icon)
  ```

### 完整工作流（手动 git + Claude Code 工具）

```bash
# ① 创建：平级目录新建 worktree + 新分支（不能复用被主检出占用的 main）
git worktree add -b feat/my-feature ../my-feature

# ② 进入：把当前 Claude Code 会话切进去（路径须在 git worktree list 里）
#    → 调用 EnterWorktree(path: "<worktree 绝对路径>")
#    此后状态栏显示 🌳 feat/my-feature，编辑/构建都发生在这个目录

# ③ 提交：在 worktree 里正常提交到 feature 分支
git add -u && git commit -m "feat: ..."

# ④ 合并：回主检出执行（worktree 里不能 checkout main，main 被它占用）
git -C <主检出目录> merge --ff-only feat/my-feature

# ⑤ 收尾：先 ExitWorktree(keep) 切回主检出，再删 worktree 和已合并的分支
git worktree remove ../my-feature
git branch -d feat/my-feature
```

> 记住核心：**worktree 里干活、提交；合并进主分支要回主检出目录做。**

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
| `🌿 main` | Git 分支（带 `+新增 ~修改 -删除` 时显示文件改动统计）；当 cwd 是 linked worktree 时图标变为 `🌳`，详见 [Git Worktree 支持](#git-worktree-支持) |
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

**GLM Coding Plan（Max）** —— `[Max] 1% 5h ↻ 3h23m · 🧩 42/4k`，唯一带 MCP 月度调用量（🧩 表示 MCP 工具调用）

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

## 实现方案对比与性能基准

statusline 采用 **"fire-and-forget"** 执行模型：Claude Code 每次刷新状态栏都会**重新拉起一个新进程**。这意味着语言运行时的**启动开销**被放大到了每一次调用——这是评估任何替代实现时最关键的指标。

为验证"为什么主力分发物是 Go 编译的原生二进制，而非脚本"，本项目实测了 5 种实现。所有数据均在 **Windows 11 + 同一终端**下取得，使用同一份 stdin 输入（`test_input2.json`）、同一 git 工作区状态，每个实现连续跑 7 次取平均（已 warmup，取稳态值）。

### 性能基准

| 实现 | 平均延迟 | min / max | 相对 Go | 运行时依赖 | 平台 |
|------|---------|-----------|---------|-----------|------|
| **Go 原生二进制** | **156 ms** | 151 / 162 | 1× | 无（单文件） | 全平台 |
| Node.js (`nodejs/`) | 176 ms | 172 / 179 | ×1.13 | 需 Node.js ≥ 18 | 全平台 |
| Python (`python/`) | 193 ms | 187 / 204 | ×1.24 | 需 Python 3.8+ | 全平台 |
| PowerShell | ~2130 ms | — | ×13.7 | 需 pwsh | Windows 为主 |
| Bash (`statusline/statusline.sh`) | 13312 ms | 10037 / 17861 | ×85 | 需 bash + jq | Linux/macOS 为主 |

> 四者共享约 75ms 的 `git` 子进程开销，因此 Go / Node / Python 都落在 150–200ms 区间，差异主要来自语言本身的启动与解析/渲染。Bash / PowerShell 的巨大差距则源于运行时的进程启动模型。

### 为什么不用 Bash 脚本

- **Git Bash 即 msys2**，其模拟的 POSIX `fork/exec` 是性能黑洞——每次 spawn 子进程（`jq` / `git` / `grep` / `sed` / `date`）的开销是原生 Linux 的几十倍。
- Bash 版重度依赖外部命令，每次刷新要 fork 数十次，实测 **13.3 秒**（慢 85×），**Windows 下完全不可用**。
- 原生 Linux / macOS 上 fork 是微秒级，Bash 版会快得多；但本项目主力用户在 Windows，msys2 直接出局。
- `statusline/statusline.sh` 仅作为**参考实现**保留，不作为分发物。

### 为什么不用 PowerShell 脚本

- 每次启动要加载 **.NET CLR**，纯启动就 **0.4–1.4s**——fire-and-forget 模型下这是致命的、无法优化的固定开销。
- 实测 **2.1 秒**（慢 13.7×），且 pwsh 以 Windows 为主，macOS / Linux 默认不带。
- 早期的 `*.ps1` 单文件原型与模块化版本均已从项目中**移除**。

### 为什么不用 Node.js

- 性能其实**接近 Go**（176ms，仅慢 13%）——V8 启动约 90ms，远好于 .NET CLR，是脚本语言里最有希望逼近原生的。
- **但用户电脑可能没有安装 Node.js**。statusline 的核心价值之一是"下载一个二进制就能用"，引入 Node.js 运行时会破坏这一零依赖分发模型，也增加版本与环境管理负担。
- `nodejs/` 目录作为**分层架构的参考实现**保留（零 npm 依赖，仅用标准库），便于对照阅读与教学，但不作为主力分发。

### 为什么不用 Python

1. **慢**：193ms（慢 24%）。CPython 是解释执行，叠加 `git` subprocess 调用，比 Node.js 还慢。
2. **用户电脑可能没装 Python**：同样是运行时依赖问题，且 Python 版本碎片化严重（3.8 / 3.12 / 3.13 并存），shebang 与包管理在 Windows 上尤其麻烦。

`python/` 目录同样作为**参考实现**保留（零第三方依赖，仅用标准库）。

### 结论

Go 原生二进制是**唯一同时满足**以下四点的方案：

- ✅ **最快**（156ms，warm 缓存命中可到 10–20ms）
- ✅ **零运行时依赖**（单文件，不要求用户预装任何运行时）
- ✅ **单文件分发**（`go build` 产出独立的 exe / ELF / Mach-O）
- ✅ **真跨平台**（Windows / macOS / Linux 原生编译，无需模拟层）

Node.js / Python 版虽性能可用，但都引入运行时依赖；Bash / PowerShell 版的性能差距已到不可用程度。因此**主力分发物固定为 Go 编译的原生二进制**，`nodejs/`、`python/`、`statusline/`（Bash）三个脚本目录仅作为分层架构的参考实现与教学对照保留。

复现基准测试：

```bash
# 同一 input、同一 git 状态下，各实现连续跑 7 次取平均
export TIMEFORMAT='%R'
for impl in "./statusline.exe" \
            "node nodejs/statusline.js" \
            "python python/statusline.py" \
            "bash statusline/statusline.sh"; do
  echo "--- $impl ---"
  for i in $(seq 1 7); do { time $impl < test_input2.json > /dev/null 2>&1; } 2>&1; done
done
```

---

[English Documentation](./README.en-US.md)
