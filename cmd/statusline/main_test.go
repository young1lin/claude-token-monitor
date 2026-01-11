package main

import (
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
