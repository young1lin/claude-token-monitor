package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// RateLimitState stores the rate limit state
type RateLimitState struct {
	Requests []time.Time `json:"requests"`
	Tokens   []int       `json:"tokens"`
}

// SimpleTracker provides file-based rate limit tracking
type SimpleTracker struct {
	stateFile string
	state     *RateLimitState
}

// NewSimpleTracker creates a new rate limit tracker
func NewSimpleTracker() *SimpleTracker {
	// Use temp directory for state file
	tempDir := os.TempDir()
	stateFile := filepath.Join(tempDir, "claude-statusline-ratelimit.json")

	return &SimpleTracker{
		stateFile: stateFile,
		state:     &RateLimitState{
			Requests: make([]time.Time, 0),
			Tokens:   make([]int, 0),
		},
	}
}

// Load loads the state from file
func (t *SimpleTracker) Load() error {
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // First run, no state file
		}
		return err
	}

	return json.Unmarshal(data, t.state)
}

// Save saves the state to file
func (t *SimpleTracker) Save() error {
	data, err := json.Marshal(t.state)
	if err != nil {
		return err
	}

	return os.WriteFile(t.stateFile, data, 0644)
}

// RecordRequest records a new request
func (t *SimpleTracker) RecordRequest(tokens int) error {
	// Clean old requests (older than 1 minute)
	t.cleanup()

	// Add new request
	t.state.Requests = append(t.state.Requests, time.Now())
	t.state.Tokens = append(t.state.Tokens, tokens)

	return t.Save()
}

// GetRateLimitStatus returns the current rate limit status
func (t *SimpleTracker) GetRateLimitStatus() (requestsRemaining, requestsLimit, tokensRemaining, tokensLimit int) {
	// Clean old requests first
	t.cleanup()

	// Default limits (Claude API)
	requestsLimit = 120
	tokensLimit = 100000

	// Count requests in last minute
	requestsInWindow := len(t.state.Requests)
	tokensInWindow := 0
	for _, tok := range t.state.Tokens {
		tokensInWindow += tok
	}

	requestsRemaining = requestsLimit - requestsInWindow
	if requestsRemaining < 0 {
		requestsRemaining = 0
	}

	tokensRemaining = tokensLimit - tokensInWindow
	if tokensRemaining < 0 {
		tokensRemaining = 0
	}

	return
}

// cleanup removes entries older than 1 minute
func (t *SimpleTracker) cleanup() {
	cutoff := time.Now().Add(-time.Minute)

	// Filter requests
	var validRequests []time.Time
	for _, req := range t.state.Requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}

	// Filter tokens (keep in sync with requests)
	validTokens := make([]int, 0)
	if len(validRequests) > 0 {
		// Keep the last N tokens where N equals valid requests
		startIdx := len(t.state.Tokens) - len(validRequests)
		if startIdx < 0 {
			startIdx = 0
		}
		validTokens = t.state.Tokens[startIdx:]
	}

	t.state.Requests = validRequests
	t.state.Tokens = validTokens
}
