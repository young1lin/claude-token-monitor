// Package monitor provides session monitoring and management.
package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/ratelimit"
)

// SessionState represents the current state of a session.
type SessionState struct {
	SessionID    string
	FilePath     string
	Project      string
	Model        string
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	TotalTokens  int
	Cost         float64
	ContextPct   float64
	IsActive     bool
	LastUpdate   time.Time
	RateLimit    ratelimit.RateLimitStatus
}

// SessionManager manages multiple Claude Code sessions.
type SessionManager struct {
	mu              sync.RWMutex
	sessions        map[string]*SessionState
	activeSessionID string
	watchers        map[string]WatcherInterface
 trackers        map[string]*ratelimit.Tracker
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SessionState),
		watchers: make(map[string]WatcherInterface),
		trackers: make(map[string]*ratelimit.Tracker),
	}
}

// AddSession adds a new session to the manager.
func (sm *SessionManager) AddSession(session *SessionInfo) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[session.ID]; exists {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	sm.sessions[session.ID] = &SessionState{
		SessionID:  session.ID,
		FilePath:   session.FilePath,
		Project:    session.Project,
		IsActive:   true,
		LastUpdate: time.Now(),
	}

	// Create rate limit tracker for this session
	sm.trackers[session.ID] = ratelimit.NewTracker(ratelimit.DefaultTrackerConfig())

	return nil
}

// RemoveSession removes a session from the manager.
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close watcher if exists
	if watcher, ok := sm.watchers[sessionID]; ok {
		watcher.Close()
		delete(sm.watchers, sessionID)
	}

	delete(sm.sessions, sessionID)
	delete(sm.trackers, sessionID)

	// If this was the active session, switch to another
	if sm.activeSessionID == sessionID {
		sm.activeSessionID = ""
		for id := range sm.sessions {
			sm.activeSessionID = id
			break
		}
	}
}

// UpdateSessionTokens updates token usage for a session.
func (sm *SessionManager) UpdateSessionTokens(sessionID string, input, output, cache int, cost float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return
	}

	session.InputTokens += input
	session.OutputTokens += output
	session.CacheTokens += cache
	session.TotalTokens = session.InputTokens + session.OutputTokens
	session.Cost = cost
	session.LastUpdate = time.Now()

	// Update rate limit tracker
	if tracker, ok := sm.trackers[sessionID]; ok {
		tracker.RecordRequest()
		tracker.RecordTokenUsage(input + output)
		session.RateLimit = tracker.GetStatus()
	}
}

// SetActiveSession sets the active session.
func (sm *SessionManager) SetActiveSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.sessions[sessionID]; !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sm.activeSessionID = sessionID
	return nil
}

// GetActiveSession returns the active session state.
func (sm *SessionManager) GetActiveSession() *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.activeSessionID == "" {
		return nil
	}
	return sm.sessions[sm.activeSessionID]
}

// GetSession returns a session by ID.
func (sm *SessionManager) GetSession(sessionID string) *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.sessions[sessionID]
}

// GetAllSessions returns all sessions.
func (sm *SessionManager) GetAllSessions() []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetSessionIDs returns all session IDs.
func (sm *SessionManager) GetSessionIDs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	return ids
}

// SetWatcher sets a file watcher for a session.
func (sm *SessionManager) SetWatcher(sessionID string, watcher WatcherInterface) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.watchers[sessionID] = watcher
}

// GetWatcher returns the watcher for a session.
func (sm *SessionManager) GetWatcher(sessionID string) WatcherInterface {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.watchers[sessionID]
}

// CloseAll closes all watchers and cleans up resources.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, watcher := range sm.watchers {
		watcher.Close()
	}

	sm.sessions = make(map[string]*SessionState)
	sm.watchers = make(map[string]WatcherInterface)
	sm.trackers = make(map[string]*ratelimit.Tracker)
	sm.activeSessionID = ""
}

// GetRateLimitStatus returns the rate limit status for a session.
func (sm *SessionManager) GetRateLimitStatus(sessionID string) ratelimit.RateLimitStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if tracker, ok := sm.trackers[sessionID]; ok {
		return tracker.GetStatus()
	}
	return ratelimit.RateLimitStatus{}
}

// GetSessionCount returns the number of sessions.
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.sessions)
}

// NextSession cycles to the next session.
func (sm *SessionManager) NextSession() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		sm.activeSessionID = ""
		return
	}

	// Find current index
	currentIdx := -1
	for i, id := range ids {
		if id == sm.activeSessionID {
			currentIdx = i
			break
		}
	}

	// Move to next (with wraparound)
	nextIdx := (currentIdx + 1) % len(ids)
	sm.activeSessionID = ids[nextIdx]
}

// PreviousSession cycles to the previous session.
func (sm *SessionManager) PreviousSession() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		sm.activeSessionID = ""
		return
	}

	// Find current index
	currentIdx := -1
	for i, id := range ids {
		if id == sm.activeSessionID {
			currentIdx = i
			break
		}
	}

	// If no active session, start at last
	if currentIdx == -1 {
		currentIdx = 0
	}

	// Move to previous (with wraparound)
	prevIdx := (currentIdx - 1 + len(ids)) % len(ids)
	sm.activeSessionID = ids[prevIdx]
}
