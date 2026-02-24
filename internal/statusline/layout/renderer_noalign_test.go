package layout

import (
	"strings"
	"testing"
)

// buildTestGrid creates a Grid with controlled row content for testing.
// It ensures all rows have a properly initialised 4-element Cells slice,
// mirroring what NewGrid does so calculateWidths doesn't panic.
func buildTestGrid(rows []GridRow) *Grid {
	// Ensure every row has a 4-element Cells slice (calculateWidths accesses by index)
	padded := make([]GridRow, 4)
	for i := 0; i < 4; i++ {
		if i < len(rows) {
			padded[i] = rows[i]
		}
		if len(padded[i].Cells) < 4 {
			cells := make([]string, 4)
			copy(cells, padded[i].Cells)
			padded[i].Cells = cells
		}
	}

	layout := &Layout{Cells: []Cell{}}
	grid := &Grid{
		Layout:    layout,
		Content:   CellContent{},
		Rows:      padded,
		ColWidths: make([]int, 4),
	}
	grid.calculateWidths()
	return grid
}

// TestNoAlignRowSkipsColumnWidthCalculation verifies that a NoAlign row does not
// influence column widths used by aligned rows.
func TestNoAlignRowSkipsColumnWidthCalculation(t *testing.T) {
	// Arrange: Row 0 = two aligned cells; Row 1 = long NoAlign cell
	rows := make([]GridRow, 4)
	rows[0] = GridRow{Cells: []string{"short", "ab", "", ""}}
	rows[1] = GridRow{
		Cells:   []string{"this is a very long unaligned tool status line", "", "", ""},
		NoAlign: true,
	}

	grid := buildTestGrid(rows)
	renderer := NewRenderer(grid)

	// Act
	lines := renderer.Render()

	// Assert: 2 lines rendered
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "short") || !strings.Contains(lines[0], "ab") {
		t.Errorf("aligned line should contain 'short' and 'ab', got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "this is a very long unaligned tool status line") {
		t.Errorf("noAlign line should contain raw content, got: %q", lines[1])
	}
	// NoAlign row has a single cell so no " | " separator
	if strings.Contains(lines[1], " | ") {
		t.Errorf("noAlign line should not have ' | ' separator, got: %q", lines[1])
	}
}

// TestNoAlignRowRenderedWithoutPadding verifies a NoAlign row with one cell
// is output as-is without extra padding.
func TestNoAlignRowRenderedWithoutPadding(t *testing.T) {
	// Arrange
	rows := make([]GridRow, 4)
	rows[0] = GridRow{
		Cells:   []string{"✓ Read(3) ✖ Edit(1)", "", "", ""},
		NoAlign: true,
	}

	grid := buildTestGrid(rows)
	renderer := NewRenderer(grid)

	// Act
	lines := renderer.Render()

	// Assert: single line, exact content
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "✓ Read(3) ✖ Edit(1)" {
		t.Errorf("expected %q, got %q", "✓ Read(3) ✖ Edit(1)", lines[0])
	}
}

// TestMixedAlignedAndNoAlignRows verifies correct rendering with both row types present.
func TestMixedAlignedAndNoAlignRows(t *testing.T) {
	// Arrange
	rows := make([]GridRow, 4)
	rows[0] = GridRow{Cells: []string{"📁 project", "token-info", "", ""}}
	rows[1] = GridRow{
		Cells:   []string{"✓ Read(5)", "", "", ""},
		NoAlign: true,
	}

	grid := buildTestGrid(rows)
	renderer := NewRenderer(grid)

	// Act
	lines := renderer.Render()

	// Assert
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], " | ") {
		t.Errorf("aligned row should use ' | ' separator, got: %q", lines[0])
	}
	if lines[1] != "✓ Read(5)" {
		t.Errorf("NoAlign row should be raw content, got: %q", lines[1])
	}
}

// TestAllAlignedRowsUseColumnWidths verifies column padding when all rows are aligned.
func TestAllAlignedRowsUseColumnWidths(t *testing.T) {
	// Arrange: "longer-cell" (11 chars) and "X" (1 char) in Col 0 of different rows
	rows := make([]GridRow, 4)
	rows[0] = GridRow{Cells: []string{"longer-cell", "B", "", ""}}
	rows[1] = GridRow{Cells: []string{"X", "Y", "", ""}}

	grid := buildTestGrid(rows)
	renderer := NewRenderer(grid)

	// Act
	lines := renderer.Render()

	// Assert: both rows rendered
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Row 1: "X" padded to 11 chars before " | "
	separatorIdx := strings.Index(lines[1], " | ")
	if separatorIdx != 11 {
		t.Errorf("X should be padded to width 11, separator at %d in: %q", separatorIdx, lines[1])
	}
}
