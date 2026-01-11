package tui

import (
	"fmt"
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

	case ProjectsDiscoveredMsg:
		m.projects = msg.Projects
		if len(msg.Projects) == 0 {
			m.ready = true
			m.err = fmt.Errorf("no projects found")
		} else if len(msg.Projects) == 1 {
			// Auto-select single project
			m.viewState = ViewMonitoring
			return m, func() tea.Msg {
				return ProjectSelectedMsg{Project: msg.Projects[0]}
			}
		} else {
			m.viewState = ViewProjectSelection
			m.selectedProject = 0
			m.ready = true
		}
		return m, nil

	case ProjectSelectedMsg:
		m.viewState = ViewMonitoring
		m.project = msg.Project.Name
		// Reset all monitoring data for fresh start
		m.sessionID = ""
		m.model = ""
		m.inputTokens = 0
		m.outputTokens = 0
		m.cacheTokens = 0
		m.totalTokens = 0
		m.cost = 0
		m.contextPct = 0
		m.gitBranch = ""
		m.gitStatus = ""
		m.activeTools = nil
		m.completedTools = nil
		m.agents = nil
		m.todoTotal = 0
		m.todoCompleted = 0
		m.ready = false // Show loading while switching
		// Trigger session discovery for selected project
		return m, func() tea.Msg {
			return SessionFoundMsg{
				SessionID: msg.Project.MostRecentSession.ID,
				Project:   msg.Project.Name,
			}
		}

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
	// Handle project selection keys
	if m.viewState == ViewProjectSelection {
		return m.handleProjectSelectionKeys(msg)
	}

	// Existing monitoring view handlers
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

// handleProjectSelectionKeys handles keys in project selection view
func (m Model) handleProjectSelectionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up":
		if m.selectedProject > 0 {
			m.selectedProject--
		}
		return m, nil

	case "down":
		if m.selectedProject < len(m.projects)-1 {
			m.selectedProject++
		}
		return m, nil

	case "enter":
		if m.selectedProject >= 0 && m.selectedProject < len(m.projects) {
			selected := m.projects[m.selectedProject]
			return m, func() tea.Msg {
				return ProjectSelectedMsg{Project: selected}
			}
		}
		return m, nil

	case "esc":
		m.quitting = true
		return m, tea.Quit

	case "r":
		// Retry project discovery
		return m, func() tea.Msg {
			// For now, just resend the same projects
			// In a full implementation, this would trigger a re-discovery
			return ProjectsDiscoveredMsg{Projects: m.projects}
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
