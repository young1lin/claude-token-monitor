package layout

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// ansiRegex matches ANSI escape sequences (color codes, etc.)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// isBlockElement checks if a rune is a Block Elements character (U+2580-U+259F)
func isBlockElement(r rune) bool {
	return r >= '▀' && r <= '▟'
}

// rowMeta holds a compacted row together with its alignment metadata
type rowMeta struct {
	cells   []string
	noAlign bool
}

// Renderer renders a grid to output lines. The narrow flag controls whether
// Block Elements (█░▓▒) are treated as width 1 — needed for terminals
// (macOS Terminal.app, VSCode, WARP, Windows conhost) that render them narrow
// while go-runewidth reports █ as width 2. Threading it as an instance field
// (instead of reading a package global) keeps the renderer stateful and
// testable: each process constructs one Renderer with the value resolved once
// from env.Terminal.NarrowBlock, and every width query is self-consistent.
type Renderer struct {
	grid   *Grid
	narrow bool
}

// NewRenderer creates a new grid renderer. The narrow flag is consulted by
// displayWidth to decide the rendered width of Block Elements.
func NewRenderer(grid *Grid, narrow bool) *Renderer {
	return &Renderer{grid: grid, narrow: narrow}
}

// displayWidth returns the visible width of a string, ignoring ANSI escape
// sequences.
//   - When r.narrow is true, Block Elements (█░▓▒) are treated as width 1
//     (terminals like macOS Terminal.app render them narrow).
//   - A base character immediately followed by U+FE0F (Variation Selector-16,
//     emoji presentation) is promoted to width 2. go-runewidth undercounts
//     emoji-presentation symbols such as 🗂 (U+1F5C2) as width 1, while every
//     terminal renders them as a 2-cell color emoji — without this override the
//     column alignment drifts.
func (r *Renderer) displayWidth(s string) int {
	// Strip ANSI codes first
	s = ansiRegex.ReplaceAllString(s, "")

	runes := []rune(s)
	width := 0
	for i, c := range runes {
		// U+FE0F adds no width on its own; it is accounted for via the base rune.
		if c == '️' {
			continue
		}
		w := runewidth.RuneWidth(c)
		if isBlockElement(c) {
			// Block Elements (█░▓▒) are East Asian Ambiguous, so RuneWidth
			// flips between 1 and 2 across CJK vs C/POSIX locales — but the
			// terminal's actual rendering is a terminal property, not a
			// locale one. Pin width to the narrow flag so alignment is stable
			// regardless of locale/CI: 1 on narrow terminals (Apple Terminal,
			// VSCode, WARP, conhost), 2 on wide ones (iTerm2, Windows Terminal).
			if r.narrow {
				w = 1
			} else {
				w = 2
			}
		}
		// Emoji presentation selector follows → render as a wide 2-cell emoji.
		if i+1 < len(runes) && runes[i+1] == '️' && w < 2 {
			w = 2
		}
		width += w
	}
	return width
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
				width := r.displayWidth(row[col])
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
			cellWidth := r.displayWidth(cell)
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
