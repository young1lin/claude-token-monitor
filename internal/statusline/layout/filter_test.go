package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/young1lin/claude-token-monitor/internal/statusline/config"
)

// newConfig is a test helper that creates a config.Config with the given show/hide lists.
func newConfig(show, hide []string) *config.Config {
	return &config.Config{
		Display: config.DisplayConfig{
			Show: show,
			Hide: hide,
		},
	}
}

// defaultLayout returns the standard 10-cell layout used across filter tests.
func defaultLayout() *Layout {
	return &Layout{
		Cells: []Cell{
			{ContentType: "folder", Position: Position{Row: 0, Col: 0}},
			{ContentType: "token", Position: Position{Row: 0, Col: 1}},
			{ContentType: "claude-version", Position: Position{Row: 0, Col: 2}},
			{ContentType: "git", Position: Position{Row: 1, Col: 0}},
			{ContentType: "memory-files", Position: Position{Row: 1, Col: 1}},
			{ContentType: "skills", Position: Position{Row: 1, Col: 2}},
			{ContentType: "time-quota", Position: Position{Row: 2, Col: 0}},
			{ContentType: "agent", Position: Position{Row: 2, Col: 1}},
			{ContentType: "todo", Position: Position{Row: 2, Col: 2}},
			{ContentType: "tool-status-detail", Position: Position{Row: 3, Col: 0}},
		},
	}
}

// TestFilterLayout tests the FilterLayout function with various configurations.
func TestFilterLayout(t *testing.T) {
	layout := defaultLayout()

	tests := []struct {
		name       string
		show       []string
		hide       []string
		wantTypes  []string
		wantLength int
	}{
		{
			name:       "empty show and hide returns default layout unchanged",
			show:       nil,
			hide:       nil,
			wantTypes:  []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			wantLength: 10,
		},
		{
			name:       "empty slices returns default layout unchanged",
			show:       []string{},
			hide:       []string{},
			wantTypes:  []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			wantLength: 10,
		},
		{
			name:       "show list filters to only specified types",
			show:       []string{"folder", "git", "token"},
			hide:       nil,
			wantTypes:  []string{"folder", "token", "git"},
			wantLength: 3,
		},
		{
			name:       "show list with non-existent type returns only matching types",
			show:       []string{"folder", "nonexistent"},
			hide:       nil,
			wantTypes:  []string{"folder"},
			wantLength: 1,
		},
		{
			name:       "show list with all non-existent types returns empty layout",
			show:       []string{"nonexistent1", "nonexistent2"},
			hide:       nil,
			wantTypes:  []string{},
			wantLength: 0,
		},
		{
			name:       "hide list excludes specified types",
			show:       nil,
			hide:       []string{"skills", "todo", "tool-status-detail"},
			wantTypes:  []string{"folder", "token", "claude-version", "git", "memory-files", "time-quota", "agent"},
			wantLength: 7,
		},
		{
			name:       "hide list with non-existent type only removes matching types",
			show:       nil,
			hide:       []string{"nonexistent"},
			wantTypes:  []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			wantLength: 10,
		},
		{
			name:       "show and hide together - hide takes priority",
			show:       []string{"folder", "token", "git", "agent"},
			hide:       []string{"token"},
			wantTypes:  []string{"folder", "git", "agent"},
			wantLength: 3,
		},
		{
			name:       "hide overrides show when same cell is in both",
			show:       []string{"folder", "token", "git"},
			hide:       []string{"folder", "git"},
			wantTypes:  []string{"token"},
			wantLength: 1,
		},
		{
			name:       "hide everything via hide list",
			show:       nil,
			hide:       []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			wantTypes:  []string{},
			wantLength: 0,
		},
		{
			name:       "show everything and hide nothing returns all",
			show:       []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			hide:       nil,
			wantTypes:  []string{"folder", "token", "claude-version", "git", "memory-files", "skills", "time-quota", "agent", "todo", "tool-status-detail"},
			wantLength: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := newConfig(tt.show, tt.hide)

			// Act
			result := FilterLayout(layout, cfg)

			// Assert
			require.NotNil(t, result)
			assert.Equal(t, tt.wantLength, len(result.Cells), "cell count mismatch")

			// Verify content types match expected
			gotTypes := make([]string, len(result.Cells))
			for i, cell := range result.Cells {
				gotTypes[i] = cell.ContentType
			}
			assert.Equal(t, tt.wantTypes, gotTypes, "content types mismatch")

			// Verify that cell positions are preserved from original layout
			for _, cell := range result.Cells {
				expectedIndex := -1
				for j, originalCell := range layout.Cells {
					if originalCell.ContentType == cell.ContentType {
						expectedIndex = j
						break
					}
				}
				require.NotEqual(t, -1, expectedIndex, "cell %q not found in original layout", cell.ContentType)
				assert.Equal(t, layout.Cells[expectedIndex].Position, cell.Position, "position mismatch for cell %q", cell.ContentType)
			}
		})
	}
}

// TestGetFilteredContent tests the GetFilteredContent function with various configurations.
func TestGetFilteredContent(t *testing.T) {
	tests := []struct {
		name        string
		content     CellContent
		show        []string
		hide        []string
		wantContent CellContent
	}{
		{
			name:        "nil content returns empty CellContent",
			content:     nil,
			show:        nil,
			hide:        nil,
			wantContent: CellContent{},
		},
		{
			name:        "empty show/hide returns all content",
			content:     CellContent{"folder": "project", "token": "75K/200K", "git": "main"},
			show:        nil,
			hide:        nil,
			wantContent: CellContent{"folder": "project", "token": "75K/200K", "git": "main"},
		},
		{
			name:        "show list returns only matching keys",
			content:     CellContent{"folder": "project", "token": "75K/200K", "git": "main", "agent": "Explore"},
			show:        []string{"folder", "git"},
			hide:        nil,
			wantContent: CellContent{"folder": "project", "git": "main"},
		},
		{
			name:        "show list with no matching keys returns empty content",
			content:     CellContent{"folder": "project", "token": "75K/200K"},
			show:        []string{"nonexistent"},
			hide:        nil,
			wantContent: CellContent{},
		},
		{
			name:        "hide list excludes matching keys",
			content:     CellContent{"folder": "project", "token": "75K/200K", "git": "main", "agent": "Explore"},
			show:        nil,
			hide:        []string{"token", "agent"},
			wantContent: CellContent{"folder": "project", "git": "main"},
		},
		{
			name:        "hide list with no matching keys returns all content",
			content:     CellContent{"folder": "project", "token": "75K/200K"},
			show:        nil,
			hide:        []string{"nonexistent"},
			wantContent: CellContent{"folder": "project", "token": "75K/200K"},
		},
		{
			name:        "show and hide together - hide takes priority",
			content:     CellContent{"folder": "project", "token": "75K/200K", "git": "main", "agent": "Explore"},
			show:        []string{"folder", "token", "git", "agent"},
			hide:        []string{"token"},
			wantContent: CellContent{"folder": "project", "git": "main", "agent": "Explore"},
		},
		{
			name:        "hide overrides show when same key is in both",
			content:     CellContent{"folder": "project", "token": "75K/200K", "git": "main"},
			show:        []string{"folder", "token", "git"},
			hide:        []string{"folder", "git"},
			wantContent: CellContent{"token": "75K/200K"},
		},
		{
			name:        "empty content with show list returns empty result",
			content:     CellContent{},
			show:        []string{"folder", "token"},
			hide:        nil,
			wantContent: CellContent{},
		},
		{
			name:        "empty content with hide list returns empty result",
			content:     CellContent{},
			show:        nil,
			hide:        []string{"folder"},
			wantContent: CellContent{},
		},
		{
			name:        "content with empty string values is included when matching show list",
			content:     CellContent{"folder": "project", "token": ""},
			show:        []string{"folder", "token"},
			hide:        nil,
			wantContent: CellContent{"folder": "project", "token": ""},
		},
		{
			name:        "hide all keys returns empty content",
			content:     CellContent{"folder": "project", "token": "75K/200K"},
			show:        nil,
			hide:        []string{"folder", "token"},
			wantContent: CellContent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := newConfig(tt.show, tt.hide)

			// Act
			result := GetFilteredContent(tt.content, cfg)

			// Assert
			require.NotNil(t, result)
			assert.Equal(t, len(tt.wantContent), len(result), "content length mismatch")

			for key, expectedValue := range tt.wantContent {
				actualValue, ok := result[key]
				assert.True(t, ok, "expected key %q to be present", key)
				assert.Equal(t, expectedValue, actualValue, "value mismatch for key %q", key)
			}

			// Verify no extra keys in result
			for key := range result {
				_, ok := tt.wantContent[key]
				assert.True(t, ok, "unexpected key %q in result", key)
			}
		})
	}
}
