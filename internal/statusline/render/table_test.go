package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
)

// TestNewTableRenderer verifies that NewTableRenderer creates a renderer
// with the provided grid.
func TestNewTableRenderer(t *testing.T) {
	// Arrange
	contentMap := layout.CellContent{
		"folder": "my-project",
		"token":  "[████░░] 50K/200K",
		"git":    "main +3 ~1",
		"agent":  "Explore",
		"todo":   "3/10",
	}
	grid := layout.NewGrid(layout.DefaultLayout(), contentMap)

	// Act
	tr := NewTableRenderer(grid)

	// Assert
	require.NotNil(t, tr, "NewTableRenderer should not return nil")
}

// TestRender verifies that Render produces the expected output lines
// from a grid with content.
func TestRender(t *testing.T) {
	t.Run("grid with content produces non-empty lines", func(t *testing.T) {
		// Arrange
		contentMap := layout.CellContent{
			"folder": "my-project",
			"token":  "[████░░] 50K/200K",
			"git":    "main +3 ~1",
			"agent":  "Explore",
			"todo":   "3/10",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		lines := tr.Render()

		// Assert
		require.NotEmpty(t, lines, "Render should produce at least one line")

		// Every line should be non-empty
		for i, line := range lines {
			assert.NotEmpty(t, line, "line %d should not be empty", i)
		}
	})

	t.Run("grid with minimal content", func(t *testing.T) {
		// Arrange - only provide required content (folder and token are non-optional in row 0)
		contentMap := layout.CellContent{
			"folder": "test-folder",
			"token":  "100K/200K",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		lines := tr.Render()

		// Assert
		require.NotEmpty(t, lines, "Render should produce at least one line")
		// First row (folder, token) should be present
		assert.Contains(t, lines[0], "test-folder",
			"first line should contain folder name")
		assert.Contains(t, lines[0], "100K/200K",
			"first line should contain token info")
	})

	t.Run("grid with all content rows", func(t *testing.T) {
		// Arrange
		contentMap := layout.CellContent{
			"folder":         "full-project",
			"token":          "[████████░░] 80K/200K",
			"claude-version": "v1.0",
			"git":            "main +5 ~2 -1",
			"memory-files":   "CLAUDE.md",
			"skills":         "commit, review",
			"time-quota":     "14:30 | 75K/5M",
			"agent":          "Code",
			"todo":           "7/20",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		lines := tr.Render()

		// Assert
		// Row 0: folder, token, claude-version
		require.NotEmpty(t, lines)
		assert.Contains(t, lines[0], "full-project")
		assert.Contains(t, lines[0], "80K/200K")
		assert.Contains(t, lines[0], "v1.0")
	})

	t.Run("grid with no content returns empty slice", func(t *testing.T) {
		// Arrange - all cells in DefaultLayout are optional except folder and token
		// But even with no content, the non-optional cells will just be empty strings
		// so rows may be skipped. Let's test with a completely empty content map.
		contentMap := layout.CellContent{}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		lines := tr.Render()

		// Assert - with no content, all rows are empty and should be skipped
		assert.Empty(t, lines, "empty grid should produce no lines")
	})

	t.Run("grid with tool status detail (NoAlign row)", func(t *testing.T) {
		// Arrange - Row 3 is a NoAlign row for tool-status-detail
		contentMap := layout.CellContent{
			"folder":             "align-test",
			"token":              "50K/100K",
			"tool-status-detail": "Read: 3 | Bash: 5 | Grep: 2",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		lines := tr.Render()

		// Assert
		require.NotEmpty(t, lines)
		// The last line should contain the tool status detail
		lastLine := lines[len(lines)-1]
		assert.Contains(t, lastLine, "Read: 3",
			"last line should contain tool status detail")
	})
}

// TestRenderSingleLine verifies that RenderSingleLine joins all rendered
// lines into a single string separated by " | ".
func TestRenderSingleLine(t *testing.T) {
	t.Run("joins multiple lines with separator", func(t *testing.T) {
		// Arrange
		contentMap := layout.CellContent{
			"folder": "single-line-test",
			"token":  "[████████░░] 80K/200K",
			"git":    "main +3 ~1",
			"agent":  "Explore",
			"todo":   "3/10",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		result := tr.RenderSingleLine()

		// Assert
		assert.NotEmpty(t, result, "RenderSingleLine should return non-empty string")

		// Should contain all the provided content
		assert.Contains(t, result, "single-line-test")
		assert.Contains(t, result, "80K/200K")
		assert.Contains(t, result, "main")
		assert.Contains(t, result, "Explore")
		assert.Contains(t, result, "3/10")

		// Should not contain newlines
		assert.NotContains(t, result, "\n", "single line output should not contain newlines")

		// Lines should be joined by " | "
		lines := tr.Render()
		if len(lines) > 1 {
			expectedJoin := strings.Join(lines, " | ")
			assert.Equal(t, expectedJoin, result,
				"single line should be the joined result of Render()")
		}
	})

	t.Run("single row produces output without extra separators", func(t *testing.T) {
		// Arrange - only row 0 content
		contentMap := layout.CellContent{
			"folder": "minimal",
			"token":  "10K/100K",
		}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		result := tr.RenderSingleLine()

		// Assert
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "minimal")
		assert.Contains(t, result, "10K/100K")
	})

	t.Run("empty grid returns empty string", func(t *testing.T) {
		// Arrange
		contentMap := layout.CellContent{}
		grid := layout.NewGrid(layout.DefaultLayout(), contentMap)
		tr := NewTableRenderer(grid)

		// Act
		result := tr.RenderSingleLine()

		// Assert
		assert.Empty(t, result, "empty grid should produce empty single line")
	})
}
