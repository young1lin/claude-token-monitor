package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// TranscriptEntry represents a single entry in the transcript JSONL file
type TranscriptEntry struct {
	Type      string          `json:"type"`
	Message   *MessageContent `json:"message,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	GitBranch string          `json:"git_branch,omitempty"`
}

// MessageContent represents the message content in a transcript entry.
// The "content" field in Claude Code's JSONL is polymorphic:
//   - string  → user typed a text message  (e.g. "你确定吗？")
//   - array   → tool_use / tool_result items
//
// We keep it as json.RawMessage so both cases are preserved after unmarshaling.
type MessageContent struct {
	Model   string          `json:"model,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	Usage   TokenUsage      `json:"usage,omitempty"`
}

// isTextContent reports whether the message content is a plain string
// (i.e. the user typed a text message, as opposed to a tool result batch).
func (m *MessageContent) isTextContent() bool {
	return len(m.Content) > 0 && m.Content[0] == '"'
}

// contentItems parses and returns the content array for tool_use / tool_result
// messages. Returns nil when content is a string (user text message).
func (m *MessageContent) contentItems() []ContentItem {
	if len(m.Content) == 0 || m.Content[0] != '[' {
		return nil
	}
	var items []ContentItem
	if json.Unmarshal(m.Content, &items) != nil {
		return nil
	}
	return items
}

// ContentItem represents a single content item (text or tool_use)
type ContentItem struct {
	Type      string                 `json:"type"`
	Name      string                 `json:"name,omitempty"`
	ID        string                 `json:"id,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	IsError   bool                   `json:"is_error,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
}

// TranscriptSummary contains parsed information from the transcript
type TranscriptSummary struct {
	GitBranch      string
	GitStatus      string
	ActiveTools    []string
	CompletedTools map[string]int
	FailedTools    map[string]int
	Agents         []AgentInfo
	TodoTotal      int
	TodoCompleted  int
	SessionStart   time.Time
	SessionEnd     time.Time
	TotalTokens    int
	InputTokens    int
	OutputTokens   int
	CacheTokens    int
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
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

// In-memory cache — only useful when the same process calls this function
// multiple times (e.g. several content collectors within one invocation).
var (
	transcriptCache          *TranscriptSummary
	transcriptCachePath      string
	transcriptCacheMu        sync.RWMutex
	transcriptCacheMtime     time.Time // file mtime recorded at last parse
	transcriptCacheParseTime time.Time // wall time of last parse (for TTL)
	transcriptCacheTTL       = 5 * time.Second
)

// ParseTranscriptLastNLines reads and parses the transcript file
func ParseTranscriptLastNLines(transcriptPath string, n int) (*TranscriptSummary, error) {
	return ParseTranscriptLastNLinesWithProjectPath(transcriptPath, n, "")
}

// ParseTranscriptLastNLinesWithProjectPath parses the transcript.
// Reads up to 512 KB from the end of the file to cover a full turn's entries
// even when tool results contain large file contents (e.g. the Read tool).
func ParseTranscriptLastNLinesWithProjectPath(transcriptPath string, _ int, projectPath string) (*TranscriptSummary, error) {
	if transcriptPath == "" {
		return &TranscriptSummary{}, nil
	}

	// Stat first — O(1), no file content read.
	info, err := os.Stat(transcriptPath)
	if err != nil {
		return &TranscriptSummary{}, nil
	}
	fileMtime := info.ModTime()
	now := time.Now()

	// In-memory cache: helps when multiple collectors call this within one invocation.
	transcriptCacheMu.RLock()
	if transcriptCache != nil && transcriptCachePath == transcriptPath &&
		transcriptCacheMtime.Equal(fileMtime) && now.Sub(transcriptCacheParseTime) < transcriptCacheTTL {
		cached := *transcriptCache
		transcriptCacheMu.RUnlock()
		return &cached, nil
	}
	transcriptCacheMu.RUnlock()

	file, err := os.Open(transcriptPath)
	if err != nil {
		return &TranscriptSummary{}, nil
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return &TranscriptSummary{}, nil
	}

	entries := readCurrentTurnEntries(file, stat.Size())
	summary := analyzeTranscriptEntries(entries)

	if summary.GitBranch == "" && projectPath != "" {
		summary.GitBranch = getGitBranchForPath(projectPath)
	}

	transcriptCacheMu.Lock()
	transcriptCache = summary
	transcriptCachePath = transcriptPath
	transcriptCacheMtime = fileMtime
	transcriptCacheParseTime = now
	transcriptCacheMu.Unlock()

	return summary, nil
}

// readCurrentTurnEntries reads up to 512 KB from the end of the file,
// scans backwards to locate the last real user message, then fully parses
// only the entries from that message to EOF.
//
// Large tool-result entries (e.g. the Read tool embeds the entire file content)
// that predate the current turn are skipped with a cheap string check, avoiding
// unnecessary JSON deserialization of hundreds of KB.
func readCurrentTurnEntries(f *os.File, fileSize int64) []TranscriptEntry {
	if fileSize == 0 {
		return nil
	}

	const readWindow = 512 * 1024
	offset := fileSize - readWindow
	if offset < 0 {
		offset = 0
	}

	buf := make([]byte, fileSize-offset)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil
	}

	// Collect non-empty lines (JSONL).
	rawLines := strings.Split(string(buf[:n]), "\n")
	var lines []string
	for _, l := range rawLines {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}

	// Scan backwards to find the last real user message.
	// Use a cheap contains check before full JSON parsing so that large
	// tool-result or assistant lines (which cannot be user messages) are
	// skipped without deserializing their contents.
	startIdx := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if !strings.Contains(lines[i], `"type":"user"`) {
			continue
		}
		var entry TranscriptEntry
		if json.Unmarshal([]byte(lines[i]), &entry) != nil {
			continue
		}
		if isRealUserMessage(entry) {
			startIdx = i
			break
		}
	}

	// Fully parse only the current-turn entries (user message → EOF).
	entries := make([]TranscriptEntry, 0, len(lines)-startIdx)
	for _, line := range lines[startIdx:] {
		var entry TranscriptEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

// isRealUserMessage returns true when the entry is a genuine user text message,
// as opposed to a tool_result submission (which also has type "user").
// isRealUserMessage returns true when the entry is a genuine user text message.
// In Claude Code's JSONL, user text messages have message.content as a JSON
// string, while tool_result submissions have it as an array.
func isRealUserMessage(entry TranscriptEntry) bool {
	return entry.Type == "user" && entry.Message != nil && entry.Message.isTextContent()
}

// analyzeTranscriptEntries extracts useful information from transcript entries.
// Tool call statistics (CompletedTools / FailedTools / ActiveTools) are scoped
// to the current turn — entries that appear after the last real user message.
// Session-level information (git branch, timestamps, agents, todos) is gathered
// from all entries.
func analyzeTranscriptEntries(entries []TranscriptEntry) *TranscriptSummary {
	summary := &TranscriptSummary{
		CompletedTools: make(map[string]int),
		FailedTools:    make(map[string]int),
		Agents:         []AgentInfo{},
	}

	if len(entries) == 0 {
		return summary
	}

	// Find the index of the last real user message.
	// Tool tracking starts from that point so we only show tools used in the
	// current turn (from the user's most recent prompt onward).
	lastUserMsgIdx := -1
	for i := len(entries) - 1; i >= 0; i-- {
		if isRealUserMessage(entries[i]) {
			lastUserMsgIdx = i
			break
		}
	}

	// toolIDToName maps tool_use ID -> tool name (scoped to current turn)
	toolIDToName := make(map[string]string)
	// pendingIDs tracks tool_use IDs that have not yet received a result
	pendingIDs := make(map[string]bool)

	// Process all entries in forward order
	for i, entry := range entries {
		// ── Session-level info (whole history) ──────────────────────────────

		if summary.GitBranch == "" && entry.GitBranch != "" {
			summary.GitBranch = entry.GitBranch
		}

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

		if entry.Type == "assistant" && entry.Message != nil {
			summary.InputTokens += entry.Message.Usage.InputTokens
			summary.OutputTokens += entry.Message.Usage.OutputTokens
			summary.CacheTokens += entry.Message.Usage.CacheReadInputTokens
			summary.TotalTokens = summary.InputTokens + summary.OutputTokens

			for _, content := range entry.Message.contentItems() {
				if content.Type == "tool_use" {
					// Agents and todos are tracked across the full session
					if content.Name == "Task" {
						agentType := "general-purpose"
						if subagentType, ok := content.Input["subagent_type"].(string); ok {
							agentType = subagentType
						}
						agentDesc := ""
						if desc, ok := content.Input["description"].(string); ok {
							agentDesc = desc
						}
						summary.Agents = append(summary.Agents, AgentInfo{
							Type: agentType,
							Desc: agentDesc,
						})
					} else if content.Name == "TodoWrite" {
						extractTodoInfo(content.Input, summary)
					}
				}
			}
		}

		// ── Tool call stats (current turn only) ─────────────────────────────
		// Skip entries before (and including) the last real user message.
		if i <= lastUserMsgIdx {
			continue
		}

		if entry.Type == "assistant" && entry.Message != nil {
			for _, content := range entry.Message.contentItems() {
				if content.Type == "tool_use" && content.ID != "" && content.Name != "" &&
					content.Name != "Task" && content.Name != "TodoWrite" {
					toolIDToName[content.ID] = content.Name
					pendingIDs[content.ID] = true
				}
			}
		}

		// tool_result entries come in as type "user" with content items of type "tool_result".
		// Do NOT break early: parallel tool calls produce multiple tool_result items
		// inside the same user entry.
		if entry.Type == "user" && entry.Message != nil {
			for _, content := range entry.Message.contentItems() {
				if content.Type != "tool_result" {
					continue
				}
				toolName := toolIDToName[content.ToolUseID]
				if toolName != "" {
					delete(pendingIDs, content.ToolUseID)
					if content.IsError {
						summary.FailedTools[toolName]++
					} else {
						summary.CompletedTools[toolName]++
					}
				}
			}
		}
	}

	// ActiveTools = tool_use calls in the current turn with no result yet
	seenActive := make(map[string]bool)
	for id := range pendingIDs {
		if name, ok := toolIDToName[id]; ok && !seenActive[name] {
			seenActive[name] = true
			summary.ActiveTools = append(summary.ActiveTools, name)
		}
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
		// Use rune count for proper UTF-8 (Chinese character) handling
		runes := []rune(desc)
		if len(runes) > 20 {
			desc = string(runes[:17]) + ".."
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

	// Normalize backslashes manually — filepath.ToSlash only replaces
	// os.PathSeparator, which is '/' on Linux (no-op for '\').
	parts := strings.Split(strings.ReplaceAll(dir, "\\", "/"), "/")
	name := parts[len(parts)-1]
	if len(name) > 20 {
		return name[:17] + ".."
	}
	return name
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
	output, err := defaultCommandRunner.Run(path, "git", "symbolic-ref", "--short", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}

	// Method 2: Try git rev-parse --abbrev-ref HEAD (fallback)
	// This returns "HEAD" for detached HEAD state or fresh repos
	output, err = defaultCommandRunner.Run(path, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(string(output))
		// For freshly initialized repos with no commits, show "(empty)"
		// For detached HEAD, show the commit abbreviation
		if branch == "" || branch == "HEAD" {
			// Check if this is a fresh repo (exists but no commits)
			_, err = defaultCommandRunner.Run(path, "git", "status", "--porcelain")
			if err == nil {
				// Git repo exists but might be empty
				// Try to get the default branch name
				output, err = defaultCommandRunner.Run(path, "git", "rev-parse", "--abbrev-ref", "origin/HEAD")
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
