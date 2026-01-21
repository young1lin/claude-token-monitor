// Package layout provides grid-based layout configuration for the statusline
package layout

import "github.com/young1lin/claude-token-monitor/internal/statusline/config"

// FilterLayout filters the default layout based on the configuration.
// It respects both show and hide lists from the config.
func FilterLayout(defaultLayout *Layout, cfg *config.Config) *Layout {
	show := cfg.Display.Show
	hide := cfg.Display.Hide

	// If both are empty, return the default layout as-is
	if len(show) == 0 && len(hide) == 0 {
		return defaultLayout
	}

	// Build sets for quick lookup
	showSet := make(map[string]bool)
	for _, s := range show {
		showSet[s] = true
	}

	hideSet := make(map[string]bool)
	for _, h := range hide {
		hideSet[h] = true
	}

	// Filter cells
	var filteredCells []Cell
	for _, cell := range defaultLayout.Cells {
		contentType := cell.ContentType
		shouldShow := true

		// If show is non-empty, only show items in the list
		if len(show) > 0 {
			shouldShow = showSet[contentType]
		}

		// hide takes priority over show
		if hideSet[contentType] {
			shouldShow = false
		}

		if shouldShow {
			filteredCells = append(filteredCells, cell)
		}
	}

	return &Layout{Cells: filteredCells}
}

// GetFilteredContent returns a content map filtered by the configuration.
// This is useful when you want to filter content directly.
func GetFilteredContent(content CellContent, cfg *config.Config) CellContent {
	if content == nil {
		return make(CellContent)
	}

	show := cfg.Display.Show
	hide := cfg.Display.Hide

	// If both are empty, return all content
	if len(show) == 0 && len(hide) == 0 {
		return content
	}

	result := make(CellContent)

	for key, value := range content {
		shouldShow := true

		// If show is non-empty, only show items in the list
		if len(show) > 0 {
			showSet := make(map[string]bool)
			for _, s := range show {
				showSet[s] = true
			}
			shouldShow = showSet[key]
		}

		// hide takes priority over show
		for _, h := range hide {
			if h == key {
				shouldShow = false
				break
			}
		}

		if shouldShow {
			result[key] = value
		}
	}

	return result
}
