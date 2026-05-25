package content

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Table-driven test for getPlanName
func TestGetPlanName(t *testing.T) {
	tests := []struct {
		name             string
		subscriptionType string
		expected         string
	}{
		{"Max plan", "claude-max", "Max"},
		{"Max uppercase", "CLAUDE-MAX", "Max"},
		{"Pro plan", "claude-pro", "Pro"},
		{"Team plan", "claude-team", "Team"},
		{"API user empty", "", ""},
		{"API user api", "api", ""},
		{"API user Api", "Api", ""},
		{"Unknown type", "unknown", "Unknown"},
		{"Single char", "x", "X"},
		{"With spaces", "  pro  ", "Pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPlanName(tt.subscriptionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Table-driven test for getRateLimitedTTL
func TestGetRateLimitedTTL(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{"Zero count (same as 1)", 0, 55 * time.Second, 65 * time.Second},
		{"First retry", 1, 55 * time.Second, 65 * time.Second},
		{"Second retry", 2, 115 * time.Second, 125 * time.Second},
		{"Third retry", 3, 235 * time.Second, 245 * time.Second},
		{"Max cap", 10, 295 * time.Second, 305 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRateLimitedTTL(tt.count)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

// Table-driven test for parseRetryAfterHeader
// Note: parseRetryAfterHeader returns seconds as int, not time.Duration
func TestParseRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected int
	}{
		{"Empty header", "", 0},
		{"Seconds value", "30", 30},
		{"Seconds with spaces", " 60 ", 60},
		{"Invalid value", "invalid", 0},
		{"Negative value", "-10", 0},
		{"Zero value", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRetryAfterHeader(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test for parseRetryAfterHeader with HTTP date
func TestParseRetryAfterHeader_HTTPDate(t *testing.T) {
	// HTTP date format - we can't test exact values because it depends on current time
	// Just verify it doesn't panic and returns non-negative value
	result := parseRetryAfterHeader("Wed, 21 Oct 2015 07:28:00 GMT")
	assert.GreaterOrEqual(t, result, 0)
}

// Test for parseRetryAfterHeader with a future HTTP date
func TestParseRetryAfterHeader_FutureHTTPDate(t *testing.T) {
	future := time.Now().UTC().Add(60 * time.Second)
	result := parseRetryAfterHeader(future.Format(time.RFC1123))
	assert.Greater(t, result, 50)
	assert.Less(t, result, 70)
}

// Table-driven test for quotaPercentColor.
//
// Boundary cases assert each tier's lower edge AND a value safely inside the
// tier so a future off-by-one (e.g. ">=80" vs ">80") fails loudly.
// Semantics: for quota lines, "more used" === "less budget" → red kicks in
// at 80% and below that we go yellow / cyan / green / bright-green. This is
// the OPPOSITE direction from the context bar in model.go on purpose; the
// two lines mean different things to the user.
func TestQuotaPercentColor(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"negative is uncoloured (defensive)", -1, ""},
		{"zero is bright green (plenty of headroom)", 0, "\x1b[1;92m"},
		{"19.9 is bright green", 19.9, "\x1b[1;92m"},
		{"20 enters green tier (normal usage)", 20, "\x1b[1;32m"},
		{"39.9 is green", 39.9, "\x1b[1;32m"},
		{"40 enters cyan tier (past halfway)", 40, "\x1b[1;36m"},
		{"59.9 is cyan", 59.9, "\x1b[1;36m"},
		{"60 enters yellow tier (heads-up)", 60, "\x1b[1;33m"},
		{"79.9 is yellow", 79.9, "\x1b[1;33m"},
		{"80 enters red tier (out-of-budget warning)", 80, "\x1b[1;31m"},
		{"100 stays red", 100, "\x1b[1;31m"},
		{"125 stays red (overcap defensive)", 125, "\x1b[1;31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quotaPercentColor(tt.pct))
		})
	}
}

// colouredPercent must always emit a reset code so downstream text is not
// accidentally coloured. This is the invariant the renderer depends on.
func TestColouredPercentAlwaysClosesAnsi(t *testing.T) {
	for _, pct := range []float64{0, 15, 25, 45, 65, 85, 100} {
		got := colouredPercent(pct)
		assert.True(t, len(got) > 0, "non-empty output for %v", pct)
		assert.Contains(t, got, "\x1b[0m", "reset code missing for %v", pct)
	}
}

// formatPercentWindow with a coloured percentage MUST keep the label and
// countdown OUTSIDE the colour span so layout code that strips ANSI does
// not lose "5h" / "↻ 2h33m".
func TestFormatPercentWindow_ColoursOnlyTheNumber(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	reset := now.Add(2*time.Hour + 33*time.Minute)

	got := formatPercentWindow(28, "5h", reset, now)

	// Expect: "<green>28%<reset> 5h ↻ 2h33m"
	assert.Equal(t, "\x1b[1;32m28%\x1b[0m 5h ↻ 2h33m", got)

	// Above 80% → red instead, same surrounding text.
	red := formatPercentWindow(85, "5h", reset, now)
	assert.Equal(t, "\x1b[1;31m85%\x1b[0m 5h ↻ 2h33m", red)

	// No reset time → trailing "↻" must not appear, percentage still coloured.
	noReset := formatPercentWindow(15, "7d", time.Time{}, now)
	assert.Equal(t, "\x1b[1;92m15%\x1b[0m 7d", noReset)
}

// Test for getLocalTimeZoneName
func TestGetLocalTimeZoneName(t *testing.T) {
	// Save original TZ
	originalTZ := os.Getenv("TZ")
	defer os.Setenv("TZ", originalTZ)

	tests := []struct {
		name string
		tz   string
	}{
		{"UTC", "UTC"},
		{"America/New_York", "America/New_York"},
		{"Asia/Shanghai", "Asia/Shanghai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TZ", tt.tz)
			result := getLocalTimeZoneName()
			// Just verify it's not empty
			assert.NotEmpty(t, result)
		})
	}
}
