// Package ratelimit provides rate limit tracking and status for Claude API calls.
package ratelimit

import (
	"sync"
	"time"
)

// Tracker tracks API request and token usage using a sliding window algorithm.
type Tracker struct {
	mu sync.RWMutex

	// Configuration
	config TrackerConfig

	// Request tracking
	requests []time.Time

	// Token tracking
	tokens []tokenEntry

	// Current limits (updated from API responses)
	requestsLimit int
	tokensLimit   int

	// Reset times
	requestsReset time.Time
	tokensReset   time.Time
}

type tokenEntry struct {
	timestamp time.Time
	count     int
}

// NewTracker creates a new rate limit tracker.
func NewTracker(config TrackerConfig) *Tracker {
	if config.WindowDuration == 0 {
		config = DefaultTrackerConfig()
	}

	return &Tracker{
		config:         config,
		requests:       make([]time.Time, 0),
		tokens:         make([]tokenEntry, 0),
		requestsLimit:  config.DefaultRequestsPerMinute,
		tokensLimit:    config.DefaultTokensPerMinute,
		requestsReset:  time.Now().Add(config.WindowDuration),
		tokensReset:    time.Now().Add(config.WindowDuration),
	}
}

// RecordRequest records a new API request.
func (t *Tracker) RecordRequest() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.cleanup(now)

	t.requests = append(t.requests, now)
}

// RecordTokenUsage records token usage for a request.
func (t *Tracker) RecordTokenUsage(count int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.cleanup(now)

	t.tokens = append(t.tokens, tokenEntry{
		timestamp: now,
		count:     count,
	})
}

// UpdateLimits updates the rate limit info from API response headers.
func (t *Tracker) UpdateLimits(requestsRemaining, requestsLimit, tokensRemaining, tokensLimit int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requestsLimit = requestsLimit
	t.tokensLimit = tokensLimit

	// Estimate reset time based on when we started tracking
	// In production, this would come from actual API headers
	now := time.Now()
	windowEnd := now.Add(t.config.WindowDuration)

	if !t.requestsReset.IsZero() && t.requestsReset.After(now) {
		// Keep existing reset time if still valid
	} else {
		t.requestsReset = windowEnd
	}

	if !t.tokensReset.IsZero() && t.tokensReset.After(now) {
		// Keep existing reset time if still valid
	} else {
		t.tokensReset = windowEnd
	}
}

// GetStatus returns the current rate limit status.
func (t *Tracker) GetStatus() RateLimitStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()

	// Count requests in current window
	requestCount := t.countRequests(now)

	// Count tokens in current window
	tokenCount := t.countTokens(now)

	// Calculate remaining
	requestsRemaining := max(0, t.requestsLimit-requestCount)
	tokensRemaining := max(0, t.tokensLimit-tokenCount)

	return RateLimitStatus{
		RequestsRemaining: requestsRemaining,
		RequestsLimit:     t.requestsLimit,
		RequestsReset:     t.requestsReset,
		TokensRemaining:   tokensRemaining,
		TokensLimit:       t.tokensLimit,
		TokensReset:       t.tokensReset,
		IsLimited:         requestsRemaining <= 0 || tokensRemaining <= 0,
	}
}

// cleanup removes entries outside the current time window.
func (t *Tracker) cleanup(now time.Time) {
	windowStart := now.Add(-t.config.WindowDuration)

	// Clean requests
	i := 0
	for _, ts := range t.requests {
		if ts.After(windowStart) {
			t.requests[i] = ts
			i++
		}
	}
	t.requests = t.requests[:i]

	// Clean tokens
	j := 0
	for _, entry := range t.tokens {
		if entry.timestamp.After(windowStart) {
			t.tokens[j] = entry
			j++
		}
	}
	t.tokens = t.tokens[:j]
}

// countRequests counts requests in the current window.
func (t *Tracker) countRequests(now time.Time) int {
	windowStart := now.Add(-t.config.WindowDuration)
	count := 0
	for _, ts := range t.requests {
		if ts.After(windowStart) {
			count++
		}
	}
	return count
}

// countTokens counts tokens in the current window.
func (t *Tracker) countTokens(now time.Time) int {
	windowStart := now.Add(-t.config.WindowDuration)
	count := 0
	for _, entry := range t.tokens {
		if entry.timestamp.After(windowStart) {
			count += entry.count
		}
	}
	return count
}

// Reset clears all tracking data.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requests = make([]time.Time, 0)
	t.tokens = make([]tokenEntry, 0)
}

// GetRequestCount returns the number of requests in the current window.
func (t *Tracker) GetRequestCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countRequests(time.Now())
}

// GetTokenCount returns the number of tokens used in the current window.
func (t *Tracker) GetTokenCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.countTokens(time.Now())
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
