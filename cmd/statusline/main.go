package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/young1lin/claude-token-monitor/internal/parser"
	"github.com/young1lin/claude-token-monitor/internal/statusline/config"
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
	"github.com/young1lin/claude-token-monitor/internal/statusline/content/composers"
	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
	"github.com/young1lin/claude-token-monitor/internal/statusline/render"
)

// Version information injected by ldflags during build
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Handle --version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("statusline version %s (commit: %s)\n", version, commit)
		return
	}
	// Initialize Windows console for UTF-8 and ANSI support
	initConsole()

	// Read all input from stdin
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		return
	}

	// Trim null bytes
	inputBytes = trimNullBytes(inputBytes)
	if len(inputBytes) == 0 {
		return
	}

	// Parse input JSON
	var input content.StatusLineInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		fmt.Fprintf(os.Stderr, "JSON parse error: %v\n", err)
		return
	}

	// Parse transcript if available
	var summary *content.TranscriptSummary
	if input.TranscriptPath != "" {
		parserSummary, _ := parser.ParseTranscriptLastNLines(input.TranscriptPath, 100)
		if parserSummary != nil {
			summary = convertToContentSummary(parserSummary)
		}
	}
	if summary == nil {
		summary = &content.TranscriptSummary{}
	}

	// === Layer 1: Content Collection ===
	contentMgr := content.NewManager()
	registerAllCollectors(contentMgr)
	registerAllComposers(contentMgr)

	// Load configuration
	cfg, err := config.Load(input.Cwd)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Build content map using composers
	contentMap := contentMgr.Compose(&input, summary)

	// Apply folder prefix
	if folder, ok := contentMap["folder"]; ok && folder != "" {
		contentMap["folder"] = "📁 " + folder
	}
	// Apply version prefix
	if version, ok := contentMap["claude-version"]; ok && version != "" {
		contentMap["claude-version"] = "v" + version
	}

	// === Layer 2: Layout ===
	defaultLayout := layout.DefaultLayout()
	gridLayout := layout.FilterLayout(defaultLayout, cfg)
	grid := layout.NewGrid(gridLayout, contentMap)

	// === Layer 3: Render ===
	tableRenderer := render.NewTableRenderer(grid)

	// Check if single-line mode is enabled
	// Environment variable takes precedence over config file
	singleLine := os.Getenv("STATUSLINE_SINGLELINE") == "1" || cfg.IsSingleLine()

	var lines []string
	if singleLine {
		lines = []string{tableRenderer.RenderSingleLine()}
	} else {
		lines = tableRenderer.Render()
	}

	// Print output
	for _, line := range lines {
		fmt.Println(line)
	}
}

func trimNullBytes(data []byte) []byte {
	result := make([]byte, 0, len(data))
	for _, b := range data {
		if b != 0 {
			result = append(result, b)
		}
	}
	return result
}

// convertToContentSummary converts parser.TranscriptSummary to content.TranscriptSummary
func convertToContentSummary(parserSummary *parser.TranscriptSummary) *content.TranscriptSummary {
	if parserSummary == nil {
		return nil
	}

	// Convert agents
	agents := make([]content.AgentInfo, len(parserSummary.Agents))
	for i, agent := range parserSummary.Agents {
		agents[i] = content.AgentInfo{
			Type: agent.Type,
			Desc: agent.Desc,
		}
	}

	return &content.TranscriptSummary{
		GitBranch:      parserSummary.GitBranch,
		GitStatus:      parserSummary.GitStatus,
		ActiveTools:    parserSummary.ActiveTools,
		CompletedTools: parserSummary.CompletedTools,
		Agents:         agents,
		TodoTotal:      parserSummary.TodoTotal,
		TodoCompleted:  parserSummary.TodoCompleted,
		SessionStart:   parserSummary.SessionStart,
		SessionEnd:     parserSummary.SessionEnd,
	}
}

// registerAllCollectors registers all content collectors
func registerAllCollectors(mgr *content.Manager) {
	mgr.RegisterAll(
		content.NewFolderCollector(),
		content.NewModelCollector(),
		content.NewTokenBarCollector(),
		content.NewTokenInfoCollector(),
		content.NewClaudeVersionCollector(),
		content.NewGitBranchCollector(),
		content.NewGitStatusCollector(),
		content.NewGitRemoteCollector(),
		content.NewMemoryFilesCollector(),
		content.NewAgentCollector(),
		content.NewTodoCollector(),
		content.NewToolsCollector(),
		content.NewSessionDurationCollector(),
		content.NewCurrentTimeCollector(),
		content.NewQuotaCollector(),
	)
}

// registerAllComposers registers all built-in composers
func registerAllComposers(mgr *content.Manager) {
	mgr.RegisterComposers(
		composers.NewTokenComposer(),
		composers.NewGitComposer(),
		composers.NewTimeQuotaComposer(),
	)
}
