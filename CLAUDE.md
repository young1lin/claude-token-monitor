# Claude Token Monitor - Project Documentation for Claude Code

## Project Overview

**Claude Token Monitor** is a pure statusline plugin for Claude Code that displays real-time session information directly in the IDE's status bar. It provides live token usage statistics, git status, tool calls, agent information, and more.

### Key Features

- **Token Usage**: Real-time token display with colored progress bar
- **Git Integration**: Branch name and file change statistics (+new ~modified -deleted)
- **Tool Tracking**: Displays active and completed tool calls
- **Agent Info**: Shows active agents and their descriptions
- **TODO Progress**: Tracks completion of TODO items from session
- **Session Duration**: Shows elapsed time for current session
- **Cross-platform**: Supports Windows, macOS, and Linux
- **High Performance**: Stateless execution with <10ms startup time

### Architecture

```
cmd/statusline/            # Statusline plugin entry point
└── main.go               # Entry point with JSON input/output

internal/
├── parser/               # Transcript parsing
│   ├── transcript.go     # JSONL transcript parser
│   └── transcript_test.go
├── statusline/
│   ├── config/           # Configuration management
│   ├── content/          # Content collectors and composers
│   ├── layout/           # Layout management
│   └── render/           # Output rendering
└── windows/              # Windows console initialization
```

### Claude Code Data Directory (IMPORTANT)

**All platforms** store Claude Code data in `$HOME/.claude/`:

```
~/.claude/
├── projects/           # Session data for all projects
│   ├── C--Users-...-project1/
│   │   ├── session-id-1.jsonl
│   │   └── session-id-2.jsonl
│   └── C--Users-...-project2/
│       └── session-id-3.jsonl
├── settings.json       # Global settings
├── CLAUDE.md           # Global instructions (if exists)
└── hooks/              # Global hooks
```

**Note**: The project directory names are URL-encoded paths (e.g., `C--Users-...-project`).

## Building

**IMPORTANT**: Binaries are built to the **current directory**, NOT to a `bin/` subdirectory.

```bash
# Build statusline plugin (outputs: statusline.exe on Windows, statusline on macOS/Linux)
go build -o statusline.exe ./cmd/statusline
```

### Platform-Specific Output

| Platform | Statusline Binary |
|----------|-------------------|
| Windows  | `statusline.exe`  |
| macOS    | `statusline`      |
| Linux    | `statusline`      |

### Quick Build Command

```bash
# Build for current platform
go build -o statusline$(go env GOEXE) ./cmd/statusline
```

Note: `go env GOEXE` returns `.exe` on Windows and empty on Unix systems.

## Running Tests

```bash
# Run all tests with coverage
go test ./... -coverprofile=coverage.out

# View detailed coverage report
go tool cover -html=coverage.out

# Run tests with race detection
go test ./... -race
```

## Configuration

### Global Configuration (Recommended)

**Location**: `~/.claude/settings.json`

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:\\\\Users\\\\YourName\\\\claude-token-monitor\\\\statusline.exe",
    "env": {
      "STATUSLINE_SINGLELINE": "1"
    }
  }
}
```

### Project-Level Configuration

**Location**: `.claude/settings.json` (project root)

Overrides global settings for specific projects.

### Environment Variables

| Variable | Values | Description |
|----------|--------|-------------|
| `STATUSLINE_SINGLELINE` | `1` | Enable single-line mode (default) |
| `STATUSLINE_DEBUG` | `1` | Enable debug output to stderr |
| `STATUSLINE_NO_COLOR` | `1` | Disable ANSI colors |
| `STATUSLINE_COMPACT` | `1` | Enable compact mode |

## StatusLine Output Format

### Single-Line Mode (default)

```
📁 minimal-mcp | [GLM-4.7] | [███░░░░░░░] 75K/200K | 🌿 main +143 ~1 | 🔧 8 tools | 🤖 Explore | 📋 3/10
```

### Component Breakdown

| Component | Description |
|-----------|-------------|
| `📁 minimal-mcp` | Project folder name |
| `[GLM-4.7]` | Model display name |
| `[███░░░░░░░]` | Colored progress bar |
| `75K/200K` | Token usage (current/total) |
| `🌿 main +143 ~1` | Git branch and file changes |
| `🔧 8 tools` | Tool call count |
| `🤖 Explore` | Active agent type |
| `📋 3/10` | TODO progress |

---

# StatusLine Plugin Development - Implementation Summary

## Overview

The statusline plugin displays real-time information in a single (or multi) line:
- Token usage with colored progress bar
- Git branch and file change statistics (+new ~modified -deleted)
- Tool call counts
- Agent information
- TODO progress
- Project folder name
- Session duration

## Location

- **Binary**: `cmd/statusline/main.go` → `statusline.exe`
- **Parser**: `internal/parser/transcript.go` (extracts tools, agents, TODO from transcript)

## Problems Encountered & Solutions

### Problem 1: Token Display Inaccuracy

**Symptom**: Status bar showed different token count than `/context` command.

**Root Cause**: Misunderstood Claude Code's JSON structure. Used `current_usage.input_tokens` (current message only) instead of calculating actual context size.

**Solution**:
```go
// WRONG: Only current message
tokens := input.ContextWindow.CurrentUsage.InputTokens

// CORRECT: Full context size
tokens := input.ContextWindow.CurrentUsage.InputTokens +
    input.ContextWindow.CurrentUsage.CacheReadInputTokens +
    input.ContextWindow.CurrentUsage.OutputTokens
```

**Key Learning**: Always inspect the actual JSON data first using debug output, don't assume field meanings.

### Problem 2: Git File Count Showing Wrong Numbers

**Symptom**: Only showed 7 new files, but there were 100+ untracked files.

**Root Cause**: `git status --porcelain` by default hides untracked files in subdirectories.

**Solution**:
```go
cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
```

### Problem 3: Branch Not Updating After Switch

**Symptom**: Switched branches in terminal, status bar still showed old branch.

**Root Cause**: Used cached `summary.GitBranch` from transcript parsing instead of real-time git info.

**Solution**: Always use `git` command to get current branch:
```go
// CORRECT: Always get real-time data
gitBranch := getGitBranch(input.Cwd)
```

### Problem 4: Progress Bar Colors Not Displaying

**Symptom**: Progress bar showed as plain text, no colors.

**Root Cause**: Used background color codes (`\x1b[48;5;Xm`) which may not be supported in all terminals.

**Solution**: Use foreground + bold colors which are more widely supported:
```go
// More compatible
colorCode = "\x1b[1;31m"  // Bold foreground
```

### Problem 5: Color Thresholds Not Matching AutoCompact

**Symptom**: Red color (80%+) never showed because AutoCompact triggers at 75%.

**Solution**: Adjusted thresholds for AutoCompact environment:
```go
// Adjusted for AutoCompact at 75%
if pct >= 60 { colorCode = "\x1b[1;31m" }  // Red
else if pct >= 40 { colorCode = "\x1b[1;33m" }  // Yellow
else if pct >= 20 { colorCode = "\x1b[1;36m" }  // Cyan
else { colorCode = "\x1b[1;32m" }  // Green
```

## Key Takeaways

### For Future Development

1. **Always inspect actual data first** - Use debug logging or write input to file to see the real JSON structure before making assumptions.

2. **Consider real-time vs cached data** - When displaying current state (git branch, file status), always fetch fresh data. Cached data (from transcript) is fine for historical info (tools used, agents run).

3. **Use widely-compatible ANSI codes** - Background colors may not work in all terminals. Foreground + bold (`\x1b[1;3Xm`) is safer:
   - 31 = Red, 32 = Green, 33 = Yellow, 36 = Cyan

4. **Adjust thresholds to actual usage** - If a system limits at 75% (AutoCompact), design UI scales around that limit, not theoretical maximums.

## Claude Code statusLine Limitations

**Important**: The statusLine is NOT refreshed continuously by Claude Code. It only updates when:
- A new message is sent
- Token usage changes significantly
- Commands like `/context` are used

External changes (like switching git branch in another terminal) won't show until a refresh trigger occurs in Claude Code. This is a Claude Code limitation, not a plugin issue.

---

# StatusLine Plugin - Hot Reload Mechanism

## How StatusLine Plugin Loading Works

### Execution Model: "Fire and Forget"

The statusline plugin works on a **stateless execution model**, not a persistent daemon process:

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│  Claude Code    │─────▶│ statusline.exe   │─────▶│  Exit (0)       │
│  (main process) │      │  (spawn & wait)  │      │  (cleanup)      │
└─────────────────┘      └──────────────────┘      └─────────────────┘
        │                         │
        │                         │
    ┌───▼──────┐          ┌──────▼──────┐
    │  stdin   │          │   stdout    │
    │  (JSON)  │          │  (output)   │
    └──────────┘          └─────────────┘
```

### Why Hot Reload Works Automatically

**Key Principle**: Each statusline refresh creates a **new process** that loads the latest `statusline.exe` from disk.

#### Execution Flow (Per Refresh)

1. **Claude Code needs to update statusline**
   - Trigger: New message, token change, `/context` command, etc.

2. **Spawn process**
   ```javascript
   // Claude Code internal (pseudocode)
   const process = spawn('statusline.exe', {
       env: { STATUSLINE_SINGLELINE: '1' }
   });
   ```

3. **Write input to stdin**
   ```json
   {
     "cwd": "C:\\Project",
     "model": { "display_name": "GLM-4.7" },
     "context_window": { "context_window_size": 200000, ... }
   }
   ```

4. **Read output from stdout**
   ```
   📁 project | [GLM-4.7] | [███░░░░░░░] 75K/200K
   ```

5. **Process exits** → Memory is freed

6. **Display output** in statusline area

### Why Recompiling Works Without Restart

| Scenario | What Happens |
|----------|--------------|
| **Initial state** | `statusline.exe` (v1.0) on disk |
| **Recompile** | `go build` overwrites `statusline.exe` with (v2.0) |
| **Next refresh** | Claude Code spawns `statusline.exe` → loads v2.0 from disk |
| **Result** | ✅ New version runs automatically |

**Critical Point**: The executable file is **read fresh from disk on each invocation**. There's no in-memory caching of the binary itself.

### Practical Implications

#### For Development
```bash
# Edit code
vim cmd/statusline/main.go

# Recompile (overwrites existing binary)
go build -o statusline.exe ./cmd/statusline

# Test immediately - no Claude Code restart needed!
# Next refresh automatically uses new version
```

#### For Users
```bash
# Update to latest version
cp new-statusline.exe ~/.claude/statusline.exe

# Works immediately - no restart
```

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Changes not appearing | Old binary still running | Check for zombie processes (`ps aux \| grep statusline`) |
| Stale output | Output cache not expired | Wait 5+ seconds or send new message |
| Wrong version | Multiple `statusline.exe` on PATH | Verify which binary is being used (`which statusline.exe`) |

## References

- [Go Testing Bible](https://go.dev/doc/build-cover)
- [Table-Driven Tests in Go](https://go.dev/wiki/TableDrivenTests)
- [Go Build Tags Documentation](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
