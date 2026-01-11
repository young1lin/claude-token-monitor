package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/ratelimit"
)

// ViewState represents the current UI view
type ViewState int

const (
	ViewProjectSelection ViewState = iota
	ViewMonitoring
)

// ProjectInfo contains information about a project
type ProjectInfo struct {
	Name              string
	SessionCount      int
	LastActivity      time.Time
	MostRecentSession *monitor.SessionInfo
}

// Model represents the application state
type Model struct {
	// Session info
	sessionID string
	project   string
	model     string

	// Multi-session support
	sessions        map[string]*SessionViewState
	activeSessionID string
	showSessionList bool

	// Token statistics
	inputTokens  int
	outputTokens int
	cacheTokens  int
	totalTokens  int
	contextPct   float64

	// Cost
	cost float64

	// Rate limit status
	rateLimitStatus ratelimit.RateLimitStatus

	// History
	history []HistoryEntry

	// State
	ready      bool
	quitting   bool
	lastUpdate string
	singleLine bool

	// View state management
	viewState       ViewState
	projects        []ProjectInfo
	selectedProject int

	// Transcript info (for single-line mode)
	activeTools    []string
	completedTools map[string]int
	agents         []AgentInfo
	todoTotal      int
	todoCompleted  int
	gitBranch      string
	gitStatus      string
	sessionStart   string
	sessionEnd     string

	// Error state
	err error

	// Styles
	styles Styles
}

// AgentInfo represents information about a running agent
type AgentInfo struct {
	Type      string
	Desc      string
	Elapsed   int
}

// HistoryEntry represents a historical session entry
type HistoryEntry struct {
	ID        string
	Timestamp string
	Tokens    int
	Cost      float64
	Project   string
}

// SessionViewState represents the view state for a single session
type SessionViewState struct {
	SessionID    string
	Project      string
	Model        string
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	TotalTokens  int
	Cost         float64
	ContextPct   float64
	IsActive     bool
}

// Styles contains the Lipgloss styles for the UI
type Styles struct {
	Border         lipgloss.Style
	Header         lipgloss.Style
	Title          lipgloss.Style
	Subtitle       lipgloss.Style
	Label          lipgloss.Style
	Value          lipgloss.Style
	Highlight      lipgloss.Style
	Muted          lipgloss.Style
	Error          lipgloss.Style
	ProgressFull   lipgloss.Style
	ProgressEmpty  lipgloss.Style
	HistoryItem    lipgloss.Style
	HistoryItemOld lipgloss.Style
	Cost           lipgloss.Style
	CostHigh       lipgloss.Style

	// Rate limit styles
	RateLimitOk      lipgloss.Style
	RateLimitWarning lipgloss.Style
	RateLimitCritical lipgloss.Style

	// Session list styles
	SessionListBorder  lipgloss.Style
	SessionListHeader  lipgloss.Style
	SessionListItem    lipgloss.Style
	SessionListActive  lipgloss.Style
	SessionListMuted   lipgloss.Style
}

// DefaultStyles returns the default UI styles
func DefaultStyles() Styles {
	var styles Styles

	// Color palette
	primaryColor := lipgloss.Color("86")    // Green
	secondaryColor := lipgloss.Color("239") // Grey
	errorColor := lipgloss.Color("196")     // Red
	warnColor := lipgloss.Color("208")      // Orange

	// Border style
	styles.Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(secondaryColor)

	// Header
	styles.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(0, 1)

	styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255"))

	styles.Subtitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))

	// Labels and values
	styles.Label = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Width(12)

	styles.Value = lipgloss.NewStyle().
		Foreground(lipgloss.Color("255"))

	styles.Highlight = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)

	styles.Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	// Error
	styles.Error = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	// Progress bar
	styles.ProgressFull = lipgloss.NewStyle().
		Background(primaryColor).
		Foreground(lipgloss.Color("0"))

	styles.ProgressEmpty = lipgloss.NewStyle().
		Background(lipgloss.Color("235"))

	// History
	styles.HistoryItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	styles.HistoryItemOld = lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	// Cost
	styles.Cost = lipgloss.NewStyle().
		Foreground(lipgloss.Color("228"))

	styles.CostHigh = lipgloss.NewStyle().
		Foreground(warnColor).
		Bold(true)

	// Rate limit styles
	styles.RateLimitOk = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	styles.RateLimitWarning = lipgloss.NewStyle().
		Foreground(warnColor).
		Bold(true)

	styles.RateLimitCritical = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	// Session list styles
	styles.SessionListBorder = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(secondaryColor)

	styles.SessionListHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)

	styles.SessionListItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	styles.SessionListActive = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	styles.SessionListMuted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	return styles
}

// NewModel creates a new Model with default values
func NewModel(singleLine bool) Model {
	return Model{
		styles:           DefaultStyles(),
		ready:            false,
		history:          make([]HistoryEntry, 0, 10),
		singleLine:       singleLine,
		sessions:         make(map[string]*SessionViewState),
		showSessionList:  false,
		completedTools:   make(map[string]int),
		activeTools:      make([]string, 0),
		agents:           make([]AgentInfo, 0),
		viewState:        ViewProjectSelection,
		projects:         make([]ProjectInfo, 0),
		selectedProject:  0,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}
