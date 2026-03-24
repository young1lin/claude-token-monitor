package render

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/young1lin/claude-token-monitor/internal/statusline/config"
)

// TestNewFormatter verifies that NewFormatter correctly initializes with
// various config values and falls back to defaults for empty fields.
func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name                 string
		cfg                  config.FormatConfig
		wantProgressBarStyle string
		wantTimeFormat       string
		wantCompact          bool
	}{
		{
			name: "braille style with 24h format",
			cfg: config.FormatConfig{
				ProgressBar: "braille",
				TimeFormat:  "24h",
				Compact:     false,
			},
			wantProgressBarStyle: "braille",
			wantTimeFormat:       "24h",
			wantCompact:          false,
		},
		{
			name: "ascii style with 12h format",
			cfg: config.FormatConfig{
				ProgressBar: "ascii",
				TimeFormat:  "12h",
				Compact:     false,
			},
			wantProgressBarStyle: "ascii",
			wantTimeFormat:       "12h",
			wantCompact:          false,
		},
		{
			name: "empty style falls back to braille",
			cfg: config.FormatConfig{
				ProgressBar: "",
				TimeFormat:  "24h",
			},
			wantProgressBarStyle: "braille",
			wantTimeFormat:       "24h",
		},
		{
			name: "empty time format falls back to 24h",
			cfg: config.FormatConfig{
				ProgressBar: "braille",
				TimeFormat:  "",
			},
			wantProgressBarStyle: "braille",
			wantTimeFormat:       "24h",
		},
		{
			name: "both empty fall back to defaults",
			cfg: config.FormatConfig{
				ProgressBar: "",
				TimeFormat:  "",
			},
			wantProgressBarStyle: "braille",
			wantTimeFormat:       "24h",
		},
		{
			name: "compact mode enabled",
			cfg: config.FormatConfig{
				ProgressBar: "ascii",
				TimeFormat:  "12h",
				Compact:     true,
			},
			wantProgressBarStyle: "ascii",
			wantTimeFormat:       "12h",
			wantCompact:          true,
		},
		{
			name: "invalid style falls back to braille",
			cfg: config.FormatConfig{
				ProgressBar: "invalid",
				TimeFormat:  "24h",
			},
			// Note: NewFormatter does not validate the style string,
			// it just stores what is given. An empty string triggers the default.
			wantProgressBarStyle: "invalid",
			wantTimeFormat:       "24h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			f := NewFormatter(tt.cfg)

			// Assert
			assert.Equal(t, tt.wantProgressBarStyle, f.GetProgressBarStyle())
			assert.Equal(t, tt.wantTimeFormat, f.GetTimeFormat())
			assert.Equal(t, tt.wantCompact, f.IsCompact())
		})
	}
}

// TestRenderProgressBar verifies progress bar rendering with different styles,
// percentages, and widths.
func TestRenderProgressBar(t *testing.T) {
	t.Run("ascii style", func(t *testing.T) {
		f := NewFormatter(config.FormatConfig{ProgressBar: "ascii"})

		tests := []struct {
			name  string
			pct   float64
			width int
			check func(t *testing.T, got string)
		}{
			{
				name:  "0 percent",
				pct:   0,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[..........]", got)
				},
			},
			{
				name:  "50 percent",
				pct:   50,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[#####.....]", got)
				},
			},
			{
				name:  "100 percent",
				pct:   100,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[##########]", got)
				},
			},
			{
				name:  "exceeds 100 percent clamped to full",
				pct:   150,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[##########]", got)
				},
			},
			{
				name:  "width 1 should be bumped to minimum 2",
				pct:   50,
				width: 1,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[#.]", got)
				},
			},
			{
				name:  "negative width uses default 20",
				pct:   50,
				width: -5,
				check: func(t *testing.T, got string) {
					// width -5 => default 20, 50% of 20 = 10 filled
					require.Len(t, got, 22, "should be [20 chars]")
					assert.True(t, strings.HasPrefix(got, "["))
					assert.True(t, strings.HasSuffix(got, "]"))
					assert.Equal(t, 10, strings.Count(got, "#"))
				},
			},
			{
				name:  "33 percent rounds down",
				pct:   33,
				width: 10,
				check: func(t *testing.T, got string) {
					// int(33/100 * 10) = int(3.3) = 3
					assert.Equal(t, "[###.......]", got)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Arrange & Act
				got := f.RenderProgressBar(tt.pct, tt.width)

				// Assert
				tt.check(t, got)
			})
		}
	})

	t.Run("braille style", func(t *testing.T) {
		f := NewFormatter(config.FormatConfig{ProgressBar: "braille"})

		tests := []struct {
			name  string
			pct   float64
			width int
			check func(t *testing.T, got string)
		}{
			{
				name:  "0 percent",
				pct:   0,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[░░░░░░░░░░]", got)
				},
			},
			{
				name:  "50 percent",
				pct:   50,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[█████░░░░░]", got)
				},
			},
			{
				name:  "100 percent",
				pct:   100,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[██████████]", got)
				},
			},
			{
				name:  "exceeds 100 percent clamped to full",
				pct:   200,
				width: 10,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[██████████]", got)
				},
			},
			{
				name:  "width 1 should be bumped to minimum 2",
				pct:   100,
				width: 1,
				check: func(t *testing.T, got string) {
					assert.Equal(t, "[██]", got)
				},
			},
			{
				name:  "zero width uses default 20",
				pct:   0,
				width: 0,
				check: func(t *testing.T, got string) {
					// width 0 => default 20, 0% of 20 = 0 filled
					assert.True(t, strings.HasPrefix(got, "["))
					assert.True(t, strings.HasSuffix(got, "]"))
					assert.Equal(t, 20, strings.Count(got, "░"))
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Arrange & Act
				got := f.RenderProgressBar(tt.pct, tt.width)

				// Assert
				tt.check(t, got)
			})
		}
	})
}

// TestFormatTime verifies that FormatTime formats a fixed time in both 12h
// and 24h formats.
func TestFormatTime(t *testing.T) {
	// Use a fixed time: 14:30:00 (2:30 PM)
	fixedTime := time.Date(2026, 3, 18, 14, 30, 0, 0, time.UTC)

	t.Run("24h format", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{TimeFormat: "24h"})

		// Act
		got := f.FormatTime(fixedTime)

		// Assert
		assert.Equal(t, "14:30", got, "24h format should produce HH:MM")
	})

	t.Run("12h format", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{TimeFormat: "12h"})

		// Act
		got := f.FormatTime(fixedTime)

		// Assert
		assert.Equal(t, "2:30 PM", got, "12h format should produce H:MM PM")
	})

	t.Run("midnight in 12h format", func(t *testing.T) {
		// Arrange
		midnight := time.Date(2026, 3, 18, 0, 5, 0, 0, time.UTC)
		f := NewFormatter(config.FormatConfig{TimeFormat: "12h"})

		// Act
		got := f.FormatTime(midnight)

		// Assert
		assert.Equal(t, "12:05 AM", got, "midnight should display as 12:05 AM")
	})

	t.Run("noon in 12h format", func(t *testing.T) {
		// Arrange
		noon := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
		f := NewFormatter(config.FormatConfig{TimeFormat: "12h"})

		// Act
		got := f.FormatTime(noon)

		// Assert
		assert.Equal(t, "12:00 PM", got, "noon should display as 12:00 PM")
	})
}

// TestFormatDuration verifies duration formatting in both full and compact modes.
func TestFormatDuration(t *testing.T) {
	t.Run("full mode", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: false})

		tests := []struct {
			name string
			dur  time.Duration
			want string
		}{
			{
				name: "30 seconds rounds to 0 minutes",
				dur:  30 * time.Second,
				want: "0m",
			},
			{
				name: "90 seconds rounds to 1 minute",
				dur:  90 * time.Second,
				want: "1m",
			},
			{
				name: "5 minutes",
				dur:  5 * time.Minute,
				want: "5m",
			},
			{
				name: "1 hour",
				dur:  1 * time.Hour,
				want: "1h0m",
			},
			{
				name: "2 hours 30 minutes",
				dur:  2*time.Hour + 30*time.Minute,
				want: "2h30m",
			},
			{
				name: "zero duration",
				dur:  0,
				want: "0m",
			},
			{
				name: "1 hour 59 minutes",
				dur:  1*time.Hour + 59*time.Minute,
				want: "1h59m",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Act
				got := f.FormatDuration(tt.dur)

				// Assert
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("compact mode", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: true})

		tests := []struct {
			name string
			dur  time.Duration
			want string
		}{
			{
				name: "30 seconds rounds to 0 minutes",
				dur:  30 * time.Second,
				want: "0m",
			},
			{
				name: "5 minutes",
				dur:  5 * time.Minute,
				want: "5m",
			},
			{
				name: "1 hour",
				dur:  1 * time.Hour,
				want: "1:00",
			},
			{
				name: "2 hours 30 minutes",
				dur:  2*time.Hour + 30*time.Minute,
				want: "2:30",
			},
			{
				name: "1 hour 5 minutes - zero padded",
				dur:  1*time.Hour + 5*time.Minute,
				want: "1:05",
			},
			{
				name: "zero duration",
				dur:  0,
				want: "0m",
			},
			{
				name: "10 hours 1 minute",
				dur:  10*time.Hour + 1*time.Minute,
				want: "10:01",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Act
				got := f.FormatDuration(tt.dur)

				// Assert
				assert.Equal(t, tt.want, got)
			})
		}
	})
}

// TestGetSeparator verifies that GetSeparator returns the correct separator
// based on compact mode.
func TestGetSeparator(t *testing.T) {
	t.Run("non-compact mode returns wide separator", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: false})

		// Act
		got := f.GetSeparator()

		// Assert
		assert.Equal(t, " | ", got)
	})

	t.Run("compact mode returns narrow separator", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: true})

		// Act
		got := f.GetSeparator()

		// Assert
		assert.Equal(t, "|", got)
	})

	t.Run("separator is always non-empty", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{})

		// Act
		got := f.GetSeparator()

		// Assert
		assert.NotEmpty(t, got)
	})
}

// TestGetProgressBarStyle verifies that the configured style is returned.
func TestGetProgressBarStyle(t *testing.T) {
	t.Run("returns braille when set", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{ProgressBar: "braille"})

		// Act & Assert
		assert.Equal(t, "braille", f.GetProgressBarStyle())
	})

	t.Run("returns ascii when set", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{ProgressBar: "ascii"})

		// Act & Assert
		assert.Equal(t, "ascii", f.GetProgressBarStyle())
	})

	t.Run("returns braille for empty (default)", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{ProgressBar: ""})

		// Act & Assert
		assert.Equal(t, "braille", f.GetProgressBarStyle())
	})
}

// TestGetTimeFormat verifies that the configured time format is returned.
func TestGetTimeFormat(t *testing.T) {
	t.Run("returns 24h when set", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{TimeFormat: "24h"})

		// Act & Assert
		assert.Equal(t, "24h", f.GetTimeFormat())
	})

	t.Run("returns 12h when set", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{TimeFormat: "12h"})

		// Act & Assert
		assert.Equal(t, "12h", f.GetTimeFormat())
	})

	t.Run("returns 24h for empty (default)", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{TimeFormat: ""})

		// Act & Assert
		assert.Equal(t, "24h", f.GetTimeFormat())
	})
}

// TestIsCompact verifies that IsCompact correctly reflects the compact setting.
func TestIsCompact(t *testing.T) {
	t.Run("returns true when compact is enabled", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: true})

		// Act & Assert
		assert.True(t, f.IsCompact())
	})

	t.Run("returns false when compact is disabled", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{Compact: false})

		// Act & Assert
		assert.False(t, f.IsCompact())
	})

	t.Run("returns false by default (zero value)", func(t *testing.T) {
		// Arrange
		f := NewFormatter(config.FormatConfig{})

		// Act & Assert
		assert.False(t, f.IsCompact())
	})
}
