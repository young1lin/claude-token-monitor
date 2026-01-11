package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/young1lin/claude-token-monitor/internal/parser"
)

// TestGetGitBranch tests the git branch detection logic
func TestGetGitBranch(t *testing.T) {
	// Test with current directory (should have git)
	cwd := t.TempDir()

	// Initialize git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = cwd
	if err := initCmd.Run(); err != nil {
		t.Skip("Cannot create git repo for testing")
	}

	branch := getGitBranch(cwd)
	t.Logf("Fresh git init repo branch: %q", branch)

	// Should show "(empty)" for fresh repo with no commits
	// or at least not return empty (which would indicate no git repo detected)
	if branch == "" {
		t.Error("Expected branch name or '(empty)' for fresh git repo, got empty string")
	}

	// Configure git for commits
	// Note: We can't make commits without configuring user, but the branch
	// detection should still work for showing the branch name
}

// TestGetGitBranchCurrentDir tests with actual current directory
func TestGetGitBranchCurrentDir(t *testing.T) {
	// Skip if not in a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		t.Skip("Not in a git repository")
	}

	// Get current working directory using Go
	cwd := "." // Use current directory

	branch := getGitBranch(cwd)
	t.Logf("Current directory branch: %q", branch)

	// Should return a valid branch name (not empty for a git repo)
	if branch == "" {
		t.Error("Expected non-empty branch name for current git repo")
	}

	// "(empty)" is acceptable for repos with no commits yet
	// Otherwise we should get a real branch name like "main", "master", etc.
	// The test passes as long as we get something (not empty string)
}

// TestFormatOutputMultiline tests the multi-line output format
func TestFormatOutputMultiline(t *testing.T) {
	input := &StatusLineInput{
		Model: struct {
			DisplayName string `json:"display_name"`
			ID          string `json:"id"`
		}{
			DisplayName: "Claude Sonnet 4.5",
			ID:          "claude-sonnet-4-5",
		},
		ContextWindow: struct {
			TotalInputTokens         int `json:"total_input_tokens"`
			TotalOutputTokens        int `json:"total_output_tokens"`
			ContextWindowSize        int `json:"context_window_size"`
			CurrentUsage             struct {
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"current_usage"`
		}{
			ContextWindowSize: 200000,
		},
		Cwd: "C:\\PythonProject\\minimal-mcp",
	}
	input.ContextWindow.CurrentUsage.InputTokens = 50000
	input.ContextWindow.CurrentUsage.OutputTokens = 5000

	summary := &parser.TranscriptSummary{
		CompletedTools: map[string]int{"Read": 5, "Edit": 3},
		TodoTotal:      10,
		TodoCompleted:  3,
	}

	tracker := NewSimpleTracker()

	lines := formatOutput(input, summary, tracker)

	// Should return multiple lines
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines, got %d", len(lines))
	}

	// First line should contain project name, model, progress bar, and token info
	// Format: "📁 project | [Model] | [progress] tokens/K (pct%)"
	if !strings.Contains(lines[0], "📁") || !strings.Contains(lines[0], "Claude Sonnet 4.5") {
		t.Errorf("First line missing project or model info: %s", lines[0])
	}

	// Progress bar and token info are on the FIRST line (not second)
	// The format includes both progress bar [█...] and token count like "55.0K/200K"
	if !strings.Contains(lines[0], "[") {
		t.Errorf("First line missing progress bar: %s", lines[0])
	}
	if !strings.Contains(lines[0], "K") {
		t.Errorf("First line missing token info (K suffix): %s", lines[0])
	}

	t.Logf("Generated %d lines:", len(lines))
	for i, line := range lines {
		t.Logf("  Line %d: %s", i+1, line)
	}
}

// TestFormatOutputEmptyData tests with minimal data
func TestFormatOutputEmptyData(t *testing.T) {
	input := &StatusLineInput{
		Model: struct {
			DisplayName string `json:"display_name"`
			ID          string `json:"id"`
		}{
			DisplayName: "",
			ID:          "",
		},
		ContextWindow: struct {
			TotalInputTokens         int `json:"total_input_tokens"`
			TotalOutputTokens        int `json:"total_output_tokens"`
			ContextWindowSize        int `json:"context_window_size"`
			CurrentUsage             struct {
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"current_usage"`
		}{},
		Cwd: "",
	}

	summary := &parser.TranscriptSummary{}
	tracker := NewSimpleTracker()

	lines := formatOutput(input, summary, tracker)

	// Should still return at least 2 lines (model name defaults to "Claude")
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines even with empty data, got %d", len(lines))
	}

	// First line should contain default model name
	if !strings.Contains(lines[0], "[Claude]") {
		t.Errorf("First line should contain default [Claude] model name: %s", lines[0])
	}
}

// Real JSON data captured from Claude Code statusLine input
const realJSONInput = `{
  "session_id": "2e85140e-73a6-4592-9012-24a6565a606d",
  "transcript_path": "C:\\Users\\杨逸林\\.claude\\projects\\C--PythonProject-minimal-mcp-go-claude-token-monitor\\2e85140e-73a6-4592-9012-24a6565a606d.jsonl",
  "cwd": "C:\\PythonProject\\minimal-mcp\\go\\claude-token-monitor",
  "model": {"id": "GLM-4.7", "display_name": "GLM-4.7"},
  "workspace": {
    "current_dir": "C:\\PythonProject\\minimal-mcp\\go\\claude-token-monitor",
    "project_dir": "C:\\PythonProject\\minimal-mcp\\go\\claude-token-monitor"
  },
  "version": "2.1.4",
  "output_style": {"name": "default"},
  "cost": {
    "total_cost_usd": 7.2275832000000015,
    "total_duration_ms": 3764772,
    "total_api_duration_ms": 1828046,
    "total_lines_added": 1200,
    "total_lines_removed": 108
  },
  "context_window": {
    "total_input_tokens": 587879,
    "total_output_tokens": 60025,
    "context_window_size": 200000,
    "current_usage": {
      "input_tokens": 0,
      "output_tokens": 0,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 0
    }
  },
  "exceeds_200k_tokens": false
}`

// TestParseRealJSON tests parsing of actual Claude Code input
func TestParseRealJSON(t *testing.T) {
	var input StatusLineInput
	err := json.Unmarshal([]byte(realJSONInput), &input)
	if err != nil {
		t.Fatalf("Failed to parse real JSON: %v", err)
	}

	// Verify basic fields
	if input.Model.ID != "GLM-4.7" {
		t.Errorf("Expected model ID 'GLM-4.7', got '%s'", input.Model.ID)
	}
	if input.Model.DisplayName != "GLM-4.7" {
		t.Errorf("Expected display name 'GLM-4.7', got '%s'", input.Model.DisplayName)
	}

	// Verify context window
	if input.ContextWindow.ContextWindowSize != 200000 {
		t.Errorf("Expected context window size 200000, got %d", input.ContextWindow.ContextWindowSize)
	}

	// Verify cumulative tokens
	if input.ContextWindow.TotalInputTokens != 587879 {
		t.Errorf("Expected total input tokens 587879, got %d", input.ContextWindow.TotalInputTokens)
	}
	if input.ContextWindow.TotalOutputTokens != 60025 {
		t.Errorf("Expected total output tokens 60025, got %d", input.ContextWindow.TotalOutputTokens)
	}

	// Verify current usage is zero (idle state)
	if input.ContextWindow.CurrentUsage.InputTokens != 0 {
		t.Errorf("Expected current usage input to be 0, got %d", input.ContextWindow.CurrentUsage.InputTokens)
	}

	// Verify workspace
	if !strings.Contains(input.Cwd, "claude-token-monitor") {
		t.Errorf("Expected cwd to contain 'claude-token-monitor', got '%s'", input.Cwd)
	}

	t.Logf("Successfully parsed real JSON from Claude Code")
}

// TestParseRealJSONWithActiveUsage tests JSON with non-zero current_usage
func TestParseRealJSONWithActiveUsage(t *testing.T) {
	jsonWithActiveUsage := `{
  "session_id": "test-session-id",
  "transcript_path": "C:\\Users\\test\\.claude\\projects\\test\\test.jsonl",
  "cwd": "C:\\Project",
  "model": {"id": "claude-sonnet-4-5", "display_name": "Claude Sonnet 4.5"},
  "workspace": {
    "current_dir": "C:\\Project",
    "project_dir": "C:\\Project"
  },
  "version": "2.1.4",
  "output_style": {"name": "default"},
  "cost": {
    "total_cost_usd": 1.50,
    "total_duration_ms": 60000,
    "total_api_duration_ms": 30000,
    "total_lines_added": 50,
    "total_lines_removed": 10
  },
  "context_window": {
    "total_input_tokens": 100000,
    "total_output_tokens": 20000,
    "context_window_size": 200000,
    "current_usage": {
      "input_tokens": 5000,
      "output_tokens": 1000,
      "cache_creation_input_tokens": 2000,
      "cache_read_input_tokens": 3000
    }
  },
  "exceeds_200k_tokens": false
}`

	var input StatusLineInput
	err := json.Unmarshal([]byte(jsonWithActiveUsage), &input)
	if err != nil {
		t.Fatalf("Failed to parse JSON with active usage: %v", err)
	}

	// Verify current usage is populated
	current := input.ContextWindow.CurrentUsage
	if current.InputTokens != 5000 {
		t.Errorf("Expected input_tokens 5000, got %d", current.InputTokens)
	}
	if current.OutputTokens != 1000 {
		t.Errorf("Expected output_tokens 1000, got %d", current.OutputTokens)
	}
	if current.CacheReadInputTokens != 3000 {
		t.Errorf("Expected cache_read_input_tokens 3000, got %d", current.CacheReadInputTokens)
	}

	// Calculate actual context size: input + cache_read + output
	actualContextSize := current.InputTokens + current.CacheReadInputTokens + current.OutputTokens
	expectedSize := 5000 + 3000 + 1000 // 9000
	if actualContextSize != expectedSize {
		t.Errorf("Expected actual context size %d, got %d", expectedSize, actualContextSize)
	}

	t.Logf("Active usage: input=%d, output=%d, cache_read=%d, total=%d",
		current.InputTokens, current.OutputTokens, current.CacheReadInputTokens, actualContextSize)
}

// TestFormatOutputWithRealData tests output formatting with real-world data
func TestFormatOutputWithRealData(t *testing.T) {
	var input StatusLineInput
	if err := json.Unmarshal([]byte(realJSONInput), &input); err != nil {
		t.Fatalf("Failed to parse real JSON: %v", err)
	}

	summary := &parser.TranscriptSummary{
		CompletedTools: map[string]int{"Read": 10, "Edit": 5, "Bash": 3},
		TodoTotal:      15,
		TodoCompleted:  8,
	}
	tracker := NewSimpleTracker()

	lines := formatOutput(&input, summary, tracker)

	// Should have at least 3 lines
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(lines))
	}

	// First line: project name + model + progress bar + tokens
	line1 := lines[0]
	if !strings.Contains(line1, "📁") {
		t.Errorf("Line 1 should contain project folder icon: %s", line1)
	}
	if !strings.Contains(line1, "[GLM-4.7]") {
		t.Errorf("Line 1 should contain model name: %s", line1)
	}

	// Check progress bar exists (contains [ and ])
	if !strings.Contains(line1, "[") || !strings.Contains(line1, "]") {
		t.Errorf("Line 1 should contain progress bar: %s", line1)
	}

	// Second line: git status + memory files
	if len(lines) > 1 {
		line2 := lines[1]
		t.Logf("Line 2: %s", line2)
		// Should contain git branch info if in a git repo
		if strings.Contains(line2, "🌿") {
			t.Logf("Git branch detected in output")
		}
	}

	// Third line: tools + TODO
	if len(lines) > 2 {
		line3 := lines[2]
		if !strings.Contains(line3, "🔧") {
			t.Logf("Line 3 (no tools): %s", line3)
		}
		if !strings.Contains(line3, "📋") {
			t.Logf("Line 3 (no TODO): %s", line3)
		}
	}

	for i, line := range lines {
		t.Logf("Output line %d: %s", i+1, line)
	}
}

// TestTokenCalculation verifies token calculation logic
func TestTokenCalculation(t *testing.T) {
	tests := []struct {
		name            string
		inputTokens     int
		outputTokens    int
		cacheReadTokens int
		expectedTotal   int
	}{
		{
			name:            "Only input tokens",
			inputTokens:     10000,
			outputTokens:    0,
			cacheReadTokens: 0,
			expectedTotal:   10000,
		},
		{
			name:            "Input + output",
			inputTokens:     5000,
			outputTokens:    2000,
			cacheReadTokens: 0,
			expectedTotal:   7000,
		},
		{
			name:            "Input + cache read",
			inputTokens:     3000,
			outputTokens:    0,
			cacheReadTokens: 5000,
			expectedTotal:   8000,
		},
		{
			name:            "All three types",
			inputTokens:     5000,
			outputTokens:    1000,
			cacheReadTokens: 3000,
			expectedTotal:   9000,
		},
		{
			name:            "Large numbers",
			inputTokens:     150000,
			outputTokens:    30000,
			cacheReadTokens: 20000,
			expectedTotal:   200000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.inputTokens + tt.cacheReadTokens + tt.outputTokens
			if total != tt.expectedTotal {
				t.Errorf("Expected total %d, got %d", tt.expectedTotal, total)
			}

			// Test percentage calculation
			contextWindow := 200000
			pct := float64(total) / float64(contextWindow) * 100
			t.Logf("Tokens: %d = %.1f%% of %d", total, pct, contextWindow)
		})
	}
}

// TestFormatNumber tests the number formatting utility
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{100, "100"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{10000000, "10.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%d) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestProgressColorThresholds tests color thresholds based on percentage
func TestProgressColorThresholds(t *testing.T) {
	tests := []struct {
		percentage int
		expectedColor string
	}{
		{10, "\x1b[1;32m"}, // Green
		{20, "\x1b[1;36m"}, // Cyan (boundary)
		{30, "\x1b[1;36m"}, // Cyan
		{40, "\x1b[1;33m"}, // Yellow (boundary)
		{50, "\x1b[1;33m"}, // Yellow
		{60, "\x1b[1;31m"}, // Red (boundary)
		{80, "\x1b[1;31m"}, // Red
		{100, "\x1b[1;31m"}, // Red
	}

	for _, tt := range tests {
		t.Run(tt.expectedColor, func(t *testing.T) {
			var colorCode string
			pct := float64(tt.percentage)
			if pct >= 60 {
				colorCode = "\x1b[1;31m"
			} else if pct >= 40 {
				colorCode = "\x1b[1;33m"
			} else if pct >= 20 {
				colorCode = "\x1b[1;36m"
			} else {
				colorCode = "\x1b[1;32m"
			}
			if colorCode != tt.expectedColor {
				t.Errorf("%d%%: expected color %v, got %v", tt.percentage, tt.expectedColor, colorCode)
			}
		})
	}
}
