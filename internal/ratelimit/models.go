// Package ratelimit provides rate limit tracking and status for Claude API calls.
package ratelimit

import "time"

// RateLimitStatus represents the current rate limit status.
type RateLimitStatus struct {
	// Request-based limits
	RequestsRemaining int
	RequestsLimit     int
	RequestsReset     time.Time

	// Token-based limits
	TokensRemaining int
	TokensLimit     int
	TokensReset     time.Time

	// Overall status
	IsLimited bool
}

// RequestUsage returns the request usage percentage (0-100).
func (s *RateLimitStatus) RequestUsage() float64 {
	if s.RequestsLimit == 0 {
		return 0
	}
	used := s.RequestsLimit - s.RequestsRemaining
	return float64(used) / float64(s.RequestsLimit) * 100
}

// TokenUsage returns the token usage percentage (0-100).
func (s *RateLimitStatus) TokenUsage() float64 {
	if s.TokensLimit == 0 {
		return 0
	}
	used := s.TokensLimit - s.TokensRemaining
	return float64(used) / float64(s.TokensLimit) * 100
}

// GetStatusLevel returns the status level based on usage.
// Returns "ok", "warning", or "critical".
func (s *RateLimitStatus) GetStatusLevel() string {
	reqUsage := s.RequestUsage()
	tokenUsage := s.TokenUsage()

	// Use the higher of the two usages
	maxUsage := reqUsage
	if tokenUsage > maxUsage {
		maxUsage = tokenUsage
	}

	if maxUsage >= 80 {
		return "critical"
	} else if maxUsage >= 50 {
		return "warning"
	}
	return "ok"
}

// TimeUntilReset returns the duration until rate limit reset.
func (s *RateLimitStatus) TimeUntilReset() time.Duration {
	reset := s.RequestsReset
	if s.TokensReset.Before(s.RequestsReset) {
		reset = s.TokensReset
	}
	return time.Until(reset)
}

// TrackerConfig holds configuration for the rate limit tracker.
type TrackerConfig struct {
	// Default limits if not specified
	DefaultRequestsPerMinute int
	DefaultTokensPerMinute   int

	// Sliding window size
	WindowDuration time.Duration
}

// DefaultTrackerConfig returns the default tracker configuration.
func DefaultTrackerConfig() TrackerConfig {
	return TrackerConfig{
		DefaultRequestsPerMinute: 120, // Claude API default
		DefaultTokensPerMinute:   100000,
		WindowDuration:           time.Minute,
	}
}
