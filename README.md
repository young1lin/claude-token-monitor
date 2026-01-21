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
Row 0: [Folder] [Model+Token] [Version]
Row 1: [Git]    [Memory]     [empty]
Row 2: [Tools]  [empty]      [Duration]
Row 3: [Time]   [empty]      [empty]
```

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
