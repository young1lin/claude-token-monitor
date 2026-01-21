package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/young1lin/claude-token-monitor/internal/parser"
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
	"github.com/young1lin/claude-token-monitor/internal/statusline/render"
	"github.com/young1lin/claude-token-monitor/internal/update"
)

// Windows API functions for console control
var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutputCP       = modkernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP             = modkernel32.NewProc("SetConsoleCP")
	procGetConsoleMode           = modkernel32.NewProc("GetConsoleMode")
	procSetConsoleMode           = modkernel32.NewProc("SetConsoleMode")
	procGetStdHandle             = modkernel32.NewProc("GetStdHandle")
)

const (
	STD_OUTPUT_HANDLE                  = uintptr(-11 & 0xFFFFFFFF)
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	CP_UTF8                            = 65001
)

// initConsole initializes Windows console for UTF-8 and virtual terminal processing
func initConsole() {
	if runtime.GOOS != "windows" {
		return
	}

	// Set console code page to UTF-8 (65001)
	procSetConsoleOutputCP.Call(CP_UTF8)
	procSetConsoleCP.Call(CP_UTF8)

	// Enable virtual terminal processing for ANSI escape sequences
	stdoutHandle, _, _ := procGetStdHandle.Call(STD_OUTPUT_HANDLE)
	if stdoutHandle != 0 {
		var mode uint32
		procGetConsoleMode.Call(stdoutHandle, uintptr(unsafe.Pointer(&mode)))
		procSetConsoleMode.Call(stdoutHandle, uintptr(mode|ENABLE_VIRTUAL_TERMINAL_PROCESSING))
	}
}

var (
	// updateAvailable holds the latest version if an update is available
	updateAvailable   string
	updateAvailableMu sync.RWMutex
)

// checkUpdate checks for updates in the background
func checkUpdate() {
	checker := update.NewChecker(update.Version)
	release, err := checker.Check()
	if err != nil || release == nil {
		return
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if update.Version != "dev" && update.Version < latest {
		updateAvailableMu.Lock()
		updateAvailable = latest
		updateAvailableMu.Unlock()
	}
}

func main() {
	// Initialize Windows console for UTF-8 and ANSI support
	initConsole()

	// Check for updates in background
	go checkUpdate()

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

	// Build content map with combined values
	contentMap := buildContentMap(contentMgr, &input, summary)

	// === Layer 2: Layout ===
	gridLayout := layout.DefaultLayout()
	grid := layout.NewGrid(gridLayout, contentMap)

	// === Layer 3: Render ===
	tableRenderer := render.NewTableRenderer(grid)

	// Check if single-line mode is enabled
	singleLine := os.Getenv("STATUSLINE_SINGLELINE") == "1"

	var lines []string
	if singleLine {
		lines = []string{tableRenderer.RenderSingleLine()}
	} else {
		lines = tableRenderer.Render()
	}

	// Add update indicator if available
	updateAvailableMu.RLock()
	latest := updateAvailable
	updateAvailableMu.RUnlock()
	if latest != "" {
		lines = append(lines, fmt.Sprintf("↑ Update available: v%s", latest))
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

// buildContentMap builds the content map with combined values
// This combines related content types (e.g., model + token bar + token info)
func buildContentMap(mgr *content.Manager, input *content.StatusLineInput, summary *content.TranscriptSummary) layout.CellContent {
	// Get individual content pieces
	folder, _ := mgr.Get(content.ContentFolder, input, summary)
	model, _ := mgr.Get(content.ContentModel, input, summary)
	tokenBar, _ := mgr.Get(content.ContentTokenBar, input, summary)
	tokenInfo, _ := mgr.Get(content.ContentTokenInfo, input, summary)
	version, _ := mgr.Get(content.ContentClaudeVersion, input, summary)

	gitBranch, _ := mgr.Get(content.ContentGitBranch, input, summary)
	gitStatus, _ := mgr.Get(content.ContentGitStatus, input, summary)
	gitRemote, _ := mgr.Get(content.ContentGitRemote, input, summary)
	memoryFiles, _ := mgr.Get(content.ContentMemoryFiles, input, summary)

	agent, _ := mgr.Get(content.ContentAgent, input, summary)
	todo, _ := mgr.Get(content.ContentTodo, input, summary)
	tools, _ := mgr.Get(content.ContentTools, input, summary)
	sessionDuration, _ := mgr.Get(content.ContentSessionDuration, input, summary)

	currentTime, _ := mgr.Get(content.ContentCurrentTime, input, summary)
	quota, _ := mgr.Get(content.ContentQuota, input, summary)

	// Build content map with combined values
	result := make(layout.CellContent)

	// Folder
	if folder != "" {
		result["folder"] = "📁 " + folder
	}

	// Model + Token Bar + Token Info (combined in column 2)
	modelLine := model
	if modelLine == "" {
		modelLine = "Claude"
	}
	if tokenBar != "" {
		modelLine += " " + tokenBar
	}
	if tokenInfo != "" {
		modelLine += " " + tokenInfo
	}
	result["model"] = fmt.Sprintf("[%s]", modelLine)

	// Version
	if version != "" {
		result["claude-version"] = "v" + version
	}

	// Git Branch + Status + Remote (combined in column 1, row 1)
	gitLine := ""
	if gitBranch != "" {
		gitLine = fmt.Sprintf("🌿 %s", gitBranch)
	}
	if gitStatus != "" {
		if gitLine != "" {
			gitLine += " " + gitStatus
		} else {
			gitLine = gitStatus
		}
	}
	if gitRemote != "" {
		if gitLine != "" {
			gitLine += " " + gitRemote
		} else {
			gitLine = gitRemote
		}
	}
	result["git-branch"] = gitLine
	result["git-status"] = "" // Already combined above
	result["git-remote"] = "" // Already combined above

	// Memory files
	if memoryFiles != "" {
		result["memory-files"] = memoryFiles
	}

	// Current time + Quota (combined in column 2, row 1)
	timeLine := currentTime
	if quota != "" {
		if timeLine != "" {
			timeLine += " | " + quota
		} else {
			timeLine = quota
		}
	}
	result["current-time"] = timeLine

	// Agent
	if agent != "" {
		result["agent"] = agent
	}

	// TODO
	if todo != "" {
		result["todo"] = todo
	}

	// Tools
	if tools != "" {
		result["tools"] = tools
	}

	// Session duration
	if sessionDuration != "" {
		result["session-duration"] = sessionDuration
	}

	return result
}
