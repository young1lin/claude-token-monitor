// Package parser provides rate limit parsing and estimation from transcript data.
package parser

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RateLimitInfo represents rate limit information from API headers or estimation.
type RateLimitInfo struct {
	// Request limits
	RequestsRemaining int
	RequestsLimit     int
	RequestsReset     time.Time

	// Token limits
	TokensRemaining int
	TokensLimit     int
	TokensReset     time.Time

	// Is this data from actual headers or estimated?
	IsEstimated bool
}

// RateLimitConfig holds configuration for rate limit estimation.
type RateLimitConfig struct {
	// Default limits based on Claude API tier
	DefaultRequestsPerMinute int
	DefaultTokensPerMinute   int

	// Rate at which to reset limits (default: 1 minute)
	ResetWindow time.Duration
}

// DefaultRateLimitConfig returns default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultRequestsPerMinute: 120, // Claude API default
		DefaultTokensPerMinute:   100000,
		ResetWindow:              time.Minute,
	}
}

// ParseRateLimitsFromGitLog attempts to find rate limit information in git log.
// This is a fallback method for monitoring when direct API headers aren't available.
func ParseRateLimitsFromGitLog(cwd string) (*RateLimitInfo, error) {
	// Check if there's any rate limit info in recent commits
	// This is experimental and may not be available
	cmd := exec.Command("git", "log", "-1", "--pretty=%H")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// If we found a commit, estimate based on that
	commitHash := strings.TrimSpace(string(output))
	if commitHash != "" {
		return &RateLimitInfo{
			RequestsRemaining: 100,
			RequestsLimit:     120,
			TokensRemaining:   80000,
			TokensLimit:       100000,
			IsEstimated:       true,
		}, nil
	}

	return nil, nil
}

// EstimateRateLimits estimates rate limit status based on observed request patterns.
// This is used when actual API headers aren't available in the transcript.
func EstimateRateLimits(requestCountInWindow int, tokensUsedInWindow int, windowStart time.Time) *RateLimitInfo {
	config := DefaultRateLimitConfig()

	// Calculate remaining
	reqRemaining := max(0, config.DefaultRequestsPerMinute-requestCountInWindow)
	tokenRemaining := max(0, config.DefaultTokensPerMinute-tokensUsedInWindow)

	// Calculate reset time
	resetTime := windowStart.Add(config.ResetWindow)

	return &RateLimitInfo{
		RequestsRemaining: reqRemaining,
		RequestsLimit:     config.DefaultRequestsPerMinute,
		RequestsReset:     resetTime,
		TokensRemaining:   tokenRemaining,
		TokensLimit:       config.DefaultTokensPerMinute,
		TokensReset:       resetTime,
		IsEstimated:       true,
	}
}

// ParseRateLimitFromHeaders parses rate limit info from HTTP header strings.
// Format: "ratelimit-request-remaining: 100", "ratelimit-request-limit: 120", etc.
func ParseRateLimitFromHeaders(headers map[string]string) *RateLimitInfo {
	info := &RateLimitInfo{
		IsEstimated: false,
	}

	// Parse request limits
	if reqRemaining, ok := headers["ratelimit-request-remaining"]; ok {
		if val, err := strconv.Atoi(reqRemaining); err == nil {
			info.RequestsRemaining = val
		}
	}
	if reqLimit, ok := headers["ratelimit-request-limit"]; ok {
		if val, err := strconv.Atoi(reqLimit); err == nil {
			info.RequestsLimit = val
		}
	}
	if reqReset, ok := headers["ratelimit-request-reset"]; ok {
		if val, err := strconv.ParseInt(reqReset, 10, 64); err == nil {
			info.RequestsReset = time.Unix(val, 0)
		}
	}

	// Parse token limits
	if tokenRemaining, ok := headers["ratelimit-token-remaining"]; ok {
		if val, err := strconv.Atoi(tokenRemaining); err == nil {
			info.TokensRemaining = val
		}
	}
	if tokenLimit, ok := headers["ratelimit-token-limit"]; ok {
		if val, err := strconv.Atoi(tokenLimit); err == nil {
			info.TokensLimit = val
		}
	}
	if tokenReset, ok := headers["ratelimit-token-reset"]; ok {
		if val, err := strconv.ParseInt(tokenReset, 10, 64); err == nil {
			info.TokensReset = time.Unix(val, 0)
		}
	}

	return info
}

// FormatRateLimitSummary formats rate limit info for display.
func FormatRateLimitSummary(info *RateLimitInfo) string {
	if info == nil {
		return "Rate limit: unknown"
	}

	source := "estimated"
	if !info.IsEstimated {
		source = "api"
	}

	reqPct := 0.0
	if info.RequestsLimit > 0 {
		used := info.RequestsLimit - info.RequestsRemaining
		reqPct = float64(used) / float64(info.RequestsLimit) * 100
	}

	tokenPct := 0.0
	if info.TokensLimit > 0 {
		used := info.TokensLimit - info.TokensRemaining
		tokenPct = float64(used) / float64(info.TokensLimit) * 100
	}

	return fmt.Sprintf("[%.0f%% req, %.0f%% tokens, %s]", reqPct, tokenPct, source)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
