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
	ContentType    string
	Position       Position
	Span           int    // Number of columns to span (reserved for future use)
	Optional       bool   // Skip this cell if content is empty
	AlignWhenEmpty string // How to align when this cell is empty: "left", "center", "right"
	NoAlign        bool   // Skip column alignment for this cell's row
}

// Layout represents the grid layout
type Layout struct {
	Cells []Cell
}

// CellContent represents the actual content for each cell
type CellContent map[string]string

// GridRow represents a row in the grid with its content
type GridRow struct {
	Cells   []string // Content for each column in this row
	NoAlign bool     // When true, this row is not included in column width calculation
}

// Grid represents the complete grid with content.
//
// Column widths are intentionally NOT stored here: the renderer computes them
// itself over only the aligned rows, using a display-width function that
// strips ANSI codes and honours narrow-block terminals. A second width field
// on the Grid would be a trap — computed with different rules and never used
// for the actual output.
type Grid struct {
	Layout  *Layout
	Content CellContent
	Rows    []GridRow
}
