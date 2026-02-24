package parser

import (
	"encoding/json"
	"testing"
)

// makeToolUseEntry creates an assistant entry with a single tool_use content item.
// Content is marshaled to json.RawMessage to match the real JSONL format.
func makeToolUseEntry(toolID, toolName string) TranscriptEntry {
	raw, _ := json.Marshal([]ContentItem{{Type: "tool_use", ID: toolID, Name: toolName}})
	return TranscriptEntry{
		Type: "assistant",
		Message: &MessageContent{
			Content: json.RawMessage(raw),
		},
	}
}

// makeToolResultEntry creates a tool_result entry for a given tool_use_id.
// In the real JSONL format the outer entry type is "user", not "tool_result";
// the actual tool_result item lives inside message.content (as a JSON array).
func makeToolResultEntry(toolUseID string, isError bool) TranscriptEntry {
	raw, _ := json.Marshal([]ContentItem{{Type: "tool_result", ToolUseID: toolUseID, IsError: isError}})
	return TranscriptEntry{
		Type: "user",
		Message: &MessageContent{
			Content: json.RawMessage(raw),
		},
	}
}

// TestToolStatusSuccess verifies successful tool results populate CompletedTools
func TestToolStatusSuccess(t *testing.T) {
	// Arrange
	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Read"),
		makeToolResultEntry("id-1", false),
	}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert
	if got := summary.CompletedTools["Read"]; got != 1 {
		t.Errorf("CompletedTools[Read] = %d, want 1", got)
	}
	if len(summary.FailedTools) != 0 {
		t.Errorf("FailedTools should be empty, got %v", summary.FailedTools)
	}
}

// TestToolStatusFailure verifies is_error:true tool results populate FailedTools
func TestToolStatusFailure(t *testing.T) {
	// Arrange
	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Edit"),
		makeToolResultEntry("id-1", true),
	}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert
	if got := summary.FailedTools["Edit"]; got != 1 {
		t.Errorf("FailedTools[Edit] = %d, want 1", got)
	}
	if got := summary.CompletedTools["Edit"]; got != 0 {
		t.Errorf("CompletedTools[Edit] = %d, want 0 (should not be completed)", got)
	}
}

// TestToolStatusMixed verifies mixed success/failure for the same tool
func TestToolStatusMixed(t *testing.T) {
	// Arrange: 2 Read calls (1 success, 1 failure), 1 Bash call (success)
	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Read"),
		makeToolResultEntry("id-1", false),
		makeToolUseEntry("id-2", "Read"),
		makeToolResultEntry("id-2", true),
		makeToolUseEntry("id-3", "Bash"),
		makeToolResultEntry("id-3", false),
	}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert
	if got := summary.CompletedTools["Read"]; got != 1 {
		t.Errorf("CompletedTools[Read] = %d, want 1", got)
	}
	if got := summary.FailedTools["Read"]; got != 1 {
		t.Errorf("FailedTools[Read] = %d, want 1", got)
	}
	if got := summary.CompletedTools["Bash"]; got != 1 {
		t.Errorf("CompletedTools[Bash] = %d, want 1", got)
	}
	if got := summary.FailedTools["Bash"]; got != 0 {
		t.Errorf("FailedTools[Bash] = %d, want 0", got)
	}
}

// TestToolStatusMultipleSuccesses verifies multiple successful calls are counted correctly
func TestToolStatusMultipleSuccesses(t *testing.T) {
	// Arrange: 3 Grep calls, all successful
	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Grep"),
		makeToolResultEntry("id-1", false),
		makeToolUseEntry("id-2", "Grep"),
		makeToolResultEntry("id-2", false),
		makeToolUseEntry("id-3", "Grep"),
		makeToolResultEntry("id-3", false),
	}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert
	if got := summary.CompletedTools["Grep"]; got != 3 {
		t.Errorf("CompletedTools[Grep] = %d, want 3", got)
	}
	if len(summary.FailedTools) != 0 {
		t.Errorf("FailedTools should be empty, got %v", summary.FailedTools)
	}
}

// TestToolStatusNoResults verifies that tool_use without a result is treated as pending
func TestToolStatusNoResults(t *testing.T) {
	// Arrange: tool_use without a result yet
	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Bash"),
	}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert: tool is pending (ActiveTools), not completed or failed
	found := false
	for _, tool := range summary.ActiveTools {
		if tool == "Bash" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Bash should be in ActiveTools (pending), got %v", summary.ActiveTools)
	}
	if got := summary.CompletedTools["Bash"]; got != 0 {
		t.Errorf("CompletedTools[Bash] = %d, want 0", got)
	}
	if got := summary.FailedTools["Bash"]; got != 0 {
		t.Errorf("FailedTools[Bash] = %d, want 0", got)
	}
}

// TestFailedToolsInitialized verifies FailedTools map is always initialized (not nil)
func TestFailedToolsInitialized(t *testing.T) {
	// Arrange: empty entries
	entries := []TranscriptEntry{}

	// Act
	summary := analyzeTranscriptEntries(entries)

	// Assert: maps should be initialized, not nil
	if summary.CompletedTools == nil {
		t.Error("CompletedTools should be initialized, got nil")
	}
	if summary.FailedTools == nil {
		t.Error("FailedTools should be initialized, got nil")
	}
}
