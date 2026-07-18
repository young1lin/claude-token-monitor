package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
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

// currentOS allows tests to override runtime.GOOS for cross-platform coverage.
var currentOS = runtime.GOOS

// detectWideCharTerminal reports whether the terminal renders East Asian
// Ambiguous characters (· × → ° … and similar symbols) at width 2.
//
// IMPORTANT: go-runewidth computes emoji width (📁🌿…) independently and always
// returns 2, so this flag ONLY affects ambiguous-width symbols — never emoji.
// Almost every modern terminal (macOS Terminal.app, iTerm2, VSCode, WARP,
// Windows Terminal, cmd, PowerShell) renders ambiguous symbols at width 1 by
// default, so we default to false. If your locale/terminal genuinely renders
// ambiguous characters wide (some CJK-locale configs), opt in with
// STATUSLINE_AMBIGUOUS_WIDE=1.
func detectWideCharTerminal() bool {
	if os.Getenv("STATUSLINE_AMBIGUOUS_WIDE") == "1" {
		return true
	}
	return false
}

// detectNarrowBlockTerminal reports whether the current terminal renders Block
// Elements (█░▓▒) at width 1. go-runewidth reports █ as width 2 independent of
// EastAsianWidth, so for these terminals we must override to width 1 or the
// progress bar column alignment drifts (the "|" separators stop lining up).
//
// Width-1 terminals:
//   - macOS Terminal.app (TERM_PROGRAM=Apple_Terminal)
//   - VSCode integrated terminal (TERM_PROGRAM=vscode), any OS
//   - WARP (TERM_PROGRAM=WarpTerminal), any OS
//   - Windows cmd / PowerShell / conhost (no WT_SESSION)
//
// Width-2 terminals (no override): iTerm2, Windows Terminal, Ghostty, etc.
func detectNarrowBlockTerminal() bool {
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal", "vscode", "WarpTerminal":
		return true
	}
	// Windows classic console (cmd, PowerShell, conhost) renders blocks at width 1.
	if currentOS == "windows" && os.Getenv("WT_SESSION") == "" {
		return true
	}
	return false
}

func init() {
	// Configure the Condition that go-runewidth's RuneWidth actually consults.
	// Setting the package-level runewidth.EastAsianWidth variable is a NO-OP in
	// current go-runewidth — RuneWidth delegates to DefaultCondition. We must set
	// the field directly and rebuild its lookup table for the change to take.
	//
	// Note: emoji (📁🌿…) width is computed independently and is always 2,
	// regardless of this flag. This flag ONLY governs East Asian Ambiguous
	// symbols (· × → ° …), which almost all terminals render at width 1.
	runewidth.DefaultCondition.EastAsianWidth = detectWideCharTerminal()
	runewidth.DefaultCondition.CreateLUT()

	// Block Elements (█░▓▒) are reported as width 2 by go-runewidth regardless of
	// EastAsianWidth, but some terminals render them at width 1. Force width 1 for
	// those terminals or multi-line "|" column alignment drifts.
	layout.UseNarrowBlockWidth = detectNarrowBlockTerminal()
}

func main() {
	run(os.Stdin, os.Stdout, os.Stderr, os.Args)
}

// run contains the actual statusline logic, separated from main() for testability.
// It accepts stdin, stdout, stderr, and args as parameters so tests can inject buffers.
func run(stdin io.Reader, stdout, stderr io.Writer, args []string) {
	// Handle --version flag
	if len(args) > 1 && (args[1] == "--version" || args[1] == "-v") {
		fmt.Fprintf(stdout, "statusline version %s (commit: %s)\n", version, commit)
		return
	}

	// Parse CLI flags. We intentionally avoid the `flag` package here because
	// the rest of the entrypoint already uses ad-hoc scanning and we want to
	// stay friendly to unknown future flags rather than aborting on them.
	debugMode := false
	proxyCLI := ""
	for i, arg := range args {
		switch {
		case arg == "--debug":
			debugMode = true
		case strings.HasPrefix(arg, "--proxy="):
			proxyCLI = strings.TrimPrefix(arg, "--proxy=")
		case arg == "--proxy" && i+1 < len(args):
			// Tolerate the space-separated form (`--proxy URL`) too.
			proxyCLI = args[i+1]
		}
	}

	// Initialize Windows console for UTF-8 and ANSI support
	initConsole()

	// Read all input from stdin
	inputBytes, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading stdin: %v\n", err)
		return
	}

	// Trim null bytes
	inputBytes = trimNullBytes(inputBytes)
	if len(inputBytes) == 0 {
		return
	}

	// Write debug file if --debug is enabled
	if debugMode {
		// Get executable directory
		exePath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "Debug: failed to get executable path: %v\n", err)
		} else {
			exeDir := filepath.Dir(exePath)
			debugFile := filepath.Join(exeDir, "statusline.debug")

			// Mask user home directory in raw JSON for privacy
			debugJSON := string(inputBytes)
			if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
				// On Windows, JSON escapes backslashes so C:\Users\xxx becomes C:\\Users\\xxx
				// On Unix, no escaping needed for forward slashes
				if currentOS == "windows" {
					escapedHomeDir := strings.ReplaceAll(homeDir, "\\", "\\\\")
					debugJSON = strings.ReplaceAll(debugJSON, escapedHomeDir, "~")
				} else {
					debugJSON = strings.ReplaceAll(debugJSON, homeDir, "~")
				}
			}

			// Build new entry: timestamp line + JSON line

			// Read existing lines
			var lines []string
			if existingData, err := os.ReadFile(debugFile); err == nil {
				lines = strings.Split(string(existingData), "\n")
				// Remove empty lines
				var cleanLines []string
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						cleanLines = append(cleanLines, line)
					}
				}
				lines = cleanLines
			}

			// Prepend new entry (2 lines)
			lines = append([]string{time.Now().Format("2006-01-02 15:04:05"), debugJSON}, lines...)

			// Keep only last 40 lines (20 entries)
			const maxLines = 40
			if len(lines) > maxLines {
				lines = lines[:maxLines]
			}

			// Write back
			finalContent := strings.Join(lines, "\n") + "\n"
			if err := os.WriteFile(debugFile, []byte(finalContent), 0644); err != nil {
				fmt.Fprintf(stderr, "Debug: failed to write debug file: %v\n", err)
			} else {
				fmt.Fprintf(stderr, "Debug: wrote to %s (%d entries)\n", debugFile, len(lines)/2)
			}
		}
	}

	// Parse input JSON
	var input content.StatusLineInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		fmt.Fprintf(stderr, "JSON parse error: %v\n", err)
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

	// Apply network + cache config before collectors run — the proxy targeting
	// api.anthropic.com must be in place before the quota collector issues its
	// OAuth-usage request, and the cache TTL governs whether that request even
	// happens this refresh. Precedence: --proxy flag > STATUSLINE_CLAUDE_PROXY
	// env > network.claudeAPIProxy YAML, all resolved in one place.
	content.SetClaudeAPIProxy(cfg.ResolveClaudeAPIProxy(proxyCLI))
	content.SetUsageCacheTTL(cfg.GetUsageCacheTTL())
	content.SetIdleWarnThreshold(cfg.GetIdleWarnThreshold())

	// Build content map using composers. Each collector emits display-ready
	// content (glyphs/prefixes included), so the entrypoint no longer post-
	// processes individual cells here — it just hands the map to the layout.
	contentMap := contentMgr.Compose(&input, summary)

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
		fmt.Fprintln(stdout, line)
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
		FailedTools:    parserSummary.FailedTools,
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
		content.NewGitWorktreeCollector(),
		content.NewMemoryFilesCollector(),
		content.NewSkillsCollector(),
		content.NewSessionTotalCollector(),
		content.NewAgentCollector(),
		content.NewTodoCollector(),
		content.NewToolsCollector(),
		content.NewSessionDurationCollector(),
		content.NewCurrentTimeCollector(),
		content.NewQuotaCollector(),
		content.NewToolStatusDetailCollector(),
		content.NewParentMemoryCollector(),
		content.NewModeFlagsCollector(),
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
