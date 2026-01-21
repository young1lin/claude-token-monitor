# Claude Token Monitor

A TUI tool for real-time Claude Code token usage monitoring.

## Features

- Real-time token usage display
- Context window percentage visualization
- Cost estimation
- Session history with persistence
- Cross-platform (Windows, macOS, Linux)

## Installation

```bash
go install github.com/young1lin/claude-token-monitor/cmd/monitor@latest
```

## Usage

```bash
claude-token-monitor
```

## Requirements

- Go 1.23+
- Claude Code installed

## How it works

The tool monitors Claude Code's JSONL session files located at:

**All platforms**: `$HOME/.claude/`

```
~/.claude/
├── projects/           # Session data for all projects
│   ├── C--Users-...-project1/
│   │   ├── session-id-1.jsonl
│   │   └── session-id-2.jsonl
│   └── C--Users-...-project2/
│       └── session-id-3.jsonl
├── settings.json       # Global settings
└── CLAUDE.md           # Global instructions (if exists)
```

It parses `type: "assistant"` messages to extract token usage data.

## StatusLine Plugin

The `statusline` command provides a customizable status bar for Claude Code, displaying real-time information in a grid layout.

### Configuration

Add to your `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:\\\\Path\\\\To\\\\statusline.exe"
  }
}
```

### Architecture

The statusline plugin implements a **three-layer architecture** for separation of concerns:

#### Layer 1: Content Layer (`internal/statusline/content/`)

Collects data from various sources with individual collectors and caching.

```
content/
├── types.go           # ContentType enum and ContentCollector interface
├── collector.go       # BaseCollector implementation
├── manager.go         # ContentManager with caching
├── folder.go          # Project folder name collector
├── git.go             # Git branch, status, remote collectors
├── model.go           # Model, token bar, token info collectors
├── memory.go          # CLAUDE.md, rules, MCPs collectors
├── session.go         # Agent, TODO, tools, duration collectors
├── time.go            # Current time, quota collectors
└── version.go         # Claude Code version collector
```

**Key Interfaces:**
- `ContentCollector` - Interface for data collection with caching support
- `ContentManager` - Manages collectors and caches results

#### Layer 2: Layout Layer (`internal/statusline/layout/`)

Defines a 4x4 grid layout and populates it with content.

```
layout/
├── types.go           # Position, Cell, Layout, Grid types
├── grid.go            # Default 4x4 layout definition
└── renderer.go        # Grid rendering logic
```

**Grid Layout:**
```
Row 0: [Folder] [Token]          [Version]
Row 1: [Git]    [Memory-files]   [Quota]
Row 2: [Tools]  [Agent]          [Todo+Duration]
Row 3: [Time-Quota] ...
```

**Note**: `Token` = model+token-bar+token-info, `Git` = branch+status+remote, `Time-Quota` = time+quota

#### Layer 3: Render Layer (`internal/statusline/render/`)

Handles final rendering with intelligent column alignment.

```
render/
├── table.go           # TableRenderer for grid-to-output
└── align.go           # Alignment utilities using runewidth
```

**Rendering Logic:**
1. **Compact**: Remove empty cells and shift content left
2. **Align**: Calculate column widths for proper alignment
3. **Render**: Output aligned rows with ` | ` separators

### Recent Changes

**v1.1.0 - Three-Layer Architecture Refactoring**

- Separated concerns into Content, Layout, and Render layers
- Each content type now has its own collector with independent caching
- Smart column alignment that skips empty columns
- Easier to extend with new content types

**Before:**
```go
// All logic mixed in main.go (1000+ lines)
func formatOutput(input, summary) []string {
    // Data collection
    // Layout calculation
    // Rendering
}
```

**After:**
```go
// Layer 1: Content Collection
manager := content.NewManager()
manager.RegisterAll(collectors...)
data := manager.GetAll(input, summary)

// Layer 2: Layout
grid := layout.NewGrid(layout.DefaultLayout(), data)

// Layer 3: Render
lines := render.NewTableRenderer(grid).Render()
```

### Adding New Content

To add a new content type:

1. Create a collector in `content/`:
```go
type MyCollector struct {
    *content.BaseCollector
}

func (c *MyCollector) Collect(input, summary) (string, error) {
    return "my data", nil
}
```

2. Register it in `main.go`:
```go
manager.Register(content.NewMyCollector())
```

3. Add to layout in `layout/grid.go`:
```go
{ContentType: "my-data", Position: Position{Row: 0, Col: 1}}
```

No changes needed to rendering logic!

### YAML Configuration

The statusline plugin supports YAML configuration files for customization without code changes.

#### Configuration File Locations

Configuration files are loaded with priority:
1. **Project-level**: `.claude/statusline.yaml` (highest priority)
2. **Global**: `~/.claude/statusline.yaml`
3. **Built-in defaults** (lowest priority)

#### Display Configuration

Control what content is displayed:

```yaml
# .claude/statusline.yaml
display:
  singleLine: false  # Enable single-line mode
  show:              # Only show these items (if specified)
    - folder
    - token
    - git-branch
  hide:              # Hide these items (takes priority over show)
    - claude-version
    - memory-files
```

#### Format Configuration

Control formatting options:

```yaml
format:
  progressBar: braille  # "braille" or "ascii"
  timeFormat: 24h       # "12h" or "24h"
  compact: false        # Enable compact mode
```

#### Content Composition

The statusline uses **composers** to combine related content types. Built-in composers:

- **`token`** - Combines model + token-bar + token-info → `[GLM-4.7 ███░░░░ 75K/200K]`
- **`git`** - Combines git-branch + git-status + git-remote → `🌿 main +3 ~2 🔄`
- **`time-quota`** - Combines current-time + quota → `14:30 | ⚡ 115/120 req`

#### Custom Composers

Define your own composers with custom formats:

```yaml
content:
  composers:
    - name: token-simple
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"  # Go template syntax

    - name: my-git
      input: [git-branch, git-status]
      format: "⎇ {{index . \"git-branch\"}}{{if (index . \"git-status\")}} {{index . \"git-status\"}}{{end}}"

  use:                    # Override default composers
    token: token-simple    # Use custom composer instead of default
    git: my-git
```

**Note**: Use `{{index . "key-name"}}` syntax for keys with hyphens in Go templates.

#### Example: Minimal Configuration

Hide unnecessary items, keep default composers:

```yaml
display:
  hide:
    - claude-version
    - memory-files
    - session-duration
```

**Output:**
```
📁 minimal-mcp
[GLM-4.7 ███░░░░ 75K/200K]
🌿 main +3 ~2
🔧 8 tools
14:30 | ⚡ 115/120 req
```

#### Example: Custom Token Display

Show only model and progress bar, skip token info:

```yaml
content:
  composers:
    - name: token-simple
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"
  use:
    token: token-simple
```

**Output:**
```
[GLM-4.7 ███░░░░]
```

#### Example: Single-Line Mode

All content on one line with `|` separators:

```yaml
display:
  singleLine: true
```

**Output:**
```
📁 minimal-mcp | [GLM-4.7 ███░░░░ 75K/200K] | 🌿 main +3 ~2 | 🔧 8 tools | 14:30 | ⚡ 115/120 req
```

### Composer Reference

#### Built-in Composers

| Name | Input Types | Output Format |
|------|-------------|---------------|
| `token` | model, token-bar, token-info | `[model bar info]` |
| `git` | git-branch, git-status, git-remote | `🌿 branch status remote` |
| `time-quota` | current-time, quota | `time \| quota` |

#### Composer Variants

Use built-in variants via `content.use`:

```yaml
content:
  use:
    token: token-simple     # Model + bar only (need to define)
    git: git-branch-only    # Branch only (need to define)
```

Or create your own:

```yaml
content:
  composers:
    - name: model-only
      input: [model]
      format: "🤖 {{.model}}"

    - name: git-clean
      input: [git-branch]
      format: "⎇ {{index . \"git-branch\"}}"

  use:
    token: model-only
    git: git-clean
```

### Full Example Configuration

```yaml
# .claude/statusline.yaml
display:
  singleLine: false
  show:
    - folder
    - token
    - git-branch
    - tools
  hide:
    - claude-version
    - memory-files

format:
  progressBar: braille
  timeFormat: 24h
  compact: false

content:
  composers:
    - name: my-token
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"

    - name: my-git
      input: [git-branch, git-status]
      format: "🌿 {{index . \"git-branch\"}}{{if (index . \"git-status\")}} {{index . \"git-status\"}}{{end}}"

  use:
    token: my-token
    git: my-git
```

**Output:**
```
📁 minimal-mcp
[GLM-4.7 ███░░░░]
🌿 main +3 ~2
🔧 8 tools
```
