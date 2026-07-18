// Package render provides rendering utilities for the statusline
// Render Layer: Unified alignment and output
package render

import (
	"strings"

	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
)

// TableRenderer renders a layout grid to output lines. The narrow flag is
// forwarded to the underlying layout.Renderer so Block Element width is
// resolved consistently for every render path (multi-line and single-line).
type TableRenderer struct {
	grid   *layout.Grid
	narrow bool
}

// NewTableRenderer creates a new table renderer. narrow is typically sourced
// from env.Terminal.NarrowBlock at the call site.
func NewTableRenderer(grid *layout.Grid, narrow bool) *TableRenderer {
	return &TableRenderer{grid: grid, narrow: narrow}
}

// Render renders the grid to a slice of output lines
func (t *TableRenderer) Render() []string {
	renderer := layout.NewRenderer(t.grid, t.narrow)
	return renderer.Render()
}

// RenderSingleLine renders all content as a single line
func (t *TableRenderer) RenderSingleLine() string {
	lines := t.Render()
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " | ")
}
