package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelCollector_Collect(t *testing.T) {
	collector := NewModelCollector()

	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{
			name: "valid input with display name",
			input: &StatusLineInput{
				Model: struct {
					DisplayName string `json:"display_name"`
					ID          string `json:"id"`
				}{
					DisplayName: "GLM-4.7",
					ID:          "glm-4.7",
				},
			},
			want:    "GLM-4.7",
			wantErr: false,
		},
		{
			name: "empty display name defaults to Claude",
			input: &StatusLineInput{
				Model: struct {
					DisplayName string `json:"display_name"`
					ID          string `json:"id"`
				}{
					DisplayName: "",
					ID:          "some-id",
				},
			},
			want:    "Claude",
			wantErr: false,
		},
		{
			name:    "invalid input type",
			input:   "not a StatusLineInput",
			want:    "",
			wantErr: true,
		},
		{
			name:    "nil input",
			input:   nil,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(tt.input, nil)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid input type")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTokenBarCollector_Collect(t *testing.T) {
	collector := NewTokenBarCollector()

	tests := []struct {
		name      string
		input     *StatusLineInput
		wantErr   bool
		wantColor string // ANSI color code prefix to check
	}{
		{
			name:      "0% usage",
			input:     makeStatusInput(0, 0, 0, 200000),
			wantColor: "\x1b[1;32m", // green
		},
		{
			name:      "25% usage",
			input:     makeStatusInput(25000, 0, 25000, 200000),
			wantColor: "\x1b[1;36m", // cyan (>=20% and <40%)
		},
		{
			name:      "50% usage",
			input:     makeStatusInput(40000, 0, 60000, 200000),
			wantColor: "\x1b[1;33m", // yellow
		},
		{
			name:      "75% usage",
			input:     makeStatusInput(50000, 0, 100000, 200000),
			wantColor: "\x1b[1;31m", // red
		},
		{
			name:      "100% usage",
			input:     makeStatusInput(100000, 0, 100000, 200000),
			wantColor: "\x1b[1;31m", // red
		},
		{
			name:      "over 100% usage",
			input:     makeStatusInput(150000, 0, 100000, 200000),
			wantColor: "\x1b[1;31m", // red
		},
		{
			name:      "zero context window size defaults to 200K",
			input:     makeStatusInput(10000, 0, 0, 0),
			wantColor: "\x1b[1;32m", // green
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(tt.input, nil)

			// Assert
			require.NoError(t, err)
			require.NotEmpty(t, got)
			assert.True(t, strings.HasPrefix(got, "["), "should start with [")
			assert.True(t, strings.Contains(got, "\x1b[0m"), "should contain reset code")
			assert.True(t, strings.Contains(got, tt.wantColor), "should contain expected color code")
		})
	}
}

func TestTokenBarCollector_Collect_InvalidInput(t *testing.T) {
	collector := NewTokenBarCollector()

	// Act
	_, err := collector.Collect("invalid", nil)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input type")
}

func TestTokenInfoCollector_Collect(t *testing.T) {
	collector := NewTokenInfoCollector()

	tests := []struct {
		name       string
		input      *StatusLineInput
		wantErr    bool
		wantSubstr string
	}{
		{
			name:       "small token count",
			input:      makeStatusInput(500, 0, 0, 200000),
			wantSubstr: "/200K",
		},
		{
			name:       "thousands of tokens",
			input:      makeStatusInput(50000, 10000, 5000, 200000),
			wantSubstr: "65.0K/200K",
		},
		{
			name:       "millions of tokens",
			input:      makeStatusInput(500000, 500000, 500000, 2000000),
			wantSubstr: "1.5M/2000K",
		},
		{
			name:       "zero tokens",
			input:      makeStatusInput(0, 0, 0, 200000),
			wantSubstr: "0/200K",
		},
		{
			name:       "zero context window defaults to 200K",
			input:      makeStatusInput(10000, 0, 0, 0),
			wantSubstr: "/200K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(tt.input, nil)

			// Assert
			require.NoError(t, err)
			assert.Contains(t, got, tt.wantSubstr)
		})
	}
}

func TestTokenInfoCollector_Collect_InvalidInput(t *testing.T) {
	collector := NewTokenInfoCollector()

	// Act
	_, err := collector.Collect(123, nil)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input type")
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{name: "zero", n: 0, want: "0"},
		{name: "small number", n: 42, want: "42"},
		{name: "hundreds", n: 999, want: "999"},
		{name: "exactly 1 thousand", n: 1000, want: "1.0K"},
		{name: "thousands", n: 15000, want: "15.0K"},
		{name: "large thousands", n: 999999, want: "1000.0K"},
		{name: "exactly 1 million", n: 1000000, want: "1.0M"},
		{name: "millions", n: 2500000, want: "2.5M"},
		{name: "negative is not handled by function but test small", n: 1, want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatNumber(tt.n)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// makeStatusInput is a test helper that creates a StatusLineInput with specified token values.
func makeStatusInput(inputTokens, cacheTokens, outputTokens, contextWindowSize int) *StatusLineInput {
	input := &StatusLineInput{}
	input.ContextWindow.CurrentUsage.InputTokens = inputTokens
	input.ContextWindow.CurrentUsage.CacheReadInputTokens = cacheTokens
	input.ContextWindow.CurrentUsage.OutputTokens = outputTokens
	input.ContextWindow.ContextWindowSize = contextWindowSize
	return input
}
