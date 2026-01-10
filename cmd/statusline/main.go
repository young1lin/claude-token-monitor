package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/young1lin/claude-token-monitor/internal/parser"
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
	STD_OUTPUT_HANDLE     = uintptr(-11 & 0xFFFFFFFF)
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	CP_UTF8               = 65001
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

	// Memory files cache
	memoryFilesCacheMu   sync.RWMutex
	memoryFilesCache     *MemoryFilesInfo
	memoryFilesCacheTime time.Time
	memoryFilesCacheTTL  = 60 * time.Second // 60秒缓存

	// Usage cache
	usageCache     *UsageData
	usageCacheMu   sync.RWMutex
	usageCacheTime time.Time
	usageCacheTTL  = 5 * time.Minute // 5分钟缓存，避免频繁API调用
)

// CredentialsFile represents the Claude credentials file
type CredentialsFile struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
		ExpiresAt        int64  `json:"expiresAt"`
	} `json:"claudeAiOauth"`
}

// UsageApiResponse represents the OAuth usage API response
type UsageApiResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

// UsageData holds parsed usage information
type UsageData struct {
	FiveHour     float64
	SevenDay     float64
	FiveHourResetAt time.Time
	SevenDayResetAt time.Time
	APIUnavailable bool
}

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

// StatusLineInput is the input JSON from Claude Code
type StatusLineInput struct {
	Model struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	} `json:"model"`
	ContextWindow struct {
		TotalInputTokens  int `json:"total_input_tokens"`
		TotalOutputTokens int `json:"total_output_tokens"`
		ContextWindowSize int `json:"context_window_size"`
		CurrentUsage      struct {
			InputTokens            int `json:"input_tokens"`
			OutputTokens           int `json:"output_tokens"`
			CacheReadInputTokens   int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Workspace      struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
}

// MemoryFilesInfo stores memory files statistics
type MemoryFilesInfo struct {
	HasCLAUDEMd bool
	RulesCount  int
	MCPCount    int
	HooksCount  int
}

func main() {
	// Initialize Windows console for UTF-8 and ANSI support
	initConsole()

	// Check for updates in background
	go checkUpdate()

	// Initialize rate limit tracker
	tracker := NewSimpleTracker()
	_ = tracker.Load() // Ignore errors on first run

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
	var input StatusLineInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		fmt.Fprintf(os.Stderr, "JSON parse error: %v\n", err)
		return
	}

	// Record this request in rate limit tracker
	tokensThisRequest := input.ContextWindow.CurrentUsage.InputTokens +
		input.ContextWindow.CurrentUsage.OutputTokens
	_ = tracker.RecordRequest(tokensThisRequest)

	// Parse transcript if available
	var summary *parser.TranscriptSummary
	if input.TranscriptPath != "" {
		summary, _ = parser.ParseTranscriptLastNLines(input.TranscriptPath, 100)
	} else {
		summary = &parser.TranscriptSummary{}
	}

	// Format and print output
	lines := formatOutput(&input, summary, tracker)

	// Check if single-line mode is enabled (default is multi-line now)
	singleLine := os.Getenv("STATUSLINE_SINGLELINE") == "1"

	if singleLine {
		// Single-line mode: join all lines with " | "
		fmt.Println(strings.Join(lines, " | "))
	} else {
		// Multi-line mode (default): print each line separately
		for _, line := range lines {
			fmt.Println(line)
		}
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

func formatOutput(input *StatusLineInput, summary *parser.TranscriptSummary, tracker *SimpleTracker) []string {
	var lines []string

	// Line 1: Project name + Model + Progress bar + Token + Rate limit
	line1Parts := []string{}

	projectName := getProjectName(input.Cwd)
	if projectName != "" {
		line1Parts = append(line1Parts, fmt.Sprintf("📁 %s", projectName))
	}

	modelName := input.Model.DisplayName
	if modelName == "" {
		modelName = "Claude"
	}
	line1Parts = append(line1Parts, fmt.Sprintf("[%s]", modelName))

	// Progress bar - calculate actual context tokens
	tokens := input.ContextWindow.CurrentUsage.InputTokens +
		input.ContextWindow.CurrentUsage.CacheReadInputTokens +
		input.ContextWindow.CurrentUsage.OutputTokens
	maxTokens := input.ContextWindow.ContextWindowSize
	if maxTokens == 0 {
		maxTokens = 200000
	}
	pct := float64(tokens) / float64(maxTokens) * 100

	barWidth := 10
	fillWidth := int(pct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}
	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)

	// Add colors based on percentage
	var colorCode, resetCode string
	resetCode = "\x1b[0m"
	if pct >= 60 {
		colorCode = "\x1b[1;31m" // Bold red
	} else if pct >= 40 {
		colorCode = "\x1b[1;33m" // Bold yellow
	} else if pct >= 20 {
		colorCode = "\x1b[1;36m" // Bold cyan
	} else {
		colorCode = "\x1b[1;32m" // Bold green
	}

	progressBar := fmt.Sprintf("[%s%s%s%s]", colorCode, filled, resetCode, empty)
	tokenInfo := fmt.Sprintf("%s/%dK (%.1f%%)", formatNumber(tokens), maxTokens/1000, pct)
	line1Parts = append(line1Parts, progressBar+" "+tokenInfo)

	// Rate limit status on line 1
	reqRem, reqLimit, tokRem, tokLimit := tracker.GetRateLimitStatus()
	reqPct := float64(reqLimit-reqRem) / float64(reqLimit) * 100
	tokPct := float64(tokLimit-tokRem) / float64(tokLimit) * 100

	if reqRem < reqLimit || tokRem < tokLimit {
		var rateLimitColor string
		if reqPct >= 50 || tokPct >= 50 {
			rateLimitColor = "\x1b[1;31m" // Red
		} else if reqPct >= 25 || tokPct >= 25 {
			rateLimitColor = "\x1b[1;33m" // Yellow
		} else {
			rateLimitColor = "\x1b[1;36m" // Cyan
		}
		line1Parts = append(line1Parts, fmt.Sprintf("%s⚡ %d/%d req %d%%|%.0fK tk%s", rateLimitColor, reqRem, reqLimit, int(reqPct), float64(tokRem)/1000, resetCode))
	}

	lines = append(lines, strings.Join(line1Parts, " | "))

	// Line 2: Git branch + Memory files
	line2Parts := []string{}

	gitBranch := getGitBranch(input.Cwd)
	if gitBranch != "" {
		added, deleted, modified := getGitStatus(input.Cwd)
		if added > 0 || deleted > 0 || modified > 0 {
			var statusParts []string
			if added > 0 {
				statusParts = append(statusParts, fmt.Sprintf("+%d", added))
			}
			if modified > 0 {
				statusParts = append(statusParts, fmt.Sprintf("~%d", modified))
			}
			if deleted > 0 {
				statusParts = append(statusParts, fmt.Sprintf("-%d", deleted))
			}
			gitInfo := gitBranch + " " + strings.Join(statusParts, " ")
			line2Parts = append(line2Parts, fmt.Sprintf("🌿 %s", gitInfo))
		} else {
			line2Parts = append(line2Parts, fmt.Sprintf("🌿 %s", gitBranch))
		}
	}

	// Memory files info
	memoryInfo := getMemoryFilesInfoCached(input.Cwd)
	memoryDisplay := formatMemoryFilesDisplay(memoryInfo)
	if memoryDisplay != "" {
		line2Parts = append(line2Parts, memoryDisplay)
	}

	if len(line2Parts) > 0 {
		lines = append(lines, strings.Join(line2Parts, " | "))
	}

	// Line 3: Agent + Tools + TODO
	line3Parts := []string{}

	// Tools
	if len(summary.CompletedTools) > 0 {
		total := 0
		for _, count := range summary.CompletedTools {
			total += count
		}
		line3Parts = append(line3Parts, fmt.Sprintf("🔧 %d tools", total))
	}

	// Agent
	if len(summary.Agents) > 0 {
		agent := summary.Agents[len(summary.Agents)-1]
		agentInfo := agent.Type
		if agent.Desc != "" {
			desc := agent.Desc
			if len(desc) > 20 {
				desc = desc[:17] + ".."
			}
			agentInfo = fmt.Sprintf("%s: %s", agentInfo, desc)
		}
		line3Parts = append(line3Parts, fmt.Sprintf("🤖 %s", agentInfo))
	}

	if summary.TodoTotal > 0 {
		if summary.TodoCompleted == summary.TodoTotal {
			line3Parts = append(line3Parts, fmt.Sprintf("📋 ✓ %d/%d", summary.TodoCompleted, summary.TodoTotal))
		} else {
			line3Parts = append(line3Parts, fmt.Sprintf("📋 %d/%d", summary.TodoCompleted, summary.TodoTotal))
		}
	}

	if len(line3Parts) > 0 {
		lines = append(lines, strings.Join(line3Parts, " | "))
	}

	// Line 5: Current time + Subscription quota + Duration
	line5Parts := []string{}

	// Current time (year-month-day hour:minute)
	line5Parts = append(line5Parts, fmt.Sprintf("🕐 %s", time.Now().Format("2006-01-02 15:04")))

	// Subscription quota (for Pro/Max users, not API users)
	if quota := getSubscriptionQuota(input); quota != "" {
		line5Parts = append(line5Parts, quota)
	}

	// Session duration
	if !summary.SessionStart.IsZero() {
		var duration time.Duration
		if !summary.SessionEnd.IsZero() {
			duration = summary.SessionEnd.Sub(summary.SessionStart)
		} else {
			duration = time.Since(summary.SessionStart)
		}
		line5Parts = append(line5Parts, fmt.Sprintf("⏱️ %s", formatDuration(duration)))
	}

	if len(line5Parts) > 0 {
		lines = append(lines, strings.Join(line5Parts, " | "))
	}

	// Update indicator (separate line if present)
	updateAvailableMu.RLock()
	latest := updateAvailable
	updateAvailableMu.RUnlock()
	if latest != "" {
		lines = append(lines, fmt.Sprintf("↑ Update available: v%s", latest))
	}

	return lines
}

func formatNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
}

// getMCPCount reads and parses MCP servers configuration
func getMCPCount(cwd string) int {
	count := 0

	// Method 1: Check .claude/mcp_servers.json
	mcpPath := filepath.Join(cwd, ".claude", "mcp_servers.json")
	if data, err := os.ReadFile(mcpPath); err == nil {
		var mcpServers map[string]interface{}
		if err := json.Unmarshal(data, &mcpServers); err == nil {
			if servers, ok := mcpServers["mcpServers"].([]interface{}); ok {
				count = len(servers)
			} else if len(mcpServers) > 0 {
				count = len(mcpServers)
			}
		}
	}

	// Method 2: Check ~/.claude/settings.json for mcpServers
	if count == 0 {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
			if data, err := os.ReadFile(settingsPath); err == nil {
				var settings map[string]interface{}
				if err := json.Unmarshal(data, &settings); err == nil {
					if mcpServers, ok := settings["mcpServers"].(map[string]interface{}); ok {
						count = len(mcpServers)
					}
				}
			}
		}
	}

	return count
}

// getSubscriptionQuota returns the subscription quota usage percentage with reset time
func getSubscriptionQuota(input *StatusLineInput) string {
	usage := getSubscriptionUsage()

	// API user or no data available
	if usage == nil {
		return ""
	}

	// Show 5-hour usage (more relevant for daily usage)
	if usage.FiveHour > 0 {
		resetTime := formatResetTime(usage.FiveHourResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · 重置 %s", usage.FiveHour, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.FiveHour)
	}

	// Fallback to 7-day usage
	if usage.SevenDay > 0 {
		resetTime := formatResetTime(usage.SevenDayResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · 重置 %s", usage.SevenDay, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.SevenDay)
	}

	return ""
}

// formatResetTime formats the reset time in local timezone with timezone name (e.g., "19:00 (Asia/Shanghai)")
func formatResetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Convert to local timezone
	local := t.Local()
	timeStr := local.Format("15:04") // HH:MM format in 24-hour

	// Try to get IANA timezone name
	zoneName := getLocalTimeZoneName()

	return fmt.Sprintf("%s (%s)", timeStr, zoneName)
}

// getLocalTimeZoneName attempts to get the IANA timezone name (e.g., "Asia/Shanghai")
func getLocalTimeZoneName() string {
	// Method 1: Check TZ environment variable (Linux/Mac/Unix)
	if tz := os.Getenv("TZ"); tz != "" {
		// TZ might be like "Asia/Shanghai" or ":Asia/Shanghai"
		return strings.TrimPrefix(tz, ":")
	}

	// Method 2: Try to read from /etc/localtime symlink (Linux/Mac)
	// The symlink usually points to /usr/share/zoneinfo/Area/Location
	if linkTarget, err := os.Readlink("/etc/localtime"); err == nil {
		// Extract timezone from path like "/usr/share/zoneinfo/Asia/Shanghai"
		if idx := strings.LastIndex(linkTarget, "zoneinfo/"); idx >= 0 {
			return linkTarget[idx+9:] // Skip "zoneinfo/"
		}
	}

	// Method 3: Fallback to UTC offset format
	_, zoneOffset := time.Now().Zone()
	if zoneOffset == 0 {
		return "UTC"
	}

	sign := "+"
	if zoneOffset < 0 {
		sign = "-"
		zoneOffset = -zoneOffset
	}
	zoneHours := zoneOffset / 3600
	zoneMinutes := (zoneOffset % 3600) / 60

	if zoneMinutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, zoneHours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, zoneHours, zoneMinutes)
}

// getSubscriptionUsage fetches subscription usage from Claude OAuth API
// Returns nil for API users (no OAuth credentials) or on error
func getSubscriptionUsage() *UsageData {
	now := time.Now()

	// Check cache
	usageCacheMu.RLock()
	if usageCache != nil && now.Sub(usageCacheTime) < usageCacheTTL {
		cached := *usageCache
		usageCacheMu.RUnlock()
		return &cached
	}
	usageCacheMu.RUnlock()

	// Read credentials from ~/.claude/.credentials.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		return nil // No credentials file = API user
	}

	var creds CredentialsFile
	if err := json.Unmarshal(credData, &creds); err != nil {
		return nil
	}

	if creds.ClaudeAiOauth == nil || creds.ClaudeAiOauth.AccessToken == "" {
		return nil // API user
	}

	// Check if token is expired
	if creds.ClaudeAiOauth.ExpiresAt > 0 && creds.ClaudeAiOauth.ExpiresAt < now.UnixMilli() {
		return nil // Token expired
	}

	// Check subscription type - skip for API users
	subType := strings.ToLower(creds.ClaudeAiOauth.SubscriptionType)
	if subType == "" || strings.Contains(subType, "api") {
		return nil
	}

	// Fetch usage from API
	usage, err := fetchUsageAPI(creds.ClaudeAiOauth.AccessToken)
	if err != nil || usage == nil {
		// Cache the failure to prevent retry storms
		usageCacheMu.Lock()
		usageCache = &UsageData{APIUnavailable: true}
		usageCacheTime = now
		usageCacheMu.Unlock()
		return nil
	}

	// Update cache
	usageCacheMu.Lock()
	usageCache = usage
	usageCacheTime = now
	usageCacheMu.Unlock()

	return usage
}

// fetchUsageAPI calls the Claude OAuth usage API
func fetchUsageAPI(accessToken string) (*UsageData, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-token-monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp UsageApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	usage := &UsageData{}

	// Parse 5-hour usage
	if apiResp.FiveHour != nil {
		usage.FiveHour = apiResp.FiveHour.Utilization
		if apiResp.FiveHour.ResetsAt != "" {
			usage.FiveHourResetAt, _ = time.Parse(time.RFC3339, apiResp.FiveHour.ResetsAt)
		}
	}

	// Parse 7-day usage
	if apiResp.SevenDay != nil {
		usage.SevenDay = apiResp.SevenDay.Utilization
		if apiResp.SevenDay.ResetsAt != "" {
			usage.SevenDayResetAt, _ = time.Parse(time.RFC3339, apiResp.SevenDay.ResetsAt)
		}
	}

	return usage, nil
}

// getGitBranch reads the current git branch using git command
// Tries multiple methods to handle edge cases like:
// - Freshly initialized repo (no commits yet)
// - Detached HEAD state
// - Different git versions
func getGitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}

	// Method 1: Try git symbolic-ref --short HEAD (most reliable for active branches)
	// This works for normal branch checkouts and shows the branch name
	// even if there are no commits yet
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}

	// Method 2: Try git rev-parse --abbrev-ref HEAD (fallback)
	// This returns "HEAD" for detached HEAD state or fresh repos
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	output, err = cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		// For freshly initialized repos with no commits, show "(no commits)"
		// For detached HEAD, show the commit abbreviation
		if branch == "" || branch == "HEAD" {
			// Check if this is a fresh repo (exists but no commits)
			cmd = exec.Command("git", "status", "--porcelain")
			cmd.Dir = cwd
			_, err = cmd.Output()
			if err == nil {
				// Git repo exists but might be empty
				// Try to get the default branch name
				cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD")
				cmd.Dir = cwd
				output, err = cmd.Output()
				if err == nil {
					remoteBranch := strings.TrimSpace(string(output))
					if strings.HasPrefix(remoteBranch, "origin/") {
						return strings.TrimPrefix(remoteBranch, "origin/")
					}
				}
				// Show a hint for empty repo
				return "(empty)"
			}
			return ""
		}
		return branch
	}

	return ""
}

// getGitStatus returns added, deleted, modified file counts
func getGitStatus(cwd string) (int, int, int) {
	if cwd == "" {
		return 0, 0, 0
	}

	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}

	lines := strings.Split(string(output), "\n")
	added, deleted, modified := 0, 0, 0

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		xy := line[:2]
		x := xy[0]
		y := xy[1]

		// ?? = untracked (added)
		if x == '?' && y == '?' {
			added++
			continue
		}

		// Check staged changes (x)
		switch x {
		case 'A': // added
			added++
		case 'M': // modified
			modified++
		case 'D': // deleted
			deleted++
		}

		// Check unstaged changes (y), but don't double count
		if x == ' ' {
			switch y {
			case 'M': // modified
				modified++
			case 'D': // deleted
				deleted++
			}
		}
	}

	return added, deleted, modified
}

// getProjectName extracts the project folder name
func getProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}

	// Get the last part of the path
	parts := strings.Split(cwd, "\\")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if len(name) > 15 {
			return name[:12] + ".."
		}
		return name
	}

	parts = strings.Split(cwd, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if len(name) > 15 {
			return name[:12] + ".."
		}
		return name
	}

	return ""
}

// getMemoryFilesInfo scans .claude directory for configuration files
func getMemoryFilesInfo(cwd string) MemoryFilesInfo {
	info := MemoryFilesInfo{}
	claudeDir := filepath.Join(cwd, ".claude")

	// Check .claude/CLAUDE.md
	claudeMdPath := filepath.Join(claudeDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); err == nil {
		info.HasCLAUDEMd = true
	}

	// Scan .claude/rules/ directory
	rulesDir := filepath.Join(claudeDir, "rules")
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				info.RulesCount++
			}
		}
	}

	// Get MCP count
	info.MCPCount = getMCPCount(cwd)

	return info
}

// getMemoryFilesInfoCached returns cached memory files info with 60s TTL
func getMemoryFilesInfoCached(cwd string) MemoryFilesInfo {
	now := time.Now()

	// Try to read from cache
	memoryFilesCacheMu.RLock()
	if memoryFilesCache != nil && now.Sub(memoryFilesCacheTime) < memoryFilesCacheTTL {
		cached := *memoryFilesCache
		memoryFilesCacheMu.RUnlock()
		return cached
	}
	memoryFilesCacheMu.RUnlock()

	// Cache expired or doesn't exist, re-scan
	info := getMemoryFilesInfo(cwd)

	// Update cache
	memoryFilesCacheMu.Lock()
	memoryFilesCache = &info
	memoryFilesCacheTime = now
	memoryFilesCacheMu.Unlock()

	return info
}

// formatMemoryFilesDisplay formats memory files display text
func formatMemoryFilesDisplay(info MemoryFilesInfo) string {
	if !info.HasCLAUDEMd && info.RulesCount == 0 && info.MCPCount == 0 {
		return "" // No config files, don't display
	}

	parts := []string{}

	if info.HasCLAUDEMd {
		parts = append(parts, "CLAUDE.md")
	}

	if info.RulesCount > 0 {
		parts = append(parts, fmt.Sprintf("%d rules", info.RulesCount))
	}

	if info.MCPCount > 0 {
		parts = append(parts, fmt.Sprintf("%d MCPs", info.MCPCount))
	}

	return "📦 " + strings.Join(parts, " + ")
}
