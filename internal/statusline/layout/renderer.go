package layout

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// ansiRegex matches ANSI escape sequences (color codes, etc.)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// displayWidth returns the visible width of a string, ignoring ANSI escape sequences
func displayWidth(s string) int {
	return runewidth.StringWidth(ansiRegex.ReplaceAllString(s, ""))
}

// rowMeta holds a compacted row together with its alignment metadata
type rowMeta struct {
	cells   []string
	noAlign bool
}

// Renderer renders a grid to output lines
type Renderer struct {
	grid *Grid
}

// NewRenderer creates a new grid renderer
func NewRenderer(grid *Grid) *Renderer {
	return &Renderer{grid: grid}
}

// Render renders the grid to a slice of output lines
func (r *Renderer) Render() []string {
	// Step 1: Extract non-empty cells for each row (compact to left), preserving NoAlign flag
	allRows := r.compactRowsWithMeta()

	// Skip if no content
	if len(allRows) == 0 {
		return []string{}
	}

	// Step 2: Calculate column widths using only aligned rows
	var alignedCells [][]string
	for _, row := range allRows {
		if !row.noAlign {
			alignedCells = append(alignedCells, row.cells)
		}
	}
	colWidths := r.calculateColumnWidths(alignedCells)

	// Step 3: Render each row
	lines := []string{}
	for _, row := range allRows {
		if len(row.cells) == 0 {
			continue
		}
		var line string
		if row.noAlign {
			// No padding — just join the cells directly (no column alignment)
			line = strings.Join(row.cells, " ")
		} else {
			line = r.renderRowWithAlignment(row.cells, colWidths)
		}
		lines = append(lines, line)
	}

	return lines
}

// compactRowsWithMeta removes empty cells and shifts content to the left,
// preserving the NoAlign flag from each GridRow.
func (r *Renderer) compactRowsWithMeta() []rowMeta {
	result := []rowMeta{}

	for _, row := range r.grid.Rows {
		nonEmptyCells := []string{}
		for _, cell := range row.Cells {
			if cell != "" {
				// Split cells containing " | " into multiple cells
				// This handles composed content like "time-quota" (time | quota)
				if strings.Contains(cell, " | ") {
					parts := strings.Split(cell, " | ")
					nonEmptyCells = append(nonEmptyCells, parts...)
				} else {
					nonEmptyCells = append(nonEmptyCells, cell)
				}
			}
		}
		// Only add rows that have content
		if len(nonEmptyCells) > 0 {
			result = append(result, rowMeta{
				cells:   nonEmptyCells,
				noAlign: row.NoAlign,
			})
		}
	}

	return result
}

// compactRows returns a 2D slice for backward compatibility (used by tests)
func (r *Renderer) compactRows() [][]string {
	metas := r.compactRowsWithMeta()
	rows := make([][]string, len(metas))
	for i, m := range metas {
		rows[i] = m.cells
	}
	return rows
}

// calculateColumnWidths calculates the width of each column for alignment
func (r *Renderer) calculateColumnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return []int{}
	}

	// Find maximum number of columns
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	// Calculate width for each column
	colWidths := make([]int, maxCols)
	for col := 0; col < maxCols; col++ {
		maxWidth := 0
		for _, row := range rows {
			if col < len(row) {
				width := displayWidth(row[col])
				if width > maxWidth {
					maxWidth = width
				}
			}
		}
		colWidths[col] = maxWidth
	}

	return colWidths
}

// renderRowWithAlignment renders a single row with column alignment
func (r *Renderer) renderRowWithAlignment(row []string, colWidths []int) string {
	if len(row) == 0 {
		return ""
	}

	parts := []string{}
	for col, cell := range row {
		// Always add the cell content (even if empty)
		parts = append(parts, cell)

		// Only add padding and separator if this is not the last column
		if col < len(row)-1 {
			// Calculate padding needed for this column
			cellWidth := displayWidth(cell)
			targetWidth := colWidths[col]
			padding := targetWidth - cellWidth
			if padding < 0 {
				padding = 0
			}
			parts = append(parts, strings.Repeat(" ", padding))
			parts = append(parts, " | ")
		}
	}

	return strings.Join(parts, "")
}
