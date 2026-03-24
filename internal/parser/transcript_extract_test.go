package parser

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers for building TranscriptEntry structs
// ---------------------------------------------------------------------------

// makeUserTextEntry creates a TranscriptEntry that represents a real user text
// message (type "user" with string content).
func makeUserTextEntry(text string) TranscriptEntry {
	raw, _ := json.Marshal(text) // JSON string: "hello"
	return TranscriptEntry{
		Type: "user",
		Message: &MessageContent{
			Content: json.RawMessage(raw),
		},
	}
}

// makeAssistantEntry creates an assistant entry with the given usage tokens
// and optional content items.
func makeAssistantEntry(inputTokens, outputTokens, cacheTokens int, items []ContentItem) TranscriptEntry {
	var content json.RawMessage
	if items != nil {
		raw, _ := json.Marshal(items)
		content = raw
	}
	return TranscriptEntry{
		Type: "assistant",
		Message: &MessageContent{
			Content: content,
			Usage: TokenUsage{
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheTokens,
			},
		},
	}
}

// makeTodoWriteEntry creates an assistant entry with a TodoWrite tool_use item.
// Each todo is a map[string]interface{} with a "status" key.
func makeTodoWriteEntry(todos []map[string]interface{}) TranscriptEntry {
	input := map[string]interface{}{"todos": convertToInterfaceSlice(todos)}
	items := []ContentItem{{
		Type:  "tool_use",
		Name:  "TodoWrite",
		ID:    "todo-id-1",
		Input: input,
	}}
	return makeAssistantEntry(0, 0, 0, items)
}

// convertToInterfaceSlice converts a slice of maps to []interface{}.
func convertToInterfaceSlice(maps []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(maps))
	for i, m := range maps {
		result[i] = m
	}
	return result
}

// makeTaskEntry creates an assistant entry with a Task (agent) tool_use item.
func makeTaskEntry(agentType, desc string) TranscriptEntry {
	input := map[string]interface{}{}
	if agentType != "" {
		input["subagent_type"] = agentType
	}
	if desc != "" {
		input["description"] = desc
	}
	items := []ContentItem{{
		Type:  "tool_use",
		Name:  "Task",
		ID:    "task-id-1",
		Input: input,
	}}
	return makeAssistantEntry(0, 0, 0, items)
}

// makeEmptyUserEntry creates a user entry with no message (type "user", nil message).
func makeEmptyUserEntry() TranscriptEntry {
	return TranscriptEntry{Type: "user"}
}

// makeUserEntryWithNoContent creates a user entry with a Message but empty Content.
func makeUserEntryWithNoContent() TranscriptEntry {
	return TranscriptEntry{
		Type:    "user",
		Message: &MessageContent{Content: json.RawMessage(`""`)},
	}
}

// ---------------------------------------------------------------------------
// Tests for extractTodoInfo (via analyzeTranscriptEntries)
// ---------------------------------------------------------------------------

func TestExtractTodoInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		entries       []TranscriptEntry
		wantTotal     int
		wantCompleted int
	}{
		{
			name: "no entries yields zero todos",
			entries: []TranscriptEntry{
				makeUserTextEntry("hello"),
				makeAssistantEntry(10, 5, 0, nil),
			},
			wantTotal:     0,
			wantCompleted: 0,
		},
		{
			name: "TodoWrite with completed and pending items",
			entries: []TranscriptEntry{
				makeUserTextEntry("create a feature"),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "completed"},
					{"status": "pending"},
					{"status": "completed"},
				}),
			},
			wantTotal:     3,
			wantCompleted: 2,
		},
		{
			name: "TodoWrite with all completed",
			entries: []TranscriptEntry{
				makeUserTextEntry("do it all"),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "completed"},
					{"status": "completed"},
					{"status": "completed"},
					{"status": "completed"},
				}),
			},
			wantTotal:     4,
			wantCompleted: 4,
		},
		{
			name: "TodoWrite with all pending",
			entries: []TranscriptEntry{
				makeUserTextEntry("plan it out"),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "pending"},
					{"status": "pending"},
				}),
			},
			wantTotal:     2,
			wantCompleted: 0,
		},
		{
			name: "TodoWrite with empty todos list",
			entries: []TranscriptEntry{
				makeUserTextEntry("nothing to do"),
				makeTodoWriteEntry([]map[string]interface{}{}),
			},
			wantTotal:     0,
			wantCompleted: 0,
		},
		{
			name: "TodoWrite missing status field in todo item",
			entries: []TranscriptEntry{
				makeUserTextEntry("bad data"),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "completed"},
					{"name": "no status here"},
				}),
			},
			wantTotal:     2,
			wantCompleted: 1,
		},
		{
			name: "multiple TodoWrite calls - last one wins",
			entries: []TranscriptEntry{
				makeUserTextEntry("first pass"),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "completed"},
					{"status": "pending"},
				}),
				makeTodoWriteEntry([]map[string]interface{}{
					{"status": "completed"},
					{"status": "completed"},
					{"status": "pending"},
				}),
			},
			wantTotal:     3,
			wantCompleted: 2,
		},
		{
			name: "TodoWrite with non-map items in list",
			entries: []TranscriptEntry{
				makeUserTextEntry("weird data"),
				makeAssistantEntry(0, 0, 0, []ContentItem{{
					Type: "tool_use",
					Name: "TodoWrite",
					ID:   "todo-id-x",
					Input: map[string]interface{}{
						"todos": []interface{}{"string-item", 42, nil},
					},
				}}),
			},
			wantTotal:     3,
			wantCompleted: 0,
		},
		{
			name: "TodoWrite with todos field not a list",
			entries: []TranscriptEntry{
				makeUserTextEntry("malformed"),
				makeAssistantEntry(0, 0, 0, []ContentItem{{
					Type: "tool_use",
					Name: "TodoWrite",
					ID:   "todo-id-y",
					Input: map[string]interface{}{
						"todos": "not-a-list",
					},
				}}),
			},
			wantTotal:     0,
			wantCompleted: 0,
		},
		{
			name: "TodoWrite with no todos field at all",
			entries: []TranscriptEntry{
				makeUserTextEntry("missing key"),
				makeAssistantEntry(0, 0, 0, []ContentItem{{
					Type:  "tool_use",
					Name:  "TodoWrite",
					ID:    "todo-id-z",
					Input: map[string]interface{}{},
				}}),
			},
			wantTotal:     0,
			wantCompleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			summary := analyzeTranscriptEntries(tt.entries)

			// Assert
			assert.Equal(t, tt.wantTotal, summary.TodoTotal, "TodoTotal mismatch")
			assert.Equal(t, tt.wantCompleted, summary.TodoCompleted, "TodoCompleted mismatch")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for isRealUserMessage (via analyzeTranscriptEntries)
// ---------------------------------------------------------------------------

func TestIsRealUserMessageViaAnalysis(t *testing.T) {
	t.Parallel()

	t.Run("real user message is recognized", func(t *testing.T) {
		// Arrange: a real user text message followed by tool calls.
		entries := []TranscriptEntry{
			makeUserTextEntry("fix the bug"),
			makeToolUseEntry("id-1", "Read"),
			makeToolResultEntry("id-1", false),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: tool calls after the user message should be tracked.
		assert.Equal(t, 1, summary.CompletedTools["Read"],
			"Tools after real user message should be counted")
	})

	t.Run("tool_result user entry is not treated as real user message", func(t *testing.T) {
		// Arrange: only a tool_use + tool_result, no real user text message.
		// Since there is no real user message, lastUserMsgIdx stays -1,
		// so ALL entries after index -1 (i.e. all entries) are in the current turn.
		entries := []TranscriptEntry{
			makeToolUseEntry("id-1", "Bash"),
			makeToolResultEntry("id-1", false),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: tool calls are still tracked (no user message boundary).
		assert.Equal(t, 1, summary.CompletedTools["Bash"],
			"Tool calls with no user message should still be counted")
	})

	t.Run("tool calls before user message are not in current turn", func(t *testing.T) {
		// Arrange: tool call, then user message, then tool call.
		// Only tool calls after the last user message should be in the current turn.
		entries := []TranscriptEntry{
			makeToolUseEntry("id-old", "Read"),
			makeToolResultEntry("id-old", false),
			makeUserTextEntry("now do something else"),
			makeToolUseEntry("id-new", "Bash"),
			makeToolResultEntry("id-new", false),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: only the tool after the user message should be in the current turn.
		assert.Equal(t, 0, summary.CompletedTools["Read"],
			"Tool calls before the last user message should not be counted in current turn")
		assert.Equal(t, 1, summary.CompletedTools["Bash"],
			"Tool calls after the last user message should be counted")
	})

	t.Run("empty user entry is not a real user message", func(t *testing.T) {
		// Arrange: an empty user entry (no message field) should not be
		// treated as a real user message boundary.
		entries := []TranscriptEntry{
			makeEmptyUserEntry(),
			makeToolUseEntry("id-1", "Read"),
			makeToolResultEntry("id-1", false),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: all tool calls are counted (no real user message boundary).
		assert.Equal(t, 1, summary.CompletedTools["Read"],
			"Tool calls should be counted when there is no real user message")
	})

	t.Run("user entry with empty string content is a real user message", func(t *testing.T) {
		// isTextContent checks if Content starts with '"'. An empty string `""`
		// starts with `"`, so it IS considered text content.
		// The tool call comes AFTER the user message, so it IS in the current turn.
		entries := []TranscriptEntry{
			makeUserEntryWithNoContent(),
			makeToolUseEntry("id-1", "Read"),
			makeToolResultEntry("id-1", false),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: the user message acts as a boundary, but the tool call
		// comes AFTER it, so it should be counted in the current turn.
		assert.Equal(t, 1, summary.CompletedTools["Read"],
			"Tool calls after the user message (even with empty content) should count")
	})
}

// ---------------------------------------------------------------------------
// Tests for isTextContent / contentItems (via TranscriptEntry construction)
// ---------------------------------------------------------------------------

func TestIsTextContentAndContentItems(t *testing.T) {
	t.Parallel()

	t.Run("text content returns nil from contentItems", func(t *testing.T) {
		// Arrange
		raw := json.RawMessage(`"hello world"`)
		msg := &MessageContent{Content: raw}

		// Act & Assert
		assert.True(t, msg.isTextContent(), "string content should be text")
		assert.Nil(t, msg.contentItems(), "text content should return nil items")
	})

	t.Run("array content returns items from contentItems", func(t *testing.T) {
		// Arrange
		raw := json.RawMessage(`[{"type":"tool_use","name":"Read","id":"id-1"}]`)
		msg := &MessageContent{Content: raw}

		// Act & Assert
		assert.False(t, msg.isTextContent(), "array content should not be text")
		items := msg.contentItems()
		require.Len(t, items, 1, "should have 1 content item")
		assert.Equal(t, "tool_use", items[0].Type)
		assert.Equal(t, "Read", items[0].Name)
		assert.Equal(t, "id-1", items[0].ID)
	})

	t.Run("empty content is neither text nor items", func(t *testing.T) {
		// Arrange
		msg := &MessageContent{Content: nil}

		// Act & Assert
		assert.False(t, msg.isTextContent(), "nil content should not be text")
		assert.Nil(t, msg.contentItems(), "nil content should return nil items")
	})

	t.Run("empty byte slice content is neither text nor items", func(t *testing.T) {
		// Arrange
		msg := &MessageContent{Content: json.RawMessage{}}

		// Act & Assert
		assert.False(t, msg.isTextContent(), "empty content should not be text")
		assert.Nil(t, msg.contentItems(), "empty content should return nil items")
	})

	t.Run("invalid JSON returns nil items", func(t *testing.T) {
		// Arrange
		msg := &MessageContent{Content: json.RawMessage(`[{invalid}]`)}

		// Act & Assert
		assert.False(t, msg.isTextContent(), "invalid JSON starting with [ is not text")
		assert.Nil(t, msg.contentItems(), "invalid JSON should return nil items")
	})

	t.Run("tool_result items are parsed correctly", func(t *testing.T) {
		// Arrange
		raw := json.RawMessage(`[{"type":"tool_result","tool_use_id":"id-1","is_error":true}]`)
		msg := &MessageContent{Content: raw}

		// Act
		items := msg.contentItems()

		// Assert
		require.Len(t, items, 1)
		assert.Equal(t, "tool_result", items[0].Type)
		assert.Equal(t, "id-1", items[0].ToolUseID)
		assert.True(t, items[0].IsError)
	})

	t.Run("multiple content items are parsed correctly", func(t *testing.T) {
		// Arrange
		raw := json.RawMessage(`[
			{"type":"tool_use","name":"Read","id":"id-a"},
			{"type":"text"},
			{"type":"tool_use","name":"Bash","id":"id-b"}
		]`)
		msg := &MessageContent{Content: raw}

		// Act
		items := msg.contentItems()

		// Assert
		require.Len(t, items, 3)
		assert.Equal(t, "tool_use", items[0].Type)
		assert.Equal(t, "Read", items[0].Name)
		assert.Equal(t, "text", items[1].Type)
		assert.Equal(t, "tool_use", items[2].Type)
		assert.Equal(t, "Bash", items[2].Name)
	})
}

// ---------------------------------------------------------------------------
// Tests for agent extraction (Task tool_use)
// ---------------------------------------------------------------------------

func TestAgentExtraction(t *testing.T) {
	t.Parallel()

	t.Run("Task with default agent type", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			makeUserTextEntry("explore the code"),
			makeTaskEntry("", ""),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		require.Len(t, summary.Agents, 1)
		assert.Equal(t, "general-purpose", summary.Agents[0].Type)
		assert.Equal(t, "", summary.Agents[0].Desc)
	})

	t.Run("Task with subagent_type", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			makeUserTextEntry("refactor"),
			makeTaskEntry("code", "Refactor the module"),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		require.Len(t, summary.Agents, 1)
		assert.Equal(t, "code", summary.Agents[0].Type)
		assert.Equal(t, "Refactor the module", summary.Agents[0].Desc)
	})

	t.Run("multiple Task calls accumulate agents", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			makeUserTextEntry("do many things"),
			makeTaskEntry("general-purpose", "First task"),
			makeTaskEntry("code", "Second task"),
			makeTaskEntry("general-purpose", ""),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		require.Len(t, summary.Agents, 3)
		assert.Equal(t, "general-purpose", summary.Agents[0].Type)
		assert.Equal(t, "code", summary.Agents[1].Type)
		assert.Equal(t, "general-purpose", summary.Agents[2].Type)
	})

	t.Run("Task and TodoWrite in same session", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			makeUserTextEntry("work on it"),
			makeTaskEntry("code", "Implement feature"),
			makeTodoWriteEntry([]map[string]interface{}{
				{"status": "completed"},
				{"status": "pending"},
			}),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: both agent and todo should be extracted.
		require.Len(t, summary.Agents, 1)
		assert.Equal(t, "code", summary.Agents[0].Type)
		assert.Equal(t, 2, summary.TodoTotal)
		assert.Equal(t, 1, summary.TodoCompleted)
	})
}

// ---------------------------------------------------------------------------
// Tests for git branch extraction from entries
// ---------------------------------------------------------------------------

func TestGitBranchExtraction(t *testing.T) {
	t.Parallel()

	t.Run("git branch from entry is populated", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user", GitBranch: "feature-branch"},
			makeUserTextEntry("hello"),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, "feature-branch", summary.GitBranch)
	})

	t.Run("first non-empty git branch wins", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user", GitBranch: "main"},
			{Type: "assistant", GitBranch: "feature", Timestamp: "2026-01-01T00:00:00Z"},
			makeUserTextEntry("hello"),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: the first entry with a non-empty branch should win.
		assert.Equal(t, "main", summary.GitBranch)
	})

	t.Run("empty git branch entries are skipped", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user", GitBranch: ""},
			{Type: "user", GitBranch: "develop"},
			makeUserTextEntry("hello"),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, "develop", summary.GitBranch)
	})
}

// ---------------------------------------------------------------------------
// Tests for token accumulation
// ---------------------------------------------------------------------------

func TestTokenAccumulation(t *testing.T) {
	t.Parallel()

	t.Run("tokens from assistant entries are summed", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			makeUserTextEntry("question 1"),
			makeAssistantEntry(100, 50, 20, nil),
			makeAssistantEntry(200, 80, 30, nil),
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, 300, summary.InputTokens)
		assert.Equal(t, 130, summary.OutputTokens)
		assert.Equal(t, 50, summary.CacheTokens)
		assert.Equal(t, 430, summary.TotalTokens) // InputTokens + OutputTokens
	})

	t.Run("user entries do not contribute tokens", func(t *testing.T) {
		// Arrange: user entry with usage should not contribute.
		entries := []TranscriptEntry{
			{Type: "user", Message: &MessageContent{
				Usage: TokenUsage{InputTokens: 999, OutputTokens: 888, CacheReadInputTokens: 777},
			}},
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, 0, summary.InputTokens)
		assert.Equal(t, 0, summary.OutputTokens)
		assert.Equal(t, 0, summary.CacheTokens)
	})
}

// ---------------------------------------------------------------------------
// Tests for session timestamps
// ---------------------------------------------------------------------------

func TestSessionTimestamps(t *testing.T) {
	t.Parallel()

	t.Run("earliest and latest timestamps are captured", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user", Timestamp: "2026-01-01T10:00:00Z"},
			{Type: "assistant", Timestamp: "2026-01-01T10:05:00Z"},
			{Type: "user", Timestamp: "2026-01-01T10:10:00Z"},
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, "2026-01-01T10:00:00Z", summary.SessionStart.Format(time.RFC3339))
		assert.Equal(t, "2026-01-01T10:10:00Z", summary.SessionEnd.Format(time.RFC3339))
	})

	t.Run("entries without timestamps are skipped", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user"},
			{Type: "assistant", Timestamp: "2026-06-15T12:00:00Z"},
			{Type: "user"},
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert
		assert.Equal(t, "2026-06-15T12:00:00Z", summary.SessionStart.Format(time.RFC3339))
		assert.Equal(t, "2026-06-15T12:00:00Z", summary.SessionEnd.Format(time.RFC3339))
	})

	t.Run("invalid timestamps are skipped", func(t *testing.T) {
		// Arrange
		entries := []TranscriptEntry{
			{Type: "user", Timestamp: "not-a-valid-timestamp"},
			{Type: "assistant", Timestamp: "2026-06-15T12:00:00Z"},
		}

		// Act
		summary := analyzeTranscriptEntries(entries)

		// Assert: only the valid timestamp should be used.
		assert.Equal(t, "2026-06-15T12:00:00Z", summary.SessionStart.Format(time.RFC3339))
		assert.Equal(t, "2026-06-15T12:00:00Z", summary.SessionEnd.Format(time.RFC3339))
	})
}

// ---------------------------------------------------------------------------
// Tests for ParseTranscriptLastNLinesWithProjectPath — file I/O branches
// ---------------------------------------------------------------------------

// writeTestTranscriptFile creates a temp file with the given content and returns
// its path. The file is cleaned up when the test finishes.
func writeTestTranscriptFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "transcript-*.jsonl")
	require.NoError(t, err)
	path := f.Name()
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()
	t.Cleanup(func() { os.Remove(path) })
	return path
}

func TestParseTranscriptLastNLinesWithProjectPath_EmptyPath(t *testing.T) {
	clearTranscriptCache()

	summary, err := ParseTranscriptLastNLinesWithProjectPath("", 10, "/project")
	require.NoError(t, err)
	assert.Equal(t, &TranscriptSummary{}, summary)
}

func TestParseTranscriptLastNLinesWithProjectPath_NonexistentFile(t *testing.T) {
	clearTranscriptCache()

	summary, err := ParseTranscriptLastNLinesWithProjectPath("/nonexistent/file.jsonl", 10, "/project")
	require.NoError(t, err)
	assert.Equal(t, &TranscriptSummary{}, summary)
}

func TestParseTranscriptLastNLinesWithProjectPath_EmptyFile(t *testing.T) {
	clearTranscriptCache()

	f := writeTestTranscriptFile(t, "")

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Empty(t, summary.GitBranch)
	assert.Equal(t, 0, summary.InputTokens)
}

func TestParseTranscriptLastNLinesWithProjectPath_SingleUserMessage(t *testing.T) {
	clearTranscriptCache()

	content := `{"type":"user","message":{"role":"user","content":"hello"}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

func TestParseTranscriptLastNLinesWithProjectPath_MultipleUserMessages(t *testing.T) {
	clearTranscriptCache()

	content := `{"type":"user","message":{"role":"user","content":"first"}}
{"type":"assistant","message":{"role":"assistant","content":"response"}}
{"type":"user","message":{"role":"user","content":"second"}}
{"type":"user","message":{"role":"user","content":"third"}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

func TestParseTranscriptLastNLinesWithProjectPath_ToolResultNotUserMessage(t *testing.T) {
	// tool_result has type "user" but message.content is an array, not a string.
	// This means it is NOT a real user message boundary, so the current turn
	// should include all entries (startIdx stays 0).
	clearTranscriptCache()

	content := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"123","content":"result"}]}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

func TestParseTranscriptLastNLinesWithProjectPath_ProjectPathGitBranchFallback(t *testing.T) {
	// When no git branch is found in the transcript and projectPath is provided,
	// it falls back to getGitBranchForPath.
	clearTranscriptCache()

	content := `{"type":"assistant","message":{"role":"assistant","content":"hi"}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	oldRunner := defaultCommandRunner
	defaultCommandRunner = &stubCommandRunner{
		outputs: map[string][]byte{
			"git symbolic-ref --short HEAD": []byte("feature-branch\n"),
		},
	}
	defer func() { defaultCommandRunner = oldRunner }()

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "/some/project")
	require.NoError(t, err)
	assert.Equal(t, "feature-branch", summary.GitBranch)
}

func TestParseTranscriptLastNLinesWithProjectPath_TranscriptHasGitBranch(t *testing.T) {
	// When the transcript already has a git branch, the projectPath fallback
	// should NOT be used even if projectPath is provided.
	clearTranscriptCache()

	content := `{"type":"user","git_branch":"main","timestamp":"2026-01-01T00:00:00Z"}
{"type":"assistant","message":{"role":"assistant","content":"hi"}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	oldRunner := defaultCommandRunner
	defaultCommandRunner = &stubCommandRunner{}
	defer func() { defaultCommandRunner = oldRunner }()

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "/some/project")
	require.NoError(t, err)
	assert.Equal(t, "main", summary.GitBranch)
}

// ---------------------------------------------------------------------------
// Tests for readCurrentTurnEntries — backward scan branches
// ---------------------------------------------------------------------------

func TestReadCurrentTurnEntries_InvalidJSONDuringScan(t *testing.T) {
	// When a line contains "type":"user" but is not valid JSON,
	// json.Unmarshal fails and the line is skipped during backward scan.
	clearTranscriptCache()

	content := `{"type":"user","message":{"role":"user","content":"second"}}
{"type":"user","invalid-json-here
{"type":"assistant","message":{"role":"assistant","content":"response"}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

func TestReadCurrentTurnEntries_UserMessageFoundInBackwardScan(t *testing.T) {
	// When a real user message is found during backward scan, startIdx is set.
	// Only entries from that point onward should be included in tool tracking.
	clearTranscriptCache()

	// Layout: old tool call, then user message, then new tool call + result.
	// The backward scan should find the user message and set startIdx.
	content := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"old-id","name":"Read"}]}}
{"type":"user","message":{"role":"user","content":"do something new"}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"new-id","name":"Bash"}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"new-id","content":"done"}]}}` + "\n"
	f := writeTestTranscriptFile(t, content)

	summary, err := ParseTranscriptLastNLinesWithProjectPath(f, 10, "")
	require.NoError(t, err)
	assert.NotNil(t, summary)
	// Only the new tool (Bash) after the user message should be tracked.
	assert.Equal(t, 0, summary.CompletedTools["Read"],
		"Old tool call before user message should not be in current turn")
	assert.Equal(t, 1, summary.CompletedTools["Bash"],
		"New tool call after user message should be in current turn")
}

// ---------------------------------------------------------------------------
// Tests for analyzeTranscriptEntries — non-tool_result content item
// ---------------------------------------------------------------------------

func TestAnalyzeTranscriptEntries_NonToolResultContentInUserEntry(t *testing.T) {
	// When a user entry contains content items that are NOT tool_result
	// (e.g., a text item in an array), those items are skipped.
	t.Parallel()

	entries := []TranscriptEntry{
		makeUserTextEntry("hello"),
		makeToolUseEntry("id-1", "Bash"),
		// User entry with mixed content: text item + tool_result.
		{
			Type: "user",
			Message: &MessageContent{
				Content: json.RawMessage(`[
					{"type":"text","text":"some context"},
					{"type":"tool_result","tool_use_id":"id-1","content":"ok"}
				]`),
			},
		},
	}

	summary := analyzeTranscriptEntries(entries)

	// The text content item should be skipped; only tool_result processed.
	assert.Equal(t, 1, summary.CompletedTools["Bash"],
		"Tool result should be counted even when mixed with text items")
}

func TestAnalyzeTranscriptEntries_ToolResultWithUnknownToolID(t *testing.T) {
	// When a tool_result references an unknown tool ID (no matching tool_use),
	// it should be silently ignored.
	t.Parallel()

	entries := []TranscriptEntry{
		makeUserTextEntry("hello"),
		{
			Type: "user",
			Message: &MessageContent{
				Content: json.RawMessage(`[
					{"type":"tool_result","tool_use_id":"unknown-id","content":"ok"}
				]`),
			},
		},
	}

	summary := analyzeTranscriptEntries(entries)

	assert.Equal(t, 0, len(summary.CompletedTools),
		"Tool result with unknown ID should not produce any completed tools")
	assert.Equal(t, 0, len(summary.FailedTools))
	assert.Equal(t, 0, len(summary.ActiveTools))
}

func TestAnalyzeTranscriptEntries_ToolUseAndResultBeforeUserMessage(t *testing.T) {
	// Tool calls and results before the last user message should NOT be
	// counted in the current turn (they belong to a previous turn).
	t.Parallel()

	entries := []TranscriptEntry{
		makeToolUseEntry("old-1", "Read"),
		makeToolResultEntry("old-1", false),
		makeUserTextEntry("new request"),
	}

	summary := analyzeTranscriptEntries(entries)

	assert.Equal(t, 0, summary.CompletedTools["Read"],
		"Tool calls before the last user message should not be in current turn")
	assert.Equal(t, 0, summary.FailedTools["Read"])
}

// ---------------------------------------------------------------------------
// Tests for analyzeTranscriptEntries — multiple tool results for same ID
// ---------------------------------------------------------------------------

func TestAnalyzeTranscriptEntries_MultipleToolResultsSameID(t *testing.T) {
	// If somehow the same tool_use_id appears in multiple tool_results,
	// the second one should still count (e.g. both completed).
	t.Parallel()

	entries := []TranscriptEntry{
		makeToolUseEntry("id-1", "Read"),
		makeToolResultEntry("id-1", false),
		// Duplicate result (should still count)
		makeToolResultEntry("id-1", false),
	}

	summary := analyzeTranscriptEntries(entries)

	assert.Equal(t, 2, summary.CompletedTools["Read"],
		"Duplicate tool results for the same ID should both be counted")
}

// ---------------------------------------------------------------------------
// RealCommandRunner.Run – integration test
// ---------------------------------------------------------------------------

func TestRealCommandRunner_EchoCommand(t *testing.T) {
	runner := &RealCommandRunner{}
	out, err := runner.Run("", "echo", "hello")
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestRealCommandRunner_WithDir(t *testing.T) {
	runner := &RealCommandRunner{}
	out, err := runner.Run(t.TempDir(), "echo", "test")
	require.NoError(t, err)
	assert.Contains(t, string(out), "test")
}

func TestRealCommandRunner_NonexistentCommand(t *testing.T) {
	runner := &RealCommandRunner{}
	_, err := runner.Run("", "nonexistent_command_xyz_123")
	assert.Error(t, err)
}

func TestRealCommandRunner_EmptyDir(t *testing.T) {
	runner := &RealCommandRunner{}
	out, err := runner.Run("", "echo", "no-dir")
	require.NoError(t, err)
	assert.Contains(t, string(out), "no-dir")
}
