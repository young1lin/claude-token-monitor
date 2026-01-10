package main

import (
	"fmt"
	"io/fs"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/young1lin/claude-token-monitor/internal/config"
	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/parser"
	"github.com/young1lin/claude-token-monitor/internal/ratelimit"
	"github.com/young1lin/claude-token-monitor/internal/store"
	"github.com/young1lin/claude-token-monitor/tui"
)

// ProgramSender is an interface for sending messages to a Bubbletea program
type ProgramSender interface {
	Send(msg tea.Msg)
}

// AppDependencies contains the dependencies for the main application
type AppDependencies struct {
	ProjectsDir    string
	SessionFinder  func() (*monitor.SessionInfo, error)
	DBOpener       func(string) (*store.DB, error)
	WatcherCreator func(string) (monitor.WatcherInterface, error)
	ProgramRunner  func(*tea.Program) error
	Stat           func(string) (fs.FileInfo, error)
	HistoryDBPath  func() string
	SingleLine     bool
}

func run(deps *AppDependencies) error {

	// Use injected Stat or default to os.Stat
	statFn := deps.Stat
	if statFn == nil {
		statFn = os.Stat
	}

	// Check if Claude Code data directory exists
	if _, err := statFn(deps.ProjectsDir); os.IsNotExist(err) {
		return fmt.Errorf("Claude Code data directory not found: %s\n\nPlease make sure Claude Code is installed and has been run at least once", deps.ProjectsDir)
	}

	// Find current session
	session, err := deps.SessionFinder()
	if err != nil {
		return fmt.Errorf("failed to find active Claude Code session: %v\n\nMake sure Claude Code is running and you have an active conversation", err)
	}

	// Get database path
	dbPath := deps.HistoryDBPath
	if dbPath == nil {
		dbPath = config.HistoryDBPath
	}

	// Open database
	db, err := deps.DBOpener(dbPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	// Load history
	historyRecords, err := db.GetRecentHistory(10)
	if err != nil {
		// Warning, not fatal
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	// Convert history to TUI format
	history := make([]tui.HistoryEntry, 0, len(historyRecords))
	for _, r := range historyRecords {
		if r.ID == session.ID {
			continue // Skip current session from history
		}
		history = append(history, tui.HistoryEntry{
			ID:        r.ID,
			Timestamp: r.Timestamp.Format("2006-01-02 15:04"),
			Tokens:    r.TotalTokens,
			Cost:      r.Cost,
			Project:   r.Project,
		})
	}

	// Create TUI model
	model := tui.NewModel(deps.SingleLine)

	// Start file watcher
	watcher, err := deps.WatcherCreator(session.FilePath)
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %v", err)
	}
	defer watcher.Close()

	// Create program options
	var opts []tea.ProgramOption
	opts = append(opts, tea.WithMouseCellMotion())
	if !deps.SingleLine {
		opts = append(opts, tea.WithAltScreen())
	}

	// Create initial program
	p := tea.NewProgram(model, opts...)

	// Start goroutine to handle file watching
	go func() {
		runWatchLoop(p, watcher, db, session, history, deps.SingleLine)
	}()

	// Run the program
	return deps.ProgramRunner(p)
}

// runWatchLoop runs the main watch loop that processes file changes
func runWatchLoop(sender ProgramSender, watcher monitor.WatcherInterface, db *store.DB, session *monitor.SessionInfo, history []tui.HistoryEntry, singleLine bool) {
	// Send initial messages
	sender.Send(tui.SessionFoundMsg{
		SessionID: session.ID,
		Project:   session.Project,
	})

	sender.Send(tui.HistoryLoadedMsg{History: history})

	sender.Send(tui.WatcherStartedMsg{})

	// Track running totals
	var totalInput, totalOutput, totalCache int
	var currentModel string

	// Initialize rate limit tracker
	tracker := ratelimit.NewTracker(ratelimit.DefaultTrackerConfig())

	// Start ticker for periodic rate limit updates
	rateLimitTicker := time.NewTicker(5 * time.Second)
	defer rateLimitTicker.Stop()

	// Send initial rate limit status
	sender.Send(tui.RateLimitUpdateMsg{Status: tracker.GetStatus()})

	// Process incoming lines
	for {
		select {
		case <-rateLimitTicker.C:
			// Send periodic rate limit updates
			sender.Send(tui.RateLimitUpdateMsg{Status: tracker.GetStatus()})

		case line, ok := <-watcher.Lines():
			if !ok {
				return
			}
			msg, err := parser.ParseLine([]byte(line))
			if err != nil || msg == nil {
				continue
			}

			// Record request in rate limit tracker
			tracker.RecordRequest()

			// Update model if changed
			if msg.Message.Model != "" && msg.Message.Model != currentModel {
				currentModel = msg.Message.Model
			}

			// Accumulate tokens
			totalInput += msg.Message.Usage.InputTokens
			totalOutput += msg.Message.Usage.OutputTokens
			totalCache += msg.Message.Usage.CacheReadInputTokens

			// Record token usage in rate limit tracker
			tokensThisRequest := msg.Message.Usage.InputTokens + msg.Message.Usage.OutputTokens
			tracker.RecordTokenUsage(tokensThisRequest)

			// Calculate totals
			totalTokens := totalInput + totalOutput
			contextPct := parser.CalculateContextPercentage(currentModel, totalTokens)
			cost := parser.CalculateCost(currentModel, totalInput, totalOutput, totalCache)

			// Update database
			_ = db.UpdateSessionTokens(session.ID, msg.Message.Usage.InputTokens, msg.Message.Usage.OutputTokens, msg.Message.Usage.CacheReadInputTokens, cost)

			// Send update to TUI
			sender.Send(tui.TokenUpdateMsg{
				SessionID:    session.ID,
				Model:        config.GetModelName(currentModel),
				InputTokens:  totalInput,
				OutputTokens: totalOutput,
				CacheTokens:  totalCache,
				TotalTokens:  totalTokens,
				Cost:         cost,
				ContextPct:   contextPct,
			})

			// In single-line mode, also parse transcript for additional info
			if singleLine && session.FilePath != "" {
				// Try to get actual project directory for git detection
				// The session.Project might be a normalized path, so we try to use it
				// If git commands fail, we just won't show a branch (fallback behavior)
				projectPath := session.Project
				summary, _ := parser.ParseTranscriptLastNLinesWithProjectPath(session.FilePath, 100, projectPath)

				// Convert agents to TUI format
				agents := make([]tui.AgentInfo, 0, len(summary.Agents))
				for _, a := range summary.Agents {
					agents = append(agents, tui.AgentInfo{
						Type:    a.Type,
						Desc:    a.Desc,
						Elapsed: a.Elapsed,
					})
				}

				sender.Send(tui.TranscriptUpdateMsg{
					GitBranch:      summary.GitBranch,
					GitStatus:      summary.GitStatus,
					ActiveTools:    summary.ActiveTools,
					CompletedTools: summary.CompletedTools,
					Agents:         agents,
					TodoTotal:      summary.TodoTotal,
					TodoCompleted:  summary.TodoCompleted,
				})
			}

		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			sender.Send(tui.WatcherFailedMsg{Err: fmt.Errorf("watcher error: %w", err)})
			return
		}
	}
}
