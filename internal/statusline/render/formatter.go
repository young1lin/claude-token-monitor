// Package render provides rendering utilities for the statusline
// Render Layer: Unified alignment and output
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/statusline/config"
)

// Formatter provides custom formatting options for statusline output
type Formatter struct {
	progressBarStyle string // "ascii" or "braille"
	timeFormat       string // "12h" or "24h"
	compact          bool
}

// NewFormatter creates a new formatter with the given configuration
func NewFormatter(cfg config.FormatConfig) *Formatter {
	style := cfg.ProgressBar
	if style == "" {
		style = "braille"
	}

	tfmt := cfg.TimeFormat
	if tfmt == "" {
		tfmt = "24h"
	}

	return &Formatter{
		progressBarStyle: style,
		timeFormat:       tfmt,
		compact:          cfg.Compact,
	}
}

// RenderProgressBar renders a progress bar with the given percentage and width
func (f *Formatter) RenderProgressBar(pct float64, width int) string {
	if width <= 0 {
		width = 20 // default width
	}

	if f.progressBarStyle == "ascii" {
		return f.renderAsciiBar(pct, width)
	}
	return f.renderBrailleBar(pct, width)
}

// renderAsciiBar renders an ASCII-style progress bar: [#####.......]
func (f *Formatter) renderAsciiBar(pct float64, width int) string {
	if width < 2 {
		width = 2
	}

	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	return fmt.Sprintf("[%s%s]",
		strings.Repeat("#", filled),
		strings.Repeat(".", width-filled))
}

// renderBrailleBar renders a Braille-style progress bar: [█████░░░░░░░]
func (f *Formatter) renderBrailleBar(pct float64, width int) string {
	if width < 2 {
		width = 2
	}

	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	return fmt.Sprintf("[%s%s]",
		strings.Repeat("█", filled),
		strings.Repeat("░", width-filled))
}

// FormatTime formats a time value based on the configured format
func (f *Formatter) FormatTime(t time.Time) string {
	if f.timeFormat == "12h" {
		return t.Format("3:04 PM")
	}
	return t.Format("15:04") // 24h format
}

// FormatDuration formats a duration with optional compact mode
func (f *Formatter) FormatDuration(d time.Duration) string {
	if f.compact {
		return f.formatCompactDuration(d)
	}
	return f.formatFullDuration(d)
}

// formatFullDuration formats a duration with full labels
func (f *Formatter) formatFullDuration(d time.Duration) string {
	totalMinutes := int(d.Minutes())
	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatCompactDuration formats a duration in compact form
func (f *Formatter) formatCompactDuration(d time.Duration) string {
	totalMinutes := int(d.Minutes())
	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// GetSeparator returns the appropriate separator based on compact mode
func (f *Formatter) GetSeparator() string {
	if f.compact {
		return "|"
	}
	return " | "
}

// GetProgressBarStyle returns the current progress bar style
func (f *Formatter) GetProgressBarStyle() string {
	return f.progressBarStyle
}

// GetTimeFormat returns the current time format
func (f *Formatter) GetTimeFormat() string {
	return f.timeFormat
}

// IsCompact returns whether compact mode is enabled
func (f *Formatter) IsCompact() bool {
	return f.compact
}
