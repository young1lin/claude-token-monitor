package layout

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

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
	// Step 1: Extract non-empty cells for each row (compact to left)
	compactRows := r.compactRows()

	// Skip if no content
	if len(compactRows) == 0 {
		return []string{}
	}

	// Step 2: Calculate column widths for alignment
	colWidths := r.calculateColumnWidths(compactRows)

	// Step 3: Render each row with alignment
	lines := []string{}
	for _, row := range compactRows {
		if len(row) == 0 {
			continue
		}
		line := r.renderRowWithAlignment(row, colWidths)
		lines = append(lines, line)
	}

	return lines
}

// compactRows removes empty cells and shifts content to the left
// Returns a 2D slice where each row contains only non-empty cells
func (r *Renderer) compactRows() [][]string {
	result := [][]string{}

	for _, row := range r.grid.Rows {
		nonEmptyCells := []string{}
		for _, cell := range row.Cells {
			if cell != "" {
				nonEmptyCells = append(nonEmptyCells, cell)
			}
		}
		// Only add rows that have content
		if len(nonEmptyCells) > 0 {
			result = append(result, nonEmptyCells)
		}
	}

	return result
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
				cell := row[col]
				// For cells containing " | ", only use the part before the separator
				// This handles composed content like "time-quota" (time | quota)
				if idx := strings.Index(cell, " | "); idx > 0 {
					cell = cell[:idx]
				}
				width := runewidth.StringWidth(cell)
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
		parts = append(parts, cell)
		// Only add padding and separator if this is not the last column in this row
		if col < len(row)-1 {
			// Calculate padding needed for this column
			cellWidth := runewidth.StringWidth(cell)
			targetWidth := colWidths[col]
			padding := targetWidth - cellWidth
			if padding < 0 {
				padding = 0
			}
			parts = append(parts, strings.Repeat(" ", padding))
			parts = append(parts, " | ")
		} else if len(row) == 1 && len(colWidths) > 1 {
			// Single cell row: add padding to align with multi-column rows
			cellWidth := runewidth.StringWidth(cell)
			targetWidth := colWidths[0]
			padding := targetWidth - cellWidth
			if padding < 0 {
				padding = 0
			}
			parts = append(parts, strings.Repeat(" ", padding))
		}
	}

	return strings.Join(parts, "")
}
