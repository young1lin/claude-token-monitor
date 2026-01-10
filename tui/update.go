package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case TickMsg:
		m.lastUpdate = msg.Time
		return m, tickCmd()

	case SessionFoundMsg:
		m.sessionID = msg.SessionID
		m.project = msg.Project
		m.ready = true
		return m, nil

	case TokenUpdateMsg:
		m.sessionID = msg.SessionID
		m.model = msg.Model
		m.inputTokens = msg.InputTokens
		m.outputTokens = msg.OutputTokens
		m.cacheTokens = msg.CacheTokens
		m.totalTokens = msg.TotalTokens
		m.cost = msg.Cost
		m.contextPct = msg.ContextPct
		m.err = nil
		return m, nil

	case HistoryLoadedMsg:
		m.history = msg.History
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, nil

	case WatcherStartedMsg:
		m.ready = true
		m.err = nil
		return m, nil

	case WatcherFailedMsg:
		m.err = msg.Err
		m.ready = false
		return m, nil

	case TranscriptUpdateMsg:
		m.gitBranch = msg.GitBranch
		m.gitStatus = msg.GitStatus
		m.activeTools = msg.ActiveTools
		m.completedTools = msg.CompletedTools
		m.agents = msg.Agents
		m.todoTotal = msg.TodoTotal
		m.todoCompleted = msg.TodoCompleted
		return m, nil

	case RateLimitUpdateMsg:
		m.rateLimitStatus = msg.Status
		return m, nil

	case SessionListUpdateMsg:
		m.sessions = msg.Sessions
		// If this is the first update and there are multiple sessions, show the list
		if len(msg.Sessions) > 1 && !m.showSessionList {
			m.showSessionList = true
		}
		// Set active session if not set
		if m.activeSessionID == "" {
			for id := range msg.Sessions {
				m.activeSessionID = id
				break
			}
		}
		return m, nil

	case SessionSwitchMsg:
		m.activeSessionID = msg.SessionID
		// Update current display data from switched session
		if session, ok := m.sessions[msg.SessionID]; ok {
			m.sessionID = session.SessionID
			m.project = session.Project
			m.model = session.Model
			m.inputTokens = session.InputTokens
			m.outputTokens = session.OutputTokens
			m.cacheTokens = session.CacheTokens
			m.totalTokens = session.TotalTokens
			m.cost = session.Cost
			m.contextPct = session.ContextPct
		}
		return m, nil

	case ToggleSessionListMsg:
		if len(m.sessions) > 1 {
			m.showSessionList = !m.showSessionList
		}
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	case "r":
		// Refresh history
		return m, tea.Batch(tickCmd())
	case "tab":
		// Switch to next session
		return m, func() tea.Msg {
			sessionIDs := make([]string, 0, len(m.sessions))
			for id := range m.sessions {
				sessionIDs = append(sessionIDs, id)
			}
			// Find current index
			currentIdx := -1
			for i, id := range sessionIDs {
				if id == m.activeSessionID {
					currentIdx = i
					break
				}
			}
			// Move to next
			nextIdx := (currentIdx + 1) % len(sessionIDs)
			return SessionSwitchMsg{SessionID: sessionIDs[nextIdx]}
		}
	case "shift+tab":
		// Switch to previous session
		return m, func() tea.Msg {
			sessionIDs := make([]string, 0, len(m.sessions))
			for id := range m.sessions {
				sessionIDs = append(sessionIDs, id)
			}
			// Find current index
			currentIdx := -1
			for i, id := range sessionIDs {
				if id == m.activeSessionID {
					currentIdx = i
					break
				}
			}
			// If no active session, start at last
			if currentIdx == -1 {
				currentIdx = 0
			}
			// Move to previous
			prevIdx := (currentIdx - 1 + len(sessionIDs)) % len(sessionIDs)
			return SessionSwitchMsg{SessionID: sessionIDs[prevIdx]}
		}
	case "f1":
		// Toggle session list visibility
		return m, func() tea.Msg {
			return ToggleSessionListMsg{}
		}
	}

	return m, nil
}

// tickCmd returns a command that sends TickMsg messages
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{Time: t.Format("15:04:05")}
	})
}
