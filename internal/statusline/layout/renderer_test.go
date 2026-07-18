package layout

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockElement(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'█', true},  // U+2588 Full Block
		{'░', true},  // U+2591 Light Shade
		{'▓', true},  // U+2593 Dark Shade
		{'▒', true},  // U+2592 Medium Shade
		{'▀', true},  // U+2580 Upper Half Block
		{'▄', true},  // U+2584 Lower Half Block
		{'a', false}, // ASCII
		{'中', false}, // CJK
		{'📁', false}, // Emoji
	}

	for _, tt := range tests {
		got := isBlockElement(tt.r)
		if got != tt.want {
			t.Errorf("isBlockElement(%q): got %v, want %v", tt.r, got, tt.want)
		}
	}
}

// newWidthRenderer builds a minimal Renderer solely so tests can call the
// displayWidth method. The grid is unused by displayWidth; only narrow matters.
func newWidthRenderer(narrow bool) *Renderer {
	return NewRenderer(nil, narrow)
}

func TestDisplayWidth_BlockElements(t *testing.T) {
	progressBar := "[██░░░░░░░░]"

	t.Run("default mode (narrow=false)", func(t *testing.T) {
		r := newWidthRenderer(false)
		// Block Elements are width 2 when not narrow (regardless of locale/EAW).
		got := r.displayWidth(progressBar)
		// Note: exact value depends on go-runewidth, just verify it's calculated
		if got <= 0 {
			t.Errorf("displayWidth should be positive, got %d", got)
		}
	})

	t.Run("narrow mode (narrow=true)", func(t *testing.T) {
		r := newWidthRenderer(true)
		// All Block Elements = 1: 3*1 + 7*1 + 2 = 12
		got := r.displayWidth(progressBar)
		want := 12
		if got != want {
			t.Errorf("displayWidth(%q): got %d, want %d", progressBar, got, want)
		}
	})

	t.Run("mixed content with narrow mode", func(t *testing.T) {
		r := newWidthRenderer(true)
		// "Prefix [███░░░░░░░] Suffix" = 7 + 12 + 7 = 26
		s := "Prefix [███░░░░░░░] Suffix"
		got := r.displayWidth(s)
		want := 26
		if got != want {
			t.Errorf("displayWidth(%q): got %d, want %d", s, got, want)
		}
	})

	t.Run("two renderers with different narrow flags disagree", func(t *testing.T) {
		// Pin the behavioural contract: the narrow flag is per-instance, so a
		// narrow and a wide renderer looking at the same string return different
		// widths. This holds on every locale because Block-Element width is
		// decided solely by the narrow flag, never by EastAsianWidth.
		wide := newWidthRenderer(false).displayWidth(progressBar)
		narrow := newWidthRenderer(true).displayWidth(progressBar)
		assert.Greater(t, wide, narrow,
			"narrow renderer must report a smaller width than wide for Block Elements")
	})
}

func TestDisplayWidth_ANSIStrip(t *testing.T) {
	// ANSI codes should be stripped before calculating width
	r := newWidthRenderer(false)
	colored := "\x1b[31mRed Text\x1b[0m"
	got := r.displayWidth(colored)
	want := 8 // "Red Text"
	if got != want {
		t.Errorf("displayWidth with ANSI: got %d, want %d", got, want)
	}
}

func TestDisplayWidth_EmptyString(t *testing.T) {
	r := newWidthRenderer(true)
	got := r.displayWidth("")
	if got != 0 {
		t.Errorf("displayWidth(\"\"): got %d, want 0", got)
	}
}

func TestRenderer_NoAlignMixedRows(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"time", "model"}, NoAlign: false},
			{Cells: []string{"raw content here"}, NoAlign: true},
			{Cells: []string{"branch", "status"}, NoAlign: false},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 3 {
		t.Fatalf("Render() returned %d lines, want 3", len(lines))
	}

	// NoAlign row should be joined with space, no padding
	if lines[1] != "raw content here" {
		t.Errorf("NoAlign row = %q, want %q", lines[1], "raw content here")
	}
}

func TestRenderer_SkipEmptyRows(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"a"}, NoAlign: false},
			{Cells: []string{""}, NoAlign: false},
			{Cells: []string{"", ""}, NoAlign: false},
			{Cells: []string{"b"}, NoAlign: false},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 2 {
		t.Fatalf("Render() returned %d lines, want 2", len(lines))
	}
}

func TestRenderer_AllNoAlign(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"alpha", "beta"}, NoAlign: true},
			{Cells: []string{"gamma"}, NoAlign: true},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 2 {
		t.Fatalf("Render() returned %d lines, want 2", len(lines))
	}

	if lines[0] != "alpha beta" {
		t.Errorf("NoAlign row 0 = %q, want %q", lines[0], "alpha beta")
	}
	if lines[1] != "gamma" {
		t.Errorf("NoAlign row 1 = %q, want %q", lines[1], "gamma")
	}
}

func TestRenderer_EmptyGrid(t *testing.T) {
	grid := &Grid{Rows: []GridRow{}}
	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	// Render returns empty slice (nil) when grid has no content
	if len(lines) != 0 {
		t.Fatalf("Render() empty grid returned %d lines, want 0 (empty slice)", len(lines))
	}
}

func TestRenderRowWithAlignment_EmptyCells(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"short", "", "longer"}},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 1 {
		t.Fatalf("Render() returned %d lines, want 1", len(lines))
	}

	// Should contain all cells with proper padding
	line := lines[0]
	if !strings.Contains(line, "short") {
		t.Errorf("Row should contain 'short', got %q", line)
	}
	if !strings.Contains(line, "longer") {
		t.Errorf("Row should contain 'longer', got %q", line)
	}
	if !strings.Contains(line, " | ") {
		t.Errorf("Row should contain column separator, got %q", line)
	}
}

func TestRenderRowWithAlignment_SingleCell(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"only"}},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 1 {
		t.Fatalf("Render() returned %d lines, want 1", len(lines))
	}

	// Single cell should have no separator or padding
	if lines[0] != "only" {
		t.Errorf("Single cell row = %q, want %q", lines[0], "only")
	}
}

func TestCompactRowsWithMeta_NoAlignPreserved(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"a", "b"}, NoAlign: false},
			{Cells: []string{"x"}, NoAlign: true},
		},
	}

	renderer := NewRenderer(grid, false)
	metas := renderer.compactRowsWithMeta()

	if len(metas) != 2 {
		t.Fatalf("compactRowsWithMeta() returned %d rows, want 2", len(metas))
	}

	if metas[0].noAlign {
		t.Error("First row should not be NoAlign")
	}
	if !metas[1].noAlign {
		t.Error("Second row should be NoAlign")
	}
}

func TestRenderRowWithAlignment_CellWiderThanColWidth(t *testing.T) {
	// When a cell in one row is wider than the column width from another row,
	// padding should be 0 (not negative).
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"x", "very-long-content"}, NoAlign: false},
			{Cells: []string{"short", "b"}, NoAlign: false},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 2 {
		t.Fatalf("Render() returned %d lines, want 2", len(lines))
	}

	// First row has short first cell → should have padding
	assert.True(t, strings.Contains(lines[0], " | "), "row 0 should contain separator")
	// Second row has "very-long-content" but second col width is only 1
	// padding should be 0 (no negative padding)
	assert.True(t, strings.Contains(lines[1], " | "), "row 1 should contain separator")
}

func TestRenderRowWithAlignment_AllEmptyCells(t *testing.T) {
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"", ""}, NoAlign: false},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 0 {
		t.Errorf("all-empty row should be skipped, got %d lines", len(lines))
	}
}

func TestRenderer_RenderRowWithAlignment_EmptyRow(t *testing.T) {
	// Test renderRowWithAlignment with empty row slice
	r := NewRenderer(&Grid{Rows: []GridRow{{Cells: []string{"a"}}}}, false)
	got := r.renderRowWithAlignment([]string{}, []int{10})
	if got != "" {
		t.Errorf("renderRowWithAlignment(empty row) = %q, want %q", got, "")
	}
}

func TestRenderer_NegativePaddingClamp(t *testing.T) {
	// Two rows: first row has a very wide first cell, second row has short first cell.
	// Column width should be from row 1 (wider), so row 2 gets negative padding
	// which must be clamped to 0.
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"very-wide-first-cell", "col2"}, NoAlign: false},
			{Cells: []string{"x", "short"}, NoAlign: false},
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 2 {
		t.Fatalf("Render() returned %d lines, want 2", len(lines))
	}

	// Both rows should have separator — no crash from negative padding
	assert.True(t, strings.Contains(lines[0], " | "), "row 0 should have separator")
	assert.True(t, strings.Contains(lines[1], " | "), "row 1 should have separator")
}

func TestRenderer_SkipsPostCompactEmptyCellRow(t *testing.T) {
	// After compactRowsWithMeta, a row with only empty cells becomes zero-length cells.
	// The Render loop has a guard: if len(row.cells) == 0 { continue }
	// This is already handled by compactRowsWithMeta filtering, but test the guard.
	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{"content"}},         // normal row
			{Cells: []string{"", "", ""}},        // all empty → filtered
			{Cells: []string{"another-content"}}, // normal row
		},
	}

	renderer := NewRenderer(grid, false)
	lines := renderer.Render()

	if len(lines) != 2 {
		t.Fatalf("Render() returned %d lines, want 2", len(lines))
	}
	assert.Equal(t, "content", lines[0])
	assert.Equal(t, "another-content", lines[1])
}

func TestCompactRows(t *testing.T) {
	tests := []struct {
		name string
		grid *Grid
		want [][]string
	}{
		{
			name: "removes empty cells and shifts left",
			grid: &Grid{
				Rows: []GridRow{
					{Cells: []string{"", "main", "", "+3 ~1"}},
				},
			},
			want: [][]string{{"main", "+3 ~1"}},
		},
		{
			name: "skips all-empty rows",
			grid: &Grid{
				Rows: []GridRow{
					{Cells: []string{"", ""}},
					{Cells: []string{"a", "b"}},
					{Cells: []string{"", ""}},
				},
			},
			want: [][]string{{"a", "b"}},
		},
		{
			name: "empty grid returns empty",
			grid: &Grid{
				Rows: []GridRow{},
			},
			want: [][]string(nil),
		},
		{
			name: "splits composed cells on pipe separator",
			grid: &Grid{
				Rows: []GridRow{
					{Cells: []string{"time | quota", "model"}},
				},
			},
			want: [][]string{{"time", "quota", "model"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer(tt.grid, false)
			got := r.compactRows()
			if len(got) != len(tt.want) {
				t.Fatalf("compactRows() length = %d, want %d", len(got), len(tt.want))
			}
			for i, row := range got {
				if len(row) != len(tt.want[i]) {
					t.Errorf("row[%d] length = %d, want %d", i, len(row), len(tt.want[i]))
					continue
				}
				for j, cell := range row {
					if cell != tt.want[i][j] {
						t.Errorf("row[%d][%d] = %q, want %q", i, j, cell, tt.want[i][j])
					}
				}
			}
		})
	}
}

// TestRenderer_NarrowFlagThreadsToDisplayWidth pins the end-to-end behavioural
// contract that the narrow flag passed to NewRenderer is what displayWidth and
// the alignment pipeline actually consult. With a Block-Element bar defining
// column 0's width, a short ASCII cell in another row must be padded to a
// different position under narrow vs. wide — proving the flag reaches the
// width calculation (not just the method signature).
func TestRenderer_NarrowFlagThreadsToDisplayWidth(t *testing.T) {
	// Pure-█ bar: width is decided solely by the narrow flag (1 vs 2), never
	// by EastAsianWidth/locale, so this test is stable on every CI runner.
	bar := "████" // 4×█. narrow: 4, wide: 8

	grid := &Grid{
		Rows: []GridRow{
			{Cells: []string{bar, "tail"}, NoAlign: false},
			{Cells: []string{"x", "y"}, NoAlign: false}, // ASCII → padding is visible
		},
	}

	narrowLines := NewRenderer(grid, true).Render()
	wideLines := NewRenderer(grid, false).Render()

	// Row 1 ("x" padded to col-0 width then " | y") is where the narrow flag
	// shows up: colWidths[0] = displayWidth(bar) = 4 (narrow) or 8 (wide).
	sepNarrow := strings.Index(narrowLines[1], " | ")
	sepWide := strings.Index(wideLines[1], " | ")

	require.GreaterOrEqual(t, sepNarrow, 0)
	require.GreaterOrEqual(t, sepWide, 0)
	// Under wide, each of the 4 █ counts as 2 instead of 1, so the "x" cell is
	// padded 4 spaces longer and the separator lands 4 bytes further right.
	assert.Equal(t, 4, sepWide-sepNarrow,
		"narrow flag must shift the separator by the Block-Element width delta")
}
