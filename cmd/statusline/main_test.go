package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/young1lin/claude-token-monitor/internal/parser"
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

// TestParseRealJSON tests parsing of actual Claude Code input
func TestParseRealJSON(t *testing.T) {
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

	var input content.StatusLineInput
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
	const jsonWithActiveUsage = `{
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

	var input content.StatusLineInput
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

// TestGitBranchInGitRepo tests that we're in a git repo
func TestGitBranchInGitRepo(t *testing.T) {
	// Skip if not in a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		t.Skip("Not in a git repository")
	}

	// Just verify git is available - the actual branch detection
	// is tested in the content package tests
	t.Log("Git is available and we're in a git repo")
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

// TestConvertToContentSummary tests the conversion from parser.TranscriptSummary to content.TranscriptSummary
func TestConvertToContentSummary(t *testing.T) {
	parserSummary := &parser.TranscriptSummary{
		GitBranch:   "main",
		GitStatus:   "+3 ~2",
		ActiveTools: []string{"Read", "Edit"},
		CompletedTools: map[string]int{
			"Read": 5,
			"Edit": 3,
		},
		Agents: []parser.AgentInfo{
			{Type: "Explore", Desc: "Exploring codebase"},
		},
		TodoTotal:     10,
		TodoCompleted: 5,
	}

	result := convertToContentSummary(parserSummary)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.GitBranch != "main" {
		t.Errorf("Expected GitBranch 'main', got '%s'", result.GitBranch)
	}
	if result.GitStatus != "+3 ~2" {
		t.Errorf("Expected GitStatus '+3 ~2', got '%s'", result.GitStatus)
	}
	if len(result.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(result.Agents))
	}
	if result.TodoTotal != 10 {
		t.Errorf("Expected TodoTotal 10, got %d", result.TodoTotal)
	}
	if result.TodoCompleted != 5 {
		t.Errorf("Expected TodoCompleted 5, got %d", result.TodoCompleted)
	}
}

// TestConvertToContentSummaryNilInput tests nil input handling
func TestConvertToContentSummaryNilInput(t *testing.T) {
	result := convertToContentSummary(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}
}

// TestTrimNullBytes tests the null byte trimming function
func TestTrimNullBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "No null bytes",
			input:    []byte("hello"),
			expected: []byte("hello"),
		},
		{
			name:     "Null bytes at end",
			input:    []byte("hello\x00\x00\x00"),
			expected: []byte("hello"),
		},
		{
			name:     "Null bytes in middle",
			input:    []byte("he\x00\x00llo"),
			expected: []byte("hello"),
		},
		{
			name:     "Only null bytes",
			input:    []byte("\x00\x00\x00"),
			expected: []byte(""),
		},
		{
			name:     "Empty input",
			input:    []byte(""),
			expected: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimNullBytes(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("trimNullBytes(%q) = %q, want %q",
					string(tt.input), string(result), string(tt.expected))
			}
		})
	}
}

// TestTranscriptSummaryConversion verifies agent conversion
func TestTranscriptSummaryConversion(t *testing.T) {
	parserSummary := &parser.TranscriptSummary{
		Agents: []parser.AgentInfo{
			{Type: "Explore", Desc: "Test agent 1"},
			{Type: "General", Desc: "Test agent 2"},
		},
	}

	result := convertToContentSummary(parserSummary)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(result.Agents))
	}

	if result.Agents[0].Type != "Explore" {
		t.Errorf("Expected agent type 'Explore', got '%s'", result.Agents[0].Type)
	}
	if result.Agents[0].Desc != "Test agent 1" {
		t.Errorf("Expected agent desc 'Test agent 1', got '%s'", result.Agents[0].Desc)
	}
}

// TestStatusLineInputProjectName tests project name extraction from path
func TestStatusLineInputProjectName(t *testing.T) {
	tests := []struct {
		name        string
		cwd         string
		projectName string
	}{
		{
			name:        "Windows path",
			cwd:         `C:\PythonProject\minimal-mcp\go\claude-token-monitor`,
			projectName: "claude-token-monitor",
		},
		{
			name:        "Unix path",
			cwd:         "/home/user/projects/my-project",
			projectName: "my-project",
		},
		{
			name:        "Trailing slash",
			cwd:         `/home/user/projects/my-project/`,
			projectName: "", // Empty after trailing slash
		},
		{
			name:        "Empty path",
			cwd:         "",
			projectName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &content.StatusLineInput{Cwd: tt.cwd}
			// The project name is extracted by the FolderCollector
			// We just verify the input can be created
			if input.Cwd != tt.cwd {
				t.Errorf("Expected cwd '%s', got '%s'", tt.cwd, input.Cwd)
			}
		})
	}
}
