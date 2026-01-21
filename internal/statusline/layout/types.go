// Package layout provides grid-based layout configuration for the statusline
// Layout Layer: 4x4 grid configuration
package layout

// Position represents a position in the grid
type Position struct {
	Row int // 0-3
	Col int // 0-3
}

// Cell represents a cell in the grid
type Cell struct {
	ContentType  string
	Position     Position
	Span         int    // Number of columns to span (reserved for future use)
	Optional     bool   // Skip this cell if content is empty
	AlignWhenEmpty string // How to align when this cell is empty: "left", "center", "right"
}

// Layout represents the grid layout
type Layout struct {
	Cells []Cell
}

// CellContent represents the actual content for each cell
type CellContent map[string]string

// GridRow represents a row in the grid with its content
type GridRow struct {
	Cells []string // Content for each column in this row
}

// Grid represents the complete grid with content
type Grid struct {
	Layout   *Layout
	Content  CellContent
	Rows     []GridRow
	ColWidths []int
}
