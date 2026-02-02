package layout

import "github.com/mattn/go-runewidth"

// DefaultLayout returns the default 4x4 grid layout
// Uses composed content types for compact display
// Grid structure:
//   Row 0: Folder | Token (composed: model+token-bar+token-info) | Version
//   Row 1: Git (composed: branch+status+remote) | Memory-files
//   Row 2: Tools | Agent | Todo + Session-duration
//   Row 3: Time-Quota (composed: time+quota) on last row
func DefaultLayout() *Layout {
	return &Layout{
		Cells: []Cell{
			// Row 0
			{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
			{ContentType: "token", Position: Position{Row: 0, Col: 1}, Optional: false},
			{ContentType: "claude-version", Position: Position{Row: 0, Col: 2}, Optional: true},

			// Row 1
			{ContentType: "git", Position: Position{Row: 1, Col: 0}, Optional: false},
			{ContentType: "memory-files", Position: Position{Row: 1, Col: 1}, Optional: true},
			{ContentType: "skills", Position: Position{Row: 1, Col: 2}, Optional: true},

			// Row 2
			{ContentType: "tools", Position: Position{Row: 2, Col: 0}, Optional: true},
			{ContentType: "agent", Position: Position{Row: 2, Col: 1}, Optional: true},
			{ContentType: "todo", Position: Position{Row: 2, Col: 2}, Optional: true},
			{ContentType: "session-duration", Position: Position{Row: 2, Col: 2}, Optional: true},

			// Row 3 - Time-Quota always on last row (includes time + quota)
			{ContentType: "time-quota", Position: Position{Row: 3, Col: 0}, Optional: false},
		},
	}
}

// NewGrid creates a new grid with the given content
func NewGrid(layout *Layout, content CellContent) *Grid {
	grid := &Grid{
		Layout:  layout,
		Content: content,
		Rows:    make([]GridRow, 4), // 4 rows
		ColWidths: make([]int, 4),   // 4 columns
	}

	// Initialize rows
	for i := range grid.Rows {
		grid.Rows[i].Cells = make([]string, 4)
	}

	// Populate grid with content
	grid.populate()

	// Calculate column widths
	grid.calculateWidths()

	return grid
}

// populate fills the grid with content based on the layout
func (g *Grid) populate() {
	for _, cell := range g.Layout.Cells {
		// Skip optional cells that have no content
		if cell.Optional {
			if content, ok := g.Content[cell.ContentType]; !ok || content == "" {
				continue
			}
		}

		// Get content for this cell
		content := g.Content[cell.ContentType]

		// Combine content for cells in the same position
		if g.Rows[cell.Position.Row].Cells[cell.Position.Col] != "" {
			// Content already exists, combine with separator
			if content != "" {
				g.Rows[cell.Position.Row].Cells[cell.Position.Col] += " | " + content
			}
		} else {
			g.Rows[cell.Position.Row].Cells[cell.Position.Col] = content
		}
	}
}

// calculateWidths calculates the width of each column
func (g *Grid) calculateWidths() {
	for col := 0; col < 4; col++ {
		maxWidth := 0
		for row := 0; row < 4; row++ {
			content := g.Rows[row].Cells[col]
			// Use runewidth to correctly handle emoji and wide characters
			width := runewidth.StringWidth(content)
			if width > maxWidth {
				maxWidth = width
			}
		}
		g.ColWidths[col] = maxWidth
	}
}

// GetRowCount returns the number of non-empty rows
func (g *Grid) GetRowCount() int {
	count := 0
	for _, row := range g.Rows {
		hasContent := false
		for _, cell := range row.Cells {
			if cell != "" {
				hasContent = true
				break
			}
		}
		if hasContent {
			count++
		}
	}
	return count
}
