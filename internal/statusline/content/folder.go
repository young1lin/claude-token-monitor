package content

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// StatusLineInput represents the input from Claude Code
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
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	Cost struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMs    int     `json:"total_duration_ms"`
		TotalAPIDurationMs int     `json:"total_api_duration_ms"`
		TotalLinesAdded    int     `json:"total_lines_added"`
		TotalLinesRemoved  int     `json:"total_lines_removed"`
	} `json:"cost"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Workspace      struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`

	// Version is the Claude Code CLI version, sent directly by the host since
	// CC 2.1.x. Empty when the host predates this field — collectors fall back
	// to invoking `claude --version`.
	Version string `json:"version"`

	// RateLimits is the host-provided Anthropic subscription usage snapshot,
	// available since CC 2.1.x. When present, the quota collector uses it
	// directly and skips the OAuth /api/oauth/usage HTTP path entirely
	// (saving a request and avoiding 429 backoff). Nil = older CC / not a
	// subscription account → fall back to the API flow.
	RateLimits *StdinRateLimits `json:"rate_limits,omitempty"`

	// Effort is the current thinking-effort tier ("low" / "medium" / "high" /
	// "xhigh") chosen for this session. The mode-flags collector treats
	// "medium" as the default and only surfaces tiers that diverge from it
	// (so the chip stays out of the way in the common case).
	Effort struct {
		Level string `json:"level"`
	} `json:"effort"`

	// Thinking carries the extended-thinking toggle. When enabled, the
	// mode-flags collector renders 💭 so users see at a glance that the
	// model is allowed to spend extra tokens reasoning.
	Thinking struct {
		Enabled bool `json:"enabled"`
	} `json:"thinking"`

	// FastMode is the high-throughput / low-latency hint. Off by default;
	// when true, the mode-flags collector renders ⚡.
	FastMode bool `json:"fast_mode"`
}

// StdinRateLimitWindow is one CC-supplied usage window. ResetsAt is Unix
// seconds (e.g. 1779798600); zero means "no known reset time".
type StdinRateLimitWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

// StdinRateLimits mirrors the JSON shape Claude Code emits under "rate_limits".
// Either window can be nil if the host hasn't computed it yet.
type StdinRateLimits struct {
	FiveHour *StdinRateLimitWindow `json:"five_hour,omitempty"`
	SevenDay *StdinRateLimitWindow `json:"seven_day,omitempty"`
}

// TranscriptSummary represents parsed transcript data
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
}

// AgentInfo represents agent information
type AgentInfo struct {
	Type string
	Desc string
}

// FolderCollector collects the project folder name
type FolderCollector struct {
	*BaseCollector
}

// NewFolderCollector creates a new folder collector
func NewFolderCollector() *FolderCollector {
	return &FolderCollector{
		BaseCollector: NewBaseCollector(ContentFolder, 60*time.Second, false),
	}
}

// Collect returns the project folder name
func (c *FolderCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getProjectName(statusInput.Cwd), nil
}

// getProjectName extracts the project folder name
func getProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}

	// Normalize backslashes manually — filepath.Base on Linux treats '\' as
	// a regular character, not a path separator.
	normalized := strings.ReplaceAll(cwd, "\\", "/")
	name := filepath.Base(normalized)

	// Slice by rune, not byte: a project name like "我的中文项目-app" would
	// otherwise get cut mid-UTF-8 and render as broken replacement glyphs.
	// 32-rune cap matches TruncateBranch so the two cells stay visually
	// balanced; both leave 29 runes + ".." in the truncated case.
	runes := []rune(name)
	if len(runes) > 32 {
		return string(runes[:29]) + ".."
	}
	return name
}
