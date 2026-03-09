# Claude Token Monitor

Real-time token usage statusline for Claude Code.

![](./images/claude-code-monitor.png)

## Installation

```bash
/plugin marketplace add young1lin/claude-token-monitor
/plugin install claude-token-monitor@claude-token-monitor
/claude-token-monitor:setup
```

## Configuration

Create `.claude/statusline.yaml` in your project:

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

content:
  composers:
    - name: my-token
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"
  use:
    token: my-token
```

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

The plugin writes one or more lines of plain text (with optional ANSI color codes) to stdout:

```
[Claude Sonnet 4.5] | [███░░░░░░░] 75K/200K (37.5%) | 🌿 main +12 ~3 | 🔧 5 tools
```

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
2. **Fast** — Startup time under 10ms. No network calls. Reads only the tail of transcript files.
3. **Safe** — A crash in the plugin does not affect Claude Code. It simply shows no status text.
4. **Cross-platform** — Single Go binary with no external dependencies.

### Debugging with `--debug`

To inspect the exact JSON that Claude Code sends to the plugin, run with the `--debug` flag:

```bash
# In your Claude Code settings, temporarily add --debug:
"command": "C:\\\\path\\\\to\\\\statusline.exe --debug"
```

When `--debug` is enabled, the plugin writes the raw JSON input to a file called `statusline.debug` in the same directory as the binary:

```
+-------------------+       +--------------------+       +-------------------+
|                   | stdin  |                    | file  |                   |
|    Claude Code    +------->|  statusline.exe    +------>| statusline.debug  |
|                   | (JSON) |  --debug           |       | (pretty JSON)     |
+-------------------+       +--------+-----------+       +-------------------+
                                      |
                                      | stdout (normal output continues)
                                      v
                             +--------------------+
                             | [Model] [===---]   |
                             |  75K/200K (37.5%)  |
                             +--------------------+
```

The debug file contains a timestamped, pretty-printed copy of the input:

```
------------------------------------------------------------
Timestamp: 2026-02-02 17:55:00
File: C:\path\to\statusline.debug
------------------------------------------------------------

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
  "transcript_path": "...",
  "workspace": { ... }
}
------------------------------------------------------------
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

### Enable Auto-Update

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
