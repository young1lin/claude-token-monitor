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
			wantColor: "\x1b[1;92m", // bright green
		},
		{
			name:      "25% usage",
			input:     makeStatusInput(25000, 0, 25000, 200000),
			wantColor: "\x1b[1;32m", // green (>=20% and <40%)
		},
		{
			name:      "50% usage",
			input:     makeStatusInput(40000, 0, 60000, 200000),
			wantColor: "\x1b[1;36m", // cyan
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
			wantColor: "\x1b[1;92m", // bright green
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

// TestContextPercentColor pins the 5-tier mapping for the context-window
// scale. These thresholds intentionally differ from the quota scale (see
// quotaPercentColor): for context the percentage rising IS the warning, so
// red kicks in at 75% to give the user a few turns before AutoCompact
// triggers around 85%. Do NOT unify with quotaPercentColor — the semantics
// are inverted.
func TestContextPercentColor(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"0% is bright green", 0, "\x1b[1;92m"},
		{"19.9% is bright green", 19.9, "\x1b[1;92m"},
		{"20% enters green", 20, "\x1b[1;32m"},
		{"39.9% is green", 39.9, "\x1b[1;32m"},
		{"40% enters cyan", 40, "\x1b[1;36m"},
		{"59.9% is cyan", 59.9, "\x1b[1;36m"},
		{"60% enters yellow", 60, "\x1b[1;33m"},
		{"74.9% is yellow", 74.9, "\x1b[1;33m"},
		{"75% enters red", 75, "\x1b[1;31m"},
		{"100% stays red", 100, "\x1b[1;31m"},
		{"overcap stays red", 150, "\x1b[1;31m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contextPercentColor(tt.pct))
		})
	}
}

// TestContextAbsoluteColor pins the absolute-token tiers used for extended
// (>200K) context windows. The intent is to fire the compress-now warning
// near 200K used regardless of the window cap, because beyond ~200K the
// model degrades on speed and cost even when AutoCompact is far away.
func TestContextAbsoluteColor(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
		want   string
	}{
		{"0 tokens is green", 0, "\x1b[1;32m"},
		{"179,999 tokens is green", 179_999, "\x1b[1;32m"},
		{"180K enters cyan", 180_000, "\x1b[1;36m"},
		{"199,999 stays cyan", 199_999, "\x1b[1;36m"},
		{"200K enters yellow (compress soon)", 200_000, "\x1b[1;33m"},
		{"249,999 stays yellow", 249_999, "\x1b[1;33m"},
		{"250K enters red (compress NOW)", 250_000, "\x1b[1;31m"},
		{"500K stays red", 500_000, "\x1b[1;31m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contextAbsoluteColor(tt.tokens))
		})
	}
}

// TestContextColor verifies the dispatch rule: ≤200K windows use the legacy
// percentage tiers, while >200K windows switch to the absolute-token tiers
// so a 1M-cap user still sees a warning at ~200K used (where pct-based
// thresholds would leave them green right up to 600K).
func TestContextColor(t *testing.T) {
	tests := []struct {
		name      string
		tokens    int
		maxTokens int
		want      string
	}{
		// ≤200K cap: percentage path
		{"200K cap, 10K → bright green (5%)", 10_000, 200_000, "\x1b[1;92m"},
		{"200K cap, 60K → green (30%)", 60_000, 200_000, "\x1b[1;32m"},
		{"200K cap, 100K → cyan (50%)", 100_000, 200_000, "\x1b[1;36m"},
		{"200K cap, 130K → yellow (65%)", 130_000, 200_000, "\x1b[1;33m"},
		{"200K cap, 160K → red (80%)", 160_000, 200_000, "\x1b[1;31m"},
		// ContextWindowSize doesn't produce a divide-by-zero.
		{"zero cap defaults to 200K (bright green)", 10_000, 0, "\x1b[1;92m"},

		// >200K cap: absolute-token path. The key user-facing intent —
		// 200K used must NOT be green just because the cap is 1M.
		{"1M cap, 50K → green", 50_000, 1_000_000, "\x1b[1;32m"},
		{"1M cap, 180K → cyan (closing in)", 180_000, 1_000_000, "\x1b[1;36m"},
		{"1M cap, 200K → yellow (compress soon)", 200_000, 1_000_000, "\x1b[1;33m"},
		{"1M cap, 260K → red (compress NOW)", 260_000, 1_000_000, "\x1b[1;31m"},
		// 250K on a 1M window: under the OLD pct logic this was 25% =
		// cyan; under the new logic it's red — pinning the regression.
		{"1M cap, 250K → red (was cyan under pct logic)", 250_000, 1_000_000, "\x1b[1;31m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contextColor(tt.tokens, tt.maxTokens))
		})
	}
}

// TestTokenBarCollector_ExtendedWindow exercises the >200K dispatch end-to-end
// through the collector to make sure the bar's colour follows
// contextAbsoluteColor when the window is wider than the Anthropic default.
func TestTokenBarCollector_ExtendedWindow(t *testing.T) {
	collector := NewTokenBarCollector()
	tests := []struct {
		name      string
		input     *StatusLineInput
		wantColor string
	}{
		{
			name:      "1M cap, 50K used → green",
			input:     makeStatusInput(50_000, 0, 0, 1_000_000),
			wantColor: "\x1b[1;32m",
		},
		{
			name:      "1M cap, 180K used → cyan",
			input:     makeStatusInput(180_000, 0, 0, 1_000_000),
			wantColor: "\x1b[1;36m",
		},
		{
			name:      "1M cap, 200K used → yellow (compress soon)",
			input:     makeStatusInput(200_000, 0, 0, 1_000_000),
			wantColor: "\x1b[1;33m",
		},
		{
			name:      "1M cap, 260K used → red (compress NOW)",
			input:     makeStatusInput(260_000, 0, 0, 1_000_000),
			wantColor: "\x1b[1;31m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collector.Collect(tt.input, nil)
			require.NoError(t, err)
			assert.True(t, strings.Contains(got, tt.wantColor), "want %q to contain %q", got, tt.wantColor)
		})
	}
}

// TestTokenBarCollector_MinimumFillWhenUsed pins the rule that any non-zero
// usage must paint at least one filled block, so the tier colour is always
// visible. Regression for the 1M-window case where 9% of 1M truncates the
// fill to zero and leaves the bar rendered as bare "░░░░░░░░░░" with no
// colour at all — see the comment in TokenBarCollector.Collect.
func TestTokenBarCollector_MinimumFillWhenUsed(t *testing.T) {
	collector := NewTokenBarCollector()

	t.Run("1M window 9% usage still paints one green block", func(t *testing.T) {
		// 93.9K out of 1M = 9.39% → fillWidth would round to 0 without
		// the floor. We assert (a) the green colour code is present AND
		// (b) at least one filled "█" character appears.
		input := makeStatusInput(93_900, 0, 0, 1_000_000)
		got, err := collector.Collect(input, nil)
		require.NoError(t, err)
		assert.Contains(t, got, "\x1b[1;32m", "green tier must be applied")
		assert.Contains(t, got, "█", "must paint at least one filled block")
	})

	t.Run("0 tokens stays fully empty (no synthetic fill)", func(t *testing.T) {
		// 0 usage should still render an unfilled bar — the floor only
		// kicks in when tokens > 0.
		input := makeStatusInput(0, 0, 0, 1_000_000)
		got, err := collector.Collect(input, nil)
		require.NoError(t, err)
		assert.NotContains(t, got, "█", "zero usage must not paint a fill block")
	})

	t.Run("200K window 5% usage still paints one bright green block", func(t *testing.T) {
		// 10K out of 200K = 5% → also rounds to 0 fillWidth pre-floor;
		// the fix must apply to the legacy ≤200K path too, not just 1M.
		input := makeStatusInput(10_000, 0, 0, 200_000)
		got, err := collector.Collect(input, nil)
		require.NoError(t, err)
			assert.Contains(t, got, "\x1b[1;92m", "bright green tier must be applied")
		assert.Contains(t, got, "█", "must paint at least one filled block")
	})
}

// TestTokenInfoCollector_ExtendedWindow mirrors the bar test for the
// percent-text segment so the "(20.0%)" colouring escalates on the same
// schedule. Without this, a 1M user would see the bar go yellow at 200K but
// the parenthesised percent still display in the old (green) tier.
func TestTokenInfoCollector_ExtendedWindow(t *testing.T) {
	collector := NewTokenInfoCollector()
	tests := []struct {
		name      string
		input     *StatusLineInput
		wantColor string
	}{
		{"1M cap, 50K → green", makeStatusInput(50_000, 0, 0, 1_000_000), "\x1b[1;32m"},
		{"1M cap, 180K → cyan", makeStatusInput(180_000, 0, 0, 1_000_000), "\x1b[1;36m"},
		{"1M cap, 200K → yellow", makeStatusInput(200_000, 0, 0, 1_000_000), "\x1b[1;33m"},
		{"1M cap, 260K → red", makeStatusInput(260_000, 0, 0, 1_000_000), "\x1b[1;31m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collector.Collect(tt.input, nil)
			require.NoError(t, err)
			assert.Contains(t, got, tt.wantColor)
		})
	}
}

// TokenInfo must colour the percentage in the same tier as the bar, while
// leaving the absolute token counts plain so they remain easy to read.
func TestTokenInfoCollector_PercentColoured(t *testing.T) {
	tests := []struct {
		name      string
		input     *StatusLineInput
		wantColor string
		wantPct   string
	}{
		{
			name:      "5% lands in bright green tier",
			input:     makeStatusInput(10000, 0, 0, 200000),
			wantColor: "\x1b[1;92m",
			wantPct:   "5.0%",
		},
		{
			name:      "30% lands in green tier",
			input:     makeStatusInput(60000, 0, 0, 200000),
			wantColor: "\x1b[1;32m",
			wantPct:   "30.0%",
		},
		{
			name:      "50% lands in cyan tier",
			input:     makeStatusInput(100000, 0, 0, 200000),
			wantColor: "\x1b[1;36m",
			wantPct:   "50.0%",
		},
		{
			name:      "65% lands in yellow tier",
			input:     makeStatusInput(130000, 0, 0, 200000),
			wantColor: "\x1b[1;33m",
			wantPct:   "65.0%",
		},
		{
			name:      "80% lands in red tier",
			input:     makeStatusInput(160000, 0, 0, 200000),
			wantColor: "\x1b[1;31m",
			wantPct:   "80.0%",
		},
	}

	collector := NewTokenInfoCollector()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collector.Collect(tt.input, nil)
			require.NoError(t, err)
			// The percentage must be wrapped exactly: "(\x1b[...m12.9%\x1b[0m)".
			assert.Contains(t, got, "("+tt.wantColor+tt.wantPct+"\x1b[0m)")
			// The absolute count chunk must NOT carry ANSI codes — only the
			// percent does.
			countChunk := got[:strings.Index(got, " (")]
			assert.NotContains(t, countChunk, "\x1b[", "absolute count must stay plain")
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

func TestSessionTotalCollector_Collect(t *testing.T) {
	collector := NewSessionTotalCollector()

	tests := []struct {
		name      string
		totalIn   int
		totalOut  int
		costUSD   float64
		want      string
		wantEmpty bool
	}{
		{
			name:     "typical session",
			totalIn:  587879,
			totalOut: 60025,
			costUSD:  7.23,
			want:     "\U0001f4b0 $7.23 \u00b7 I:587.9K O:60.0K",
		},
		{
			name:     "million tokens",
			totalIn:  1200000,
			totalOut: 150000,
			costUSD:  15.50,
			want:     "\U0001f4b0 $15.50 \u00b7 I:1.2M O:150.0K",
		},
		{
			name:     "small session",
			totalIn:  500,
			totalOut: 100,
			costUSD:  0.01,
			want:     "\U0001f4b0 $0.01 \u00b7 I:500 O:100",
		},
		{
			name:     "zero cost with tokens",
			totalIn:  1000,
			totalOut: 200,
			costUSD:  0,
			want:     "\U0001f4b0 $0.00 \u00b7 I:1.0K O:200",
		},
		{
			name:      "all zero returns empty",
			totalIn:   0,
			totalOut:  0,
			costUSD:   0,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			input := &StatusLineInput{}
			input.ContextWindow.TotalInputTokens = tt.totalIn
			input.ContextWindow.TotalOutputTokens = tt.totalOut
			input.Cost.TotalCostUSD = tt.costUSD

			// Act
			got, err := collector.Collect(input, nil)

			// Assert
			require.NoError(t, err)
			if tt.wantEmpty {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSessionTotalCollector_InvalidInput(t *testing.T) {
	// Arrange
	collector := NewSessionTotalCollector()

	// Act
	_, err := collector.Collect("invalid", nil)

	// Assert
	assert.Error(t, err)
}

func TestSessionTotalCollector_Properties(t *testing.T) {
	// Arrange
	collector := NewSessionTotalCollector()

	// Assert
	assert.Equal(t, ContentSessionTotal, collector.Type())
	assert.True(t, collector.Optional())
}
