# Claude Token Monitor

Real-time token usage statusline for Claude Code.

![](./images/claude-code-monitor-team.png)

## Installation

```bash
/plugin marketplace add young1lin/claude-token-monitor
/plugin install claude-token-monitor@claude-token-monitor
/reload-plugins
/claude-token-monitor:setup
```

## What's New (v0.2.8)

### Stdin Fast Path (Skip the OAuth Rate Limit)

Starting with Claude Code 2.1.x, the stdin payload includes `rate_limits` (5h / 7d quotas) and `version`. The statusline now consumes those fields first:

- **Anthropic quota**: no more call to `https://api.anthropic.com/api/oauth/usage` — saves one request per refresh and side-steps 429 backoff entirely. The `[Max]` / `[Pro]` / `[Team]` plan tag is still read from `.credentials.json`.
- **`v2.1.150`**: echoed directly, eliminating the per-refresh `claude --version` subprocess fork.
- When older Claude Code builds don't emit those fields, the plugin transparently falls back to the original API path. GLM path is unchanged (CC doesn't proxy GLM quota).

### Mode Flags Indicator

The token cell now ends with a runtime-state chip:

```
[Opus 4.7 (1M context) [██░░░] 282K/1M (28.2%)] 💭 xhigh
                                                  ↑↑↑↑↑↑↑↑
                                       thinking + purple xhigh effort
```

| Flag | Meaning | Shown when |
|---|---|---|
| `💭` | extended thinking | `thinking.enabled == true` |
| `⚡` | fast mode | `fast_mode == true` |
| `max`/`xhigh` purple / `high` yellow / `medium` cyan / `low` green | effort level | when CC reports `effort.level` (incl. medium) |

The chip is hidden only when thinking, fast mode, and effort are all unreported.

> **v0.2.8**: the time cell now shows an idle/staleness marker `⏰ Xm` after 5 minutes of inactivity (threshold configurable / disableable — see Configuration below); the quota reset countdown shows seconds in its final minute (`↻ 56s` instead of `<1m`).

## Configuration

Create `.claude/statusline.yml` in your project (`.yml` is preferred but `.yaml` also works; drop it under your config dir for a global config — `~/.claude/` by default, or `$CLAUDE_CONFIG_DIR` such as `~/.claude-account-ME/` when set):

```yaml
display:
  singleLine: false  # Single-line mode
  hide:              # Hide items
    - claude-version
    - memory-files

format:
  progressBar: braille  # "braille" or "ascii"
  timeFormat: 24h       # "12h" or "24h"
  compact: false
  idleWarnSeconds: 300  # seconds of inactivity before the time cell shows a ⏰ marker (default 300 = 5 min). 0 disables.
                        # Overridable by the STATUSLINE_IDLE_WARN_SECONDS env var (higher precedence).

# Network (v0.2.1+).
# Applies ONLY to the OAuth-usage request to api.anthropic.com — all other
# HTTP traffic stays direct. HTTP_PROXY / HTTPS_PROXY env vars are
# intentionally ignored to avoid leakage from unrelated tools.
# Precedence: --proxy CLI flag > STATUSLINE_CLAUDE_PROXY env > this field.
network:
  # Empty = direct connection. http / https / socks5 (and socks5h) supported.
  # Username / password go inline in the URL user-info (URL-encoded):
  #   http://alice:p%40ss@127.0.0.1:7890
  #   socks5://bob:secret@127.0.0.1:1080
  claudeAPIProxy: ""

# Cache (v0.2.1+).
cache:
  # Seconds to cache a successful usage/quota response. Default 90s
  # (~40 requests/hour). Failure cache (15s) and 429 fallback backoff
  # (60→120→240s, capped at 5min) are NOT configurable.
  # When a 429 response includes Retry-After, that server value wins.
  usageTTLSeconds: 90

content:
  composers:
    - name: my-token
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"
  use:
    token: my-token
```

> Don't want to write the YAML by hand? Run `/claude-token-monitor:setup` — it ships with an interactive proxy wizard (enable? → protocol → host:port → auth? → username / password) that writes `.claude/statusline.yml` for you. The file is in `.gitignore`, so proxy credentials stay per-machine.

### GLM / Z.ai Quota Display

When `ANTHROPIC_BASE_URL` points to `api.z.ai`, `open.bigmodel.cn`, or `dev.bigmodel.cn`, the subscription quota line switches to the GLM monitor quota API. It shows the plan tag (`[Max]` / `[Pro]` / `[Lite]`), 5h / 7d token windows, and the GLM Coding Plan MCP monthly call budget, for example `🧩 380/4k` (🧩 stands in for MCP — pluggable tool calls). Anthropic accounts do not have an MCP quota, so no 🧩 segment is shown there.

GLM cache files are separated by `provider + ANTHROPIC_AUTH_TOKEN` fingerprint, so switching between Pro / Lite or different Z.ai / Zhipu accounts on the same machine does not reuse stale quota from another account. On HTTP 429, the plugin honors the server's `Retry-After` value first.

![](./images/claude-code-monitor-glm.png)

## Git Worktree Support

### How the statusline shows a worktree

When the current working directory is a **linked worktree** (rather than the main checkout), the leaf icon `🌿` in the Git branch cell automatically becomes a tree icon `🌳`:

```
🌿 main                      ← main checkout
🌳 feat/git-worktree-icon    ← you are inside a worktree
```

- The text after `🌳` is always the **branch checked out in that worktree**, not the directory name. The worktree's directory name already appears in the `📁` cell, so it is not duplicated.
- Detection: `git rev-parse --git-dir --git-common-dir` — in the main checkout the two are equal; in a linked worktree the git-dir points at `.git/worktrees/<name>` while the common-dir still points at the main `.git`. Any inequality marks a linked worktree. The check runs inside the existing parallel git fetch under the 5s cache, so there is no extra subprocess cost.

### What a git worktree is

A single repository can have **multiple working directories** checked out at once, each on a different branch with independent files, all sharing the same `.git` history. It is typically used to "work on another branch in parallel without disturbing the current one."

> Core constraint: **the same branch cannot be checked out in two worktrees at once**, so every worktree needs its own branch. Switching worktree == switching directory; there is no in-place switch.

### Claude Code's worktree tools

Claude Code ships two tools that move the **current session's working directory** in and out of a worktree:

| Tool | Purpose | Key parameters |
|------|---------|----------------|
| `EnterWorktree` | Create a new worktree and switch the session into it; or switch into an **existing** worktree | `name` (create new; branch lives under `.claude/worktrees/`) / `path` (enter an existing worktree; must appear in `git worktree list`) |
| `ExitWorktree` | Leave the worktree and return the session to the original directory | `action: keep` (leave worktree + branch on disk) / `action: remove` (delete the directory and branch; needs `discard_changes: true` if there are uncommitted changes) |

Behavior notes:

- The base branch for a new worktree is governed by the `worktree.baseRef` setting: `fresh` (default, branches from `origin/<default-branch>`) or `head` (branches from the current local HEAD).
- `ExitWorktree` **only cleans up worktrees created by `EnterWorktree` in this session**. For a worktree you created manually with `git worktree add` and then entered via `EnterWorktree path`, `ExitWorktree` will not delete it — use `action: keep` to switch back. The output looks like:

  ```
  Exiting worktree
  ⎿  Kept worktree (branch feat/git-worktree-icon)
  ```

### Full workflow (manual git + Claude Code tools)

```bash
# 1. Create: a sibling worktree on a new branch (you cannot reuse main, which the main checkout holds)
git worktree add -b feat/my-feature ../my-feature

# 2. Enter: switch the current Claude Code session into it (path must be in git worktree list)
#    -> call EnterWorktree(path: "<absolute worktree path>")
#    The statusline now shows 🌳 feat/my-feature; edits/builds happen in this directory

# 3. Commit: commit normally onto the feature branch inside the worktree
git add -u && git commit -m "feat: ..."

# 4. Merge: run from the main checkout (you cannot checkout main inside the worktree, it is held there)
git -C <main-checkout-dir> merge --ff-only feat/my-feature

# 5. Clean up: ExitWorktree(keep) back to the main checkout, then remove the worktree and merged branch
git worktree remove ../my-feature
git branch -d feat/my-feature
```

> Remember the core rule: **work and commit inside the worktree; run the merge into the main branch from the main checkout.**

## Extending

Add new content by creating a collector in `internal/statusline/content/`:

```go
type MyCollector struct {
    *content.BaseCollector
}

func (c *MyCollector) Collect(input, summary) (string, error) {
    return "my data", nil
}
```

Register in `main.go` and add to layout in `layout/grid.go`.

## How It Works

The statusline plugin follows a **stateless stdin/stdout** execution model. Claude Code spawns the plugin as a child process on every refresh, writes a JSON payload to stdin, and reads the formatted status text from stdout.

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

Claude Code sends a single JSON object via stdin:

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

The plugin writes one or more lines of plain text (with optional ANSI color codes) to stdout. The default layout is a 4-row grid covering: project folder, model + token progress bar, Claude Code version, git branch, CLAUDE.md / rules counts, session cost & I/O tokens, current time, subscription quota (5h / 7d reset countdowns), resident memory, and tool-call tally.

```
📁 claude-token-monitor | [Opus 4.7 (1M context) [░░░░░░░░░░] 59.6K/1000K (6.0%)] | v2.1.143
🌿 main                 | 📦 2 CLAUDE.md + 2 rules                                | 💰 $0.53 · I:60.6K O:78
🕐 2026-05-17 13:27     | 📊 [Team] 52% 5h ↻ 1h25m · 17% 7d ↻ 6d14h              | 💾 294.0 MB
✓ Read(9) ✓ Grep(5) ✓ Glob(2) ✖ Bash(1)
```

| Field | Meaning |
|-------|---------|
| `📁 claude-token-monitor` | Current working directory name |
| `[Opus 4.7 (1M context) [░░░░░░░░░░] 59.6K/1000K (6.0%)]` | Model + context-token progress bar |
| `v2.1.143` | Claude Code version |
| `🌿 main` | Git branch (adds `+new ~modified -deleted` when there are unstaged changes); the icon becomes `🌳` when the cwd is a linked worktree — see [Git Worktree Support](#git-worktree-support) |
| `📦 2 CLAUDE.md + 2 rules` | Number of CLAUDE.md / rules files in scope |
| `💰 $0.53 · I:60.6K O:78` | Session-cumulative cost and input/output tokens |
| `🕐 2026-05-17 13:27` | Date + time (12h / 24h controlled by `format.timeFormat`) |
| `📊 [Team] 52% 5h ↻ 1h25m · 17% 7d ↻ 6d14h` | Subscription quota: plan, 5h / 7d utilization, countdowns to reset; GLM/Z.ai accounts additionally show the MCP monthly call budget |
| `💾 294.0 MB` | Resident memory of the statusline process |
| `✓ Read(9) ✓ Grep(5) ✖ Bash(1)` | Tool calls this session — `✓` succeeded, `✖` failed |

#### Subscription Quota — Side-by-Side

How the quota row differs across plans / providers:

**Anthropic Team** — `[Team] 6% 5h ↻ 4h51m · 18% 7d ↻ 6d10h`

![](./images/claude-code-monitor-team.png)

**Anthropic Pro** — `[Pro] 4% 5h ↻ 4h49m · 56% 7d ↻ 11h49m`

![](./images/claude-code-monitor-pro.png)

**GLM Coding Plan (Max)** — `[Max] 1% 5h ↻ 3h23m · 🧩 42/4k`, the only one that surfaces an MCP monthly call budget (🧩 = MCP tool calls)

![](./images/claude-code-monitor-glm.png)

### Why Hot Reload Works

Because the plugin is **spawned fresh on every refresh**, recompiling the binary takes effect immediately — no restart of Claude Code needed.

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

1. **Stateless** — No persistent process, no IPC, no sockets. Each invocation is independent.
2. **Fast** — Cold start 30–50ms, warm cache 10–20ms; transcript is read tail-only.
3. **Safe** — A crash in the plugin does not affect Claude Code. It simply shows no status text.
4. **Cross-platform** — Single Go binary with no external dependencies.
5. **Stable grid** — The default output uses a fixed-width 4-row grid. Each cell keeps the same semantic position, so model, git, cost, quota, memory, and tool-call data do not drift around when one field gets longer or shorter. You can glance at the statusline and know what each position means.

> Terminal recommendation: on Windows, use the Windows 11 built-in Windows Terminal for the best alignment. Its ANSI color, emoji, and block-character width handling is more consistent; older cmd / PowerShell hosts or embedded terminals may show slight column drift because they use different character-width rules.

> Note: To render the subscription quota line, the plugin issues a usage/quota request to the active provider (successful responses are cached for 90s by default, failures for 15s; 429s honor `Retry-After` when present, otherwise fall back to 60→120→240s exponential backoff capped at 5min). If access to `api.anthropic.com` is blocked by a firewall or geo-restriction, configure `network.claudeAPIProxy` above.

### Debugging with `--debug`

To inspect the exact JSON that Claude Code sends to the plugin, run with the `--debug` flag:

```bash
# In your Claude Code settings, temporarily add --debug:
"command": "C:\\\\path\\\\to\\\\statusline.exe --debug"
```

When `--debug` is enabled, the plugin writes the raw JSON input to a file called `statusline.debug` in the same directory as the binary:

```
+-------------------+       +--------------------+       +-------------------------+
|                   | stdin  |                    | file  |                         |
|    Claude Code    +------->|  statusline.exe    +------>| statusline.debug        |
|                   | (JSON) |  --debug           |       | (raw JSON, last 20)     |
+-------------------+       +--------+-----------+       +-------------------------+
                                      |
                                      | stdout (normal output continues)
                                      v
                             +--------------------+
                             | [Model] [===---]   |
                             |  75K/200K (37.5%)  |
                             +--------------------+
```

The file keeps the **last 20 entries** (max 40 lines), newest at the top. Each entry is **one timestamp line + one raw JSON line** (no pretty-printing, no separators). Your home directory is replaced with `~` before writing:

```
2026-05-17 13:27:01
{"session_id":"...","transcript_path":"~\\.claude\\projects\\...","cwd":"C:\\Project","model":{"display_name":"Opus 4.7 (1M context)","id":"claude-opus-4-7"},"context_window":{...}}
2026-05-17 13:26:45
{"session_id":"...","transcript_path":"~\\.claude\\projects\\...","cwd":"C:\\Project",...}
```

This is useful for:

- Verifying which fields Claude Code actually provides
- Checking token values match what `/context` reports
- Diagnosing parsing issues when the status bar shows unexpected data

## Updating

### Update Plugin (Commands & Skills)

If you installed via marketplace, update to the latest version:

```bash
/plugin update claude-token-monitor@claude-token-monitor
```

Or via CLI:

```bash
claude plugin update claude-token-monitor@claude-token-monitor
```

**What gets updated:**
- `/setup` command
- `/commit-push` command
- `/release-github` command
- Any skills or agents included in the plugin

**Where plugins are cached:**

| Platform | Path |
|----------|------|
| Windows | `C:/Users/<username>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<version>/` |
| macOS | `/Users/<username>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<version>/` |
| Linux | `/home/<username>/.claude/plugins/cache/claude-token-monitor/claude-token-monitor/<version>/` |

### Update Statusline Binary

The `/setup` command handles binary updates automatically:

1. Run `/setup` or `/claude-token-monitor:setup`
2. It checks your local version against the latest GitHub release
3. If a newer version exists, it downloads and installs the update

### Manual Binary Update

If you need to update the binary manually:

```bash
# First resolve the config dir: $CLAUDE_CONFIG_DIR if set, else ~/.claude
#   macOS/Linux
CC_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
#   Windows (PowerShell)
# $CC_DIR = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $env:USERPROFILE '.claude' }

# Check current version
"$CC_DIR"/statusline --version

# Windows (PowerShell) — extract into $CC_DIR, do not hardcode ~/.claude
Invoke-WebRequest -Uri "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_windows_amd64.zip" -OutFile "$env:TEMP\statusline.zip"
Expand-Archive -Path "$env:TEMP\statusline.zip" -DestinationPath "$CC_DIR\" -Force
Remove-Item "$env:TEMP\statusline.zip"

# macOS
curl -L "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_darwin_$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/').tar.gz" | tar -xz -C "$CC_DIR/"

# Linux
curl -L "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_linux_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xz -C "$CC_DIR/"
```

> **Multi-account note**: if you set `CLAUDE_CONFIG_DIR` (e.g. `~/.claude-account-ME`),
> the binary and `settings.json` must go in that directory, not `~/.claude/`.
> Installing into the wrong dir leaves Claude Code loading the stale binary —
> "I updated but nothing changed."

To enable automatic plugin updates on startup:

1. Run `/plugin`
2. Go to **Marketplaces** tab
3. Select `claude-token-monitor` marketplace
4. Enable **Auto-update**

Or via CLI:

```bash
claude plugin marketplace update claude-token-monitor --auto-update true
```

---

[中文文档](./README.md)
