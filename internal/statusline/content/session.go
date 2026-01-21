package content

import (
	"fmt"
	"time"
)

// AgentCollector collects agent information
type AgentCollector struct {
	*BaseCollector
}

// NewAgentCollector creates a new agent collector
func NewAgentCollector() *AgentCollector {
	return &AgentCollector{
		BaseCollector: NewBaseCollector(ContentAgent, 5*time.Second, true),
	}
}

// Collect returns agent information
func (c *AgentCollector) Collect(input interface{}, summary interface{}) (string, error) {
	transcriptSummary, ok := summary.(*TranscriptSummary)
	if !ok {
		return "", fmt.Errorf("invalid summary type")
	}
	if len(transcriptSummary.Agents) == 0 {
		return "", nil
	}
	agent := transcriptSummary.Agents[len(transcriptSummary.Agents)-1]
	agentInfo := agent.Type
	if agent.Desc != "" {
		desc := agent.Desc
		runes := []rune(desc)
		if len(runes) > 20 {
			desc = string(runes[:17]) + ".."
		}
		agentInfo = fmt.Sprintf("%s: %s", agentInfo, desc)
	}
	return fmt.Sprintf("🤖 %s", agentInfo), nil
}

// TodoCollector collects TODO progress
type TodoCollector struct {
	*BaseCollector
}

// NewTodoCollector creates a new TODO collector
func NewTodoCollector() *TodoCollector {
	return &TodoCollector{
		BaseCollector: NewBaseCollector(ContentTodo, 5*time.Second, true),
	}
}

// Collect returns TODO progress
func (c *TodoCollector) Collect(input interface{}, summary interface{}) (string, error) {
	transcriptSummary, ok := summary.(*TranscriptSummary)
	if !ok {
		return "", fmt.Errorf("invalid summary type")
	}
	if transcriptSummary.TodoTotal == 0 {
		return "", nil
	}
	if transcriptSummary.TodoCompleted == transcriptSummary.TodoTotal {
		return fmt.Sprintf("📋 ✓ %d/%d", transcriptSummary.TodoCompleted, transcriptSummary.TodoTotal), nil
	}
	return fmt.Sprintf("📋 %d/%d", transcriptSummary.TodoCompleted, transcriptSummary.TodoTotal), nil
}

// ToolsCollector collects tool usage statistics
type ToolsCollector struct {
	*BaseCollector
}

// NewToolsCollector creates a new tools collector
func NewToolsCollector() *ToolsCollector {
	return &ToolsCollector{
		BaseCollector: NewBaseCollector(ContentTools, 5*time.Second, true),
	}
}

// Collect returns tool usage statistics
func (c *ToolsCollector) Collect(input interface{}, summary interface{}) (string, error) {
	transcriptSummary, ok := summary.(*TranscriptSummary)
	if !ok {
		return "", fmt.Errorf("invalid summary type")
	}
	if len(transcriptSummary.CompletedTools) == 0 {
		return "", nil
	}
	total := 0
	for _, count := range transcriptSummary.CompletedTools {
		total += count
	}
	return fmt.Sprintf("🔧 %d tools", total), nil
}

// SessionDurationCollector collects session duration
type SessionDurationCollector struct {
	*BaseCollector
}

// NewSessionDurationCollector creates a new session duration collector
func NewSessionDurationCollector() *SessionDurationCollector {
	return &SessionDurationCollector{
		BaseCollector: NewBaseCollector(ContentSessionDuration, 5*time.Second, true),
	}
}

// Collect returns session duration
func (c *SessionDurationCollector) Collect(input interface{}, summary interface{}) (string, error) {
	transcriptSummary, ok := summary.(*TranscriptSummary)
	if !ok {
		return "", fmt.Errorf("invalid summary type")
	}
	if transcriptSummary.SessionStart.IsZero() {
		return "", nil
	}
	var duration time.Duration
	if !transcriptSummary.SessionEnd.IsZero() {
		duration = transcriptSummary.SessionEnd.Sub(transcriptSummary.SessionStart)
	} else {
		duration = time.Since(transcriptSummary.SessionStart)
	}
	return fmt.Sprintf("⏱️ %s", formatDuration(duration)), nil
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
