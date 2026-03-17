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

// Table-driven test for formatResetTime
func TestFormatResetTime(t *testing.T) {
	tests := []struct {
		name        string
		resetAt     time.Time
		shouldEmpty bool
	}{
		{"Zero time", time.Time{}, true},
		{"Valid time", time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResetTime(tt.resetAt)
			if tt.shouldEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				// Format is "HH:MM (timezone)"
				assert.Contains(t, result, ":")
			}
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

// Test for isUsingCustomApiEndpoint
func TestIsUsingCustomApiEndpoint(t *testing.T) {
	// Save original env vars
	originalBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	originalApiBaseURL := os.Getenv("ANTHROPIC_API_BASE_URL")
	defer func() {
		os.Setenv("ANTHROPIC_BASE_URL", originalBaseURL)
		os.Setenv("ANTHROPIC_API_BASE_URL", originalApiBaseURL)
	}()

	tests := []struct {
		name           string
		baseURL        string
		apiBaseURL     string
		expectedCustom bool
	}{
		{"No custom endpoint", "", "", false},
		{"Default Anthropic API", "https://api.anthropic.com", "", false},
		{"Custom endpoint", "https://custom.api.com", "", true},
		{"Custom endpoint with spaces", "  https://custom.api.com  ", "", true},
		{"API_BASE_URL custom", "", "https://custom.api.com", true},
		{"API_BASE_URL default", "", "https://api.anthropic.com/v1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ANTHROPIC_BASE_URL", tt.baseURL)
			os.Setenv("ANTHROPIC_API_BASE_URL", tt.apiBaseURL)
			result := isUsingCustomApiEndpoint()
			assert.Equal(t, tt.expectedCustom, result)
		})
	}
}
