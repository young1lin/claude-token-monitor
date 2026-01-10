package tui

import (
	"github.com/young1lin/claude-token-monitor/internal/ratelimit"
)

// TokenUpdateMsg is sent when new token data is available
type TokenUpdateMsg struct {
	SessionID    string
	Model        string
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	TotalTokens  int
	Cost         float64
	ContextPct   float64
}

// HistoryLoadedMsg is sent when history is loaded
type HistoryLoadedMsg struct {
	History []HistoryEntry
}

// SessionFoundMsg is sent when a session is detected
type SessionFoundMsg struct {
	SessionID string
	Project   string
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically to update the UI
type TickMsg struct {
	Time string
}

// WatcherStartedMsg is sent when the file watcher starts
type WatcherStartedMsg struct{}

// WatcherFailedMsg is sent when the file watcher fails
type WatcherFailedMsg struct {
	Err error
}

// TranscriptUpdateMsg is sent when transcript info is available (single-line mode)
type TranscriptUpdateMsg struct {
	GitBranch      string
	GitStatus      string
	ActiveTools    []string
	CompletedTools map[string]int
	Agents         []AgentInfo
	TodoTotal      int
	TodoCompleted  int
}

// RateLimitUpdateMsg is sent when rate limit status changes
type RateLimitUpdateMsg struct {
	Status ratelimit.RateLimitStatus
}

// SessionListUpdateMsg is sent when the list of sessions changes
type SessionListUpdateMsg struct {
	Sessions map[string]*SessionViewState
}

// SessionSwitchMsg is sent to switch to a different session
type SessionSwitchMsg struct {
	SessionID string
}

// ToggleSessionListMsg toggles the session list visibility
type ToggleSessionListMsg struct{}
