package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TranscriptEntry represents a single entry in the transcript JSONL file
type TranscriptEntry struct {
	Type      string            `json:"type"`
	Message   *MessageContent   `json:"message,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
	GitBranch string            `json:"git_branch,omitempty"`
}

// MessageContent represents the message content in a transcript entry
type MessageContent struct {
	Model   string        `json:"model,omitempty"`
	Content []ContentItem `json:"content,omitempty"`
	Usage   TokenUsage    `json:"usage,omitempty"`
}

// ContentItem represents a single content item (text or tool_use)
type ContentItem struct {
	Type  string                 `json:"type"`
	Name  string                 `json:"name,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// TranscriptSummary contains parsed information from the transcript
type TranscriptSummary struct {
	GitBranch       string
	GitStatus       string
	ActiveTools     []string
	CompletedTools  map[string]int
	Agents          []AgentInfo
	TodoTotal       int
	TodoCompleted   int
	SessionStart    time.Time
	SessionEnd      time.Time
	TotalTokens     int
	InputTokens     int
	OutputTokens    int
	CacheTokens     int
}

// AgentInfo represents information about a running agent
type AgentInfo struct {
	Type      string
	Desc      string
	StartTime time.Time
	EndTime   time.Time
	Elapsed   int
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	CacheReadInputTokens  int `json:"cache_read_input_tokens"`
}

// ParseTranscriptLastNLines reads and parses the last N lines of a transcript file
func ParseTranscriptLastNLines(transcriptPath string, n int) (*TranscriptSummary, error) {
	return ParseTranscriptLastNLinesWithProjectPath(transcriptPath, n, "")
}

// ParseTranscriptLastNLinesWithProjectPath parses transcript with optional project path for git detection
// If projectPath is provided and no git branch is found in transcript, it will use git commands
func ParseTranscriptLastNLinesWithProjectPath(transcriptPath string, n int, projectPath string) (*TranscriptSummary, error) {
	if transcriptPath == "" {
		return &TranscriptSummary{}, nil
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return &TranscriptSummary{}, nil
	}
	defer file.Close()

	// Get file size for seeking
	stat, err := file.Stat()
	if err != nil {
		return &TranscriptSummary{}, nil
	}

	// Seek to end - 64KB (enough for ~100 lines of JSON)
	offset := stat.Size() - 65536
	if offset < 0 {
		offset = 0
	}
	file.Seek(offset, io.SeekStart)

	// Read line by line
	scanner := bufio.NewScanner(file)
	var entries []TranscriptEntry

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry TranscriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		entries = append(entries, entry)
		// Keep only the last N entries
		if len(entries) > n {
			entries = entries[len(entries)-n:]
		}
	}

	summary := analyzeTranscriptEntries(entries)

	// If no git branch found in transcript and project path is provided, try git commands
	if summary.GitBranch == "" && projectPath != "" {
		summary.GitBranch = getGitBranchForPath(projectPath)
	}

	return summary, nil
}

// analyzeTranscriptEntries extracts useful information from transcript entries
func analyzeTranscriptEntries(entries []TranscriptEntry) *TranscriptSummary {
	summary := &TranscriptSummary{
		CompletedTools: make(map[string]int),
		Agents:         []AgentInfo{},
	}

	if len(entries) == 0 {
		return summary
	}

	// Track tool calls for completion counting
	allTools := make(map[string]int)
	pendingTools := make(map[string]bool)

	// Process in forward order
	for _, entry := range entries {
		// Extract git branch from most recent entry
		if summary.GitBranch == "" && entry.GitBranch != "" {
			summary.GitBranch = entry.GitBranch
		}

		// Extract timestamp for session duration
		if entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				if summary.SessionStart.IsZero() || t.Before(summary.SessionStart) {
					summary.SessionStart = t
				}
				if summary.SessionEnd.IsZero() || t.After(summary.SessionEnd) {
					summary.SessionEnd = t
				}
			}
		}

		// Parse tool usage and agent activity
		if entry.Type == "assistant" && entry.Message != nil {
			// Accumulate tokens
			summary.InputTokens += entry.Message.Usage.InputTokens
			summary.OutputTokens += entry.Message.Usage.OutputTokens
			summary.CacheTokens += entry.Message.Usage.CacheReadInputTokens
			summary.TotalTokens = summary.InputTokens + summary.OutputTokens

			for _, content := range entry.Message.Content {
				if content.Type == "tool_use" {
					toolName := content.Name

					// Check if this is a Task (agent) call
					if toolName == "Task" {
						agentType := "general-purpose"
						if subagentType, ok := content.Input["subagent_type"].(string); ok {
							agentType = subagentType
						}

						agentDesc := ""
						if desc, ok := content.Input["description"].(string); ok {
							agentDesc = desc
						}

						// Directly add agent to summary
						summary.Agents = append(summary.Agents, AgentInfo{
							Type: agentType,
							Desc: agentDesc,
						})
					} else if toolName == "TodoWrite" {
						extractTodoInfo(content.Input, summary)
					} else if toolName != "" {
						allTools[toolName]++
						pendingTools[toolName] = true

						// Track active tools (avoid duplicates)
						isDuplicate := false
						for _, t := range summary.ActiveTools {
							if t == toolName {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							summary.ActiveTools = append(summary.ActiveTools, toolName)
						}
					}
				}
			}
		}

		// Track completed tools
		if entry.Type == "tool_result" && entry.Message != nil {
			for _, content := range entry.Message.Content {
				if content.Type == "tool_result" && content.ID != "" {
					delete(pendingTools, content.Name)
					break
				}
			}
		}
	}

	// All tools counted in allTools are completed tools
	for toolName, count := range allTools {
		summary.CompletedTools[toolName] = count
	}

	// Clear ActiveTools - only show tools that are still pending
	summary.ActiveTools = nil
	for toolName := range pendingTools {
		summary.ActiveTools = append(summary.ActiveTools, toolName)
	}

	return summary
}

// extractTodoInfo extracts TODO information from a TodoWrite tool call
func extractTodoInfo(input map[string]interface{}, summary *TranscriptSummary) {
	todosInterface, ok := input["todos"]
	if !ok {
		return
	}

	todosList, ok := todosInterface.([]interface{})
	if !ok {
		return
	}

	completed := 0
	total := len(todosList)

	for _, todoInterface := range todosList {
		todo, ok := todoInterface.(map[string]interface{})
		if !ok {
			continue
		}

		status, ok := todo["status"].(string)
		if !ok {
			continue
		}

		if status == "completed" {
			completed++
		}
	}

	summary.TodoCompleted = completed
	summary.TodoTotal = total
}

// GetSessionDuration formats the session duration
func GetSessionDuration(summary *TranscriptSummary) string {
	if summary.SessionStart.IsZero() || summary.SessionEnd.IsZero() {
		return ""
	}

	duration := summary.SessionEnd.Sub(summary.SessionStart)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else {
		hours := int(duration.Hours())
		mins := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
}

// FormatActiveTools creates a compact string of active tools
func FormatActiveTools(summary *TranscriptSummary) string {
	if len(summary.ActiveTools) == 0 {
		return ""
	}

	count := len(summary.ActiveTools)
	if count > 3 {
		return fmt.Sprintf("%d tools", count)
	}

	toolNames := make([]string, 0, len(summary.ActiveTools))
	for _, tool := range summary.ActiveTools {
		shortName := tool
		if strings.HasPrefix(tool, "mcp__") {
			shortName = "mcp:" + tool[5:]
			if len(shortName) > 15 {
				shortName = shortName[:12] + ".."
			}
		}
		toolNames = append(toolNames, shortName)
	}

	return strings.Join(toolNames, ",")
}

// FormatTodoProgress creates a compact TODO progress string
func FormatTodoProgress(summary *TranscriptSummary) string {
	if summary.TodoTotal == 0 {
		return ""
	}

	if summary.TodoCompleted == summary.TodoTotal {
		return fmt.Sprintf("✓ %d/%d", summary.TodoCompleted, summary.TodoTotal)
	}

	return fmt.Sprintf("%d/%d", summary.TodoCompleted, summary.TodoTotal)
}

// FormatAgentInfo creates a compact agent info string
func FormatAgentInfo(summary *TranscriptSummary) string {
	if len(summary.Agents) == 0 {
		return ""
	}

	agent := summary.Agents[len(summary.Agents)-1]
	info := agent.Type

	if agent.Desc != "" {
		desc := agent.Desc
		if len(desc) > 20 {
			desc = desc[:17] + ".."
		}
		info = fmt.Sprintf("%s: %s", info, desc)
	}

	// Add elapsed time if available
	if agent.Elapsed > 0 && agent.Elapsed < 3600 {
		info += fmt.Sprintf(" (%s)", formatElapsedDuration(time.Duration(agent.Elapsed)*time.Second))
	}

	return info
}

// formatElapsedDuration formats a duration in a compact way
func formatElapsedDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	} else if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// FormatCompletedTools creates a string showing completed tools with counts
func FormatCompletedTools(summary *TranscriptSummary) string {
	if len(summary.CompletedTools) == 0 {
		return ""
	}

	total := 0
	for _, count := range summary.CompletedTools {
		total += count
	}

	return fmt.Sprintf("%d tools", total)
}

// GetProjectName extracts the project directory name for display
func GetProjectName(cwd string, projectDir string) string {
	dir := cwd
	if projectDir != "" {
		dir = projectDir
	}

	parts := strings.Split(filepath.ToSlash(dir), "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if len(name) > 20 {
			return name[:17] + ".."
		}
		return name
	}

	return "project"
}

// getGitBranchForPath reads the current git branch using git command for a given path
// Tries multiple methods to handle edge cases like:
// - Freshly initialized repo (no commits yet)
// - Detached HEAD state
// - Different git versions
func getGitBranchForPath(path string) string {
	if path == "" {
		return ""
	}

	// Method 1: Try git symbolic-ref --short HEAD (most reliable for active branches)
	// This works for normal branch checkouts and shows the branch name
	// even if there are no commits yet
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = path
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
	cmd.Dir = path
	output, err = cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		// For freshly initialized repos with no commits, show "(empty)"
		// For detached HEAD, show the commit abbreviation
		if branch == "" || branch == "HEAD" {
			// Check if this is a fresh repo (exists but no commits)
			cmd = exec.Command("git", "status", "--porcelain")
			cmd.Dir = path
			_, err = cmd.Output()
			if err == nil {
				// Git repo exists but might be empty
				// Try to get the default branch name
				cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD")
				cmd.Dir = path
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
