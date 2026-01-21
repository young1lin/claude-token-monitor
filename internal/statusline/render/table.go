// Package render provides rendering utilities for the statusline
// Render Layer: Unified alignment and output
package render

import (
	"strings"

	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
)

// TableRenderer renders a layout grid to output lines
type TableRenderer struct {
	grid *layout.Grid
}

// NewTableRenderer creates a new table renderer
func NewTableRenderer(grid *layout.Grid) *TableRenderer {
	return &TableRenderer{grid: grid}
}

// Render renders the grid to a slice of output lines
func (t *TableRenderer) Render() []string {
	renderer := layout.NewRenderer(t.grid)
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
