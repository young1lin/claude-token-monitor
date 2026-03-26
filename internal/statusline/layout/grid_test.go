package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultLayout verifies DefaultLayout returns a layout with 10 cells in expected positions.
func TestDefaultLayout(t *testing.T) {
	// Act
	layout := DefaultLayout()

	// Assert
	require.NotNil(t, layout)
	assert.Equal(t, 11, len(layout.Cells), "default layout should have 11 cells")

	expectedCells := []struct {
		contentType string
		row, col    int
		optional    bool
		noAlign     bool
	}{
		{"folder", 0, 0, false, false},
		{"token", 0, 1, false, false},
		{"claude-version", 0, 2, true, false},
		{"git", 1, 0, false, false},
		{"memory-files", 1, 1, true, false},
		{"session-total", 1, 2, true, false},
		{"time-quota", 2, 0, false, false},
		{"agent", 2, 1, true, false},
		{"todo", 2, 2, true, false},
		{"parent-memory", 2, 3, true, false},
		{"tool-status-detail", 3, 0, true, true},
	}

	for i, expected := range expectedCells {
		cell := layout.Cells[i]
		assert.Equal(t, expected.contentType, cell.ContentType, "cell %d: ContentType mismatch", i)
		assert.Equal(t, expected.row, cell.Position.Row, "cell %d: Row mismatch", i)
		assert.Equal(t, expected.col, cell.Position.Col, "cell %d: Col mismatch", i)
		assert.Equal(t, expected.optional, cell.Optional, "cell %d: Optional mismatch", i)
		assert.Equal(t, expected.noAlign, cell.NoAlign, "cell %d: NoAlign mismatch", i)
	}
}

// TestNewGrid verifies grid creation, population, and width calculation.
func TestNewGrid(t *testing.T) {
	tests := []struct {
		name            string
		content         CellContent
		wantRowCount    int
		wantRow0Col0    string
		wantRow0Col1    string
		wantRow3NoAlign bool
	}{
		{
			name: "grid with all content populated",
			content: CellContent{
				"folder":             "my-project",
				"token":              "75K/200K",
				"claude-version":     "1.0.0",
				"git":                "main",
				"memory-files":       "3 files",
				"session-total":      "$7.23 · I:587K O:60K",
				"time-quota":         "5m | 50%",
				"agent":              "Explore",
				"todo":               "3/10",
				"tool-status-detail": "Read(5) Edit(2)",
			},
			wantRowCount:    4,
			wantRow0Col0:    "my-project",
			wantRow0Col1:    "75K/200K",
			wantRow3NoAlign: true,
		},
		{
			name: "grid with only required content",
			content: CellContent{
				"folder":     "my-project",
				"token":      "75K/200K",
				"git":        "main",
				"time-quota": "5m",
			},
			wantRowCount:    3,
			wantRow0Col0:    "my-project",
			wantRow0Col1:    "75K/200K",
			wantRow3NoAlign: false,
		},
		{
			name: "grid with empty content",
			content: CellContent{
				"folder": "my-project",
			},
			wantRowCount:    1,
			wantRow0Col0:    "my-project",
			wantRow0Col1:    "",
			wantRow3NoAlign: false,
		},
		{
			name:            "grid with no content",
			content:         CellContent{},
			wantRowCount:    0,
			wantRow0Col0:    "",
			wantRow0Col1:    "",
			wantRow3NoAlign: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			layout := DefaultLayout()

			// Act
			grid := NewGrid(layout, tt.content)

			// Assert: grid structure
			require.NotNil(t, grid)
			assert.Equal(t, layout, grid.Layout, "layout should be stored in grid")
			assert.Equal(t, tt.content, grid.Content, "content should be stored in grid")
			assert.Equal(t, 4, len(grid.Rows), "grid should always have 4 rows")
			assert.Equal(t, 4, len(grid.ColWidths), "grid should always have 4 column widths")

			// Assert: each row has 4 columns initialized
			for i, row := range grid.Rows {
				assert.Equal(t, 4, len(row.Cells), "row %d should have 4 columns", i)
			}

			// Assert: specific cell content
			assert.Equal(t, tt.wantRow0Col0, grid.Rows[0].Cells[0], "row 0 col 0 content mismatch")
			assert.Equal(t, tt.wantRow0Col1, grid.Rows[0].Cells[1], "row 0 col 1 content mismatch")

			// Assert: NoAlign flag propagation
			assert.Equal(t, tt.wantRow3NoAlign, grid.Rows[3].NoAlign, "row 3 NoAlign flag mismatch")

			// Assert: row count
			assert.Equal(t, tt.wantRowCount, grid.GetRowCount(), "row count mismatch")
		})
	}
}

// TestPopulate verifies cells are filled correctly and optional empty cells are skipped.
func TestPopulate(t *testing.T) {
	tests := []struct {
		name           string
		layout         *Layout
		content        CellContent
		wantRowContent map[int][]string // row index -> expected cell content
	}{
		{
			name: "required cells always populated even with empty content",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "token", Position: Position{Row: 0, Col: 1}, Optional: false},
				},
			},
			content:        CellContent{},
			wantRowContent: map[int][]string{0: {"", ""}},
		},
		{
			name: "optional cells skipped when content is missing",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "agent", Position: Position{Row: 0, Col: 1}, Optional: true},
					{ContentType: "todo", Position: Position{Row: 0, Col: 2}, Optional: true},
				},
			},
			content:        CellContent{"folder": "proj"},
			wantRowContent: map[int][]string{0: {"proj", "", ""}},
		},
		{
			name: "optional cells included when content is present",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "agent", Position: Position{Row: 0, Col: 1}, Optional: true},
				},
			},
			content:        CellContent{"folder": "proj", "agent": "Explore"},
			wantRowContent: map[int][]string{0: {"proj", "Explore", "", ""}},
		},
		{
			name: "optional cell skipped when content key exists but value is empty",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "folder", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "agent", Position: Position{Row: 0, Col: 1}, Optional: true},
				},
			},
			content:        CellContent{"folder": "proj", "agent": ""},
			wantRowContent: map[int][]string{0: {"proj", "", "", ""}},
		},
		{
			name: "NoAlign flag propagated from cell to row",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "tool-detail", Position: Position{Row: 2, Col: 0}, Optional: true, NoAlign: true},
				},
			},
			content:        CellContent{"tool-detail": "Read(5)"},
			wantRowContent: map[int][]string{2: {"Read(5)", "", "", ""}},
		},
		{
			name: "cells at same position combined with separator",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "a", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "b", Position: Position{Row: 0, Col: 0}, Optional: false},
				},
			},
			content:        CellContent{"a": "first", "b": "second"},
			wantRowContent: map[int][]string{0: {"first | second", "", "", ""}},
		},
		{
			name: "cells at same position - second empty does not append separator",
			layout: &Layout{
				Cells: []Cell{
					{ContentType: "a", Position: Position{Row: 0, Col: 0}, Optional: false},
					{ContentType: "b", Position: Position{Row: 0, Col: 0}, Optional: false},
				},
			},
			content:        CellContent{"a": "first", "b": ""},
			wantRowContent: map[int][]string{0: {"first", "", "", ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			grid := &Grid{
				Layout:    tt.layout,
				Content:   tt.content,
				Rows:      make([]GridRow, 4),
				ColWidths: make([]int, 4),
			}
			for i := range grid.Rows {
				grid.Rows[i].Cells = make([]string, 4)
			}

			// Act
			grid.populate()

			// Assert
			for rowIdx, expectedCells := range tt.wantRowContent {
				for colIdx, expected := range expectedCells {
					assert.Equal(t, expected, grid.Rows[rowIdx].Cells[colIdx],
						"row %d col %d: content mismatch", rowIdx, colIdx)
				}
			}

			// Check NoAlign flag propagation
			for _, cell := range tt.layout.Cells {
				if cell.NoAlign {
					assert.True(t, grid.Rows[cell.Position.Row].NoAlign,
						"NoAlign not propagated for cell %q at row %d", cell.ContentType, cell.Position.Row)
				}
			}

			// Rows without any content should remain empty
			for rowIdx := 0; rowIdx < 4; rowIdx++ {
				if _, ok := tt.wantRowContent[rowIdx]; !ok {
					for colIdx := 0; colIdx < 4; colIdx++ {
						assert.Equal(t, "", grid.Rows[rowIdx].Cells[colIdx],
							"row %d col %d should be empty", rowIdx, colIdx)
					}
				}
			}
		})
	}
}

// TestCalculateWidths verifies column widths are calculated correctly.
func TestCalculateWidths(t *testing.T) {
	tests := []struct {
		name          string
		rows          []GridRow
		wantColWidths []int
	}{
		{
			name: "empty grid has all zero widths",
			rows: []GridRow{
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{0, 0, 0, 0},
		},
		{
			name: "single row determines widths",
			rows: []GridRow{
				{Cells: []string{"abc", "de", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{3, 2, 0, 0},
		},
		{
			name: "maximum width across rows is used",
			rows: []GridRow{
				{Cells: []string{"short", "B", "", ""}},
				{Cells: []string{"much-longer", "Y", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{11, 1, 0, 0},
		},
		{
			name: "all four columns have content",
			rows: []GridRow{
				{Cells: []string{"aa", "bb", "cc", "dd"}},
				{Cells: []string{"aaa", "b", "c", "d"}},
				{Cells: []string{"a", "bbb", "ccc", "ddd"}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{3, 3, 3, 3},
		},
		{
			name: "wide characters counted correctly",
			rows: []GridRow{
				{Cells: []string{"ab", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{2, 0, 0, 0},
		},
		{
			name: "multiple rows contributing to max width",
			rows: []GridRow{
				{Cells: []string{"x", "12345", "", ""}},
				{Cells: []string{"yyy", "12", "", ""}},
				{Cells: []string{"zz", "123", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantColWidths: []int{3, 5, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// Ensure all rows have exactly 4 cells
			paddedRows := make([]GridRow, 4)
			for i := 0; i < 4; i++ {
				if i < len(tt.rows) {
					paddedRows[i] = tt.rows[i]
				}
				if len(paddedRows[i].Cells) < 4 {
					cells := make([]string, 4)
					copy(cells, paddedRows[i].Cells)
					paddedRows[i].Cells = cells
				}
			}

			grid := &Grid{
				Layout:    &Layout{Cells: []Cell{}},
				Content:   CellContent{},
				Rows:      paddedRows,
				ColWidths: make([]int, 4),
			}

			// Act
			grid.calculateWidths()

			// Assert
			assert.Equal(t, tt.wantColWidths, grid.ColWidths, "column widths mismatch")
		})
	}
}

// TestGetRowCount verifies row counting with various grid states.
func TestGetRowCount(t *testing.T) {
	tests := []struct {
		name         string
		rows         []GridRow
		wantRowCount int
	}{
		{
			name: "empty grid returns zero",
			rows: []GridRow{
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantRowCount: 0,
		},
		{
			name: "partially filled grid counts only non-empty rows",
			rows: []GridRow{
				{Cells: []string{"folder-content", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"git-content", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantRowCount: 2,
		},
		{
			name: "fully filled grid returns 4",
			rows: []GridRow{
				{Cells: []string{"a", "b", "", ""}},
				{Cells: []string{"c", "d", "", ""}},
				{Cells: []string{"e", "f", "", ""}},
				{Cells: []string{"g", "", "", ""}},
			},
			wantRowCount: 4,
		},
		{
			name: "single non-empty row",
			rows: []GridRow{
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"only-row", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantRowCount: 1,
		},
		{
			name: "row with content only in last column counts as non-empty",
			rows: []GridRow{
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "content", ""}},
				{Cells: []string{"", "", "", ""}},
				{Cells: []string{"", "", "", ""}},
			},
			wantRowCount: 1,
		},
		{
			name: "all rows have at least one cell filled",
			rows: []GridRow{
				{Cells: []string{"a", "", "", ""}},
				{Cells: []string{"", "b", "", ""}},
				{Cells: []string{"", "", "c", ""}},
				{Cells: []string{"", "", "", "d"}},
			},
			wantRowCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// Ensure all rows have exactly 4 cells
			paddedRows := make([]GridRow, 4)
			for i := 0; i < 4; i++ {
				if i < len(tt.rows) {
					paddedRows[i] = tt.rows[i]
				}
				if len(paddedRows[i].Cells) < 4 {
					cells := make([]string, 4)
					copy(cells, paddedRows[i].Cells)
					paddedRows[i].Cells = cells
				}
			}

			grid := &Grid{
				Layout:    &Layout{Cells: []Cell{}},
				Content:   CellContent{},
				Rows:      paddedRows,
				ColWidths: make([]int, 4),
			}

			// Act
			count := grid.GetRowCount()

			// Assert
			assert.Equal(t, tt.wantRowCount, count, "row count mismatch")
		})
	}
}
