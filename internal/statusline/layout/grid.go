package layout

import "github.com/mattn/go-runewidth"

// DefaultLayout returns the default 4x4 grid layout
// Grid structure:
//   Row 0: Folder | Model + Token | Version
//   Row 1: Git + Status | Memory | (empty)
//   Row 2: Tools | (empty) | Duration
//   Row 3: Time (always on last row)
func DefaultLayout() *Layout {
	return &Layout{
		Cells: []Cell{
			// Row 0
			{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
			{ContentType: "model", Position: Position{Row: 0, Col: 1}, Optional: false},
			{ContentType: "claude-version", Position: Position{Row: 0, Col: 2}, Optional: true},

			// Row 1
			{ContentType: "git-branch", Position: Position{Row: 1, Col: 0}, Optional: false},
			{ContentType: "git-status", Position: Position{Row: 1, Col: 0}, Optional: false},
			{ContentType: "git-remote", Position: Position{Row: 1, Col: 0}, Optional: true},
			{ContentType: "memory-files", Position: Position{Row: 1, Col: 1}, Optional: true},

			// Row 2
			{ContentType: "tools", Position: Position{Row: 2, Col: 0}, Optional: true},
			{ContentType: "agent", Position: Position{Row: 2, Col: 2}, Optional: true},
			{ContentType: "session-duration", Position: Position{Row: 2, Col: 2}, Optional: true},

			// Row 3 - Time always on last row
			{ContentType: "current-time", Position: Position{Row: 3, Col: 0}, Optional: false},
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
