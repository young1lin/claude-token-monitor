# Claude Token Monitor

Real-time token usage statusline for Claude Code.

## Installation

```bash
/plugin marketplace add young1lin/claude-token-monitor
/plugin install claude-token-monitor
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
