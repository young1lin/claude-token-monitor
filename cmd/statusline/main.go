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

// detectWideCharTerminal checks if the current terminal renders
// emoji as width 2 characters.
// Returns true ONLY for terminals known to use wide character rendering.
func detectWideCharTerminal() bool {
	// macOS terminals typically use wide character rendering
	if currentOS == "darwin" {
		return true
	}

	// Windows Terminal - the ONLY Windows terminal that uses wide chars
	if os.Getenv("WT_SESSION") != "" {
		return true
	}

	// iTerm2 (macOS, but check anyway)
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		return true
	}

	// Default: narrow character rendering (width 1)
	// This includes: VSCode Terminal, WARP, cmd.exe, PowerShell, etc.
	return false
}

func init() {
	// Set emoji width based on terminal detection for proper alignment.
	//
	// EastAsianWidth=true means emoji calculated as width 2
	// EastAsianWidth=false means emoji calculated as width 1
	//
	// Only these terminals use wide character rendering:
	// - macOS (all terminals)
	// - Windows Terminal (WT_SESSION)
	// - iTerm2 (TERM_PROGRAM=iTerm.app)
	//
	// Other terminals (VSCode, WARP, cmd, PowerShell) use narrow rendering.
	runewidth.EastAsianWidth = detectWideCharTerminal()

	// For non-Windows-Terminal on Windows, enable narrow Block Elements.
	// VSCode Terminal and WARP render all Block Elements (█░▓▒) as width 1,
	// but go-runewidth reports █ as width 2. This causes misalignment.
	if currentOS == "windows" && os.Getenv("WT_SESSION") == "" {
		layout.UseNarrowBlockWidth = true
	}
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

	// Check for --debug flag
	debugMode := false
	for _, arg := range args {
		if arg == "--debug" {
			debugMode = true
			break
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
