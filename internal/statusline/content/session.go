package content

import (
	"fmt"
	"sort"
	"strings"
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

// ToolStatusDetailCollector collects per-tool success/failure breakdown
type ToolStatusDetailCollector struct {
	*BaseCollector
}

// NewToolStatusDetailCollector creates a new tool status detail collector
func NewToolStatusDetailCollector() *ToolStatusDetailCollector {
	return &ToolStatusDetailCollector{
		BaseCollector: NewBaseCollector(ContentToolStatusDetail, 5*time.Second, true),
	}
}

// Collect returns per-tool call breakdown with success/failure indicators
func (c *ToolStatusDetailCollector) Collect(input interface{}, summary interface{}) (string, error) {
	transcriptSummary, ok := summary.(*TranscriptSummary)
	if !ok {
		return "", fmt.Errorf("invalid summary type")
	}

	if len(transcriptSummary.CompletedTools) == 0 && len(transcriptSummary.FailedTools) == 0 {
		return "", nil
	}

	const (
		green = "\x1b[1;32m"
		red   = "\x1b[1;31m"
		reset = "\x1b[0m"
	)

	// Build sorted list of successful tools (by count desc)
	type toolEntry struct {
		name  string
		count int
	}

	var successEntries []toolEntry
	for name, count := range transcriptSummary.CompletedTools {
		successEntries = append(successEntries, toolEntry{name, count})
	}
	sort.Slice(successEntries, func(i, j int) bool {
		if successEntries[i].count != successEntries[j].count {
			return successEntries[i].count > successEntries[j].count
		}
		return successEntries[i].name < successEntries[j].name
	})

	var failedEntries []toolEntry
	for name, count := range transcriptSummary.FailedTools {
		failedEntries = append(failedEntries, toolEntry{name, count})
	}
	sort.Slice(failedEntries, func(i, j int) bool {
		if failedEntries[i].count != failedEntries[j].count {
			return failedEntries[i].count > failedEntries[j].count
		}
		return failedEntries[i].name < failedEntries[j].name
	})

	var parts []string
	for _, e := range successEntries {
		parts = append(parts, fmt.Sprintf("%s✓%s %s(%d)", green, reset, e.name, e.count))
	}
	for _, e := range failedEntries {
		parts = append(parts, fmt.Sprintf("%s✖%s %s(%d)", red, reset, e.name, e.count))
	}

	return strings.Join(parts, " "), nil
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
