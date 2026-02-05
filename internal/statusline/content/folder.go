package content

import (
	"fmt"
	"path/filepath"
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
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Workspace      struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
}

// TranscriptSummary represents parsed transcript data
type TranscriptSummary struct {
	GitBranch      string
	GitStatus      string
	ActiveTools    []string
	CompletedTools map[string]int
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

	// Use filepath.Base which handles both \ and / correctly
	name := filepath.Base(cwd)

	if len(name) > 25 {
		return name[:22] + ".."
	}
	return name
}
