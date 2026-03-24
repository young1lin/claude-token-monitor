package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GetSessionDuration
// ---------------------------------------------------------------------------

func TestGetSessionDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected string
	}{
		{
			name:     "zero start and end returns empty",
			start:    time.Time{},
			end:      time.Time{},
			expected: "",
		},
		{
			name:     "zero end returns empty",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Time{},
			expected: "",
		},
		{
			name:     "zero start returns empty",
			start:    time.Time{},
			end:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "",
		},
		{
			name:     "duration under one minute shows seconds",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 12, 0, 30, 0, time.UTC),
			expected: "30s",
		},
		{
			name:     "exactly one minute shows 1m",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC),
			expected: "1m",
		},
		{
			name:     "duration of 90 seconds shows 1m",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 12, 1, 30, 0, time.UTC),
			expected: "1m",
		},
		{
			name:     "duration under one hour shows minutes",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 12, 45, 0, 0, time.UTC),
			expected: "45m",
		},
		{
			name:     "exactly one hour shows 1h0m",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC),
			expected: "1h0m",
		},
		{
			name:     "one hour thirty minutes shows 1h30m",
			start:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 13, 30, 0, 0, time.UTC),
			expected: "1h30m",
		},
		{
			name:     "multiple hours with leftover minutes",
			start:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			end:      time.Date(2026, 1, 1, 14, 15, 0, 0, time.UTC),
			expected: "4h15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			summary := &TranscriptSummary{
				SessionStart: tt.start,
				SessionEnd:   tt.end,
			}

			// Act
			result := GetSessionDuration(summary)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// FormatActiveTools
// ---------------------------------------------------------------------------

func TestFormatActiveTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  *TranscriptSummary
		expected string
	}{
		{
			name:     "empty active tools returns empty string",
			summary:  &TranscriptSummary{ActiveTools: []string{}},
			expected: "",
		},
		{
			name:     "nil active tools returns empty string",
			summary:  &TranscriptSummary{ActiveTools: nil},
			expected: "",
		},
		{
			name:     "single tool returns tool name",
			summary:  &TranscriptSummary{ActiveTools: []string{"Read"}},
			expected: "Read",
		},
		{
			name:     "two tools returns comma separated",
			summary:  &TranscriptSummary{ActiveTools: []string{"Read", "Bash"}},
			expected: "Read,Bash",
		},
		{
			name:     "three tools returns comma separated",
			summary:  &TranscriptSummary{ActiveTools: []string{"Read", "Bash", "Grep"}},
			expected: "Read,Bash,Grep",
		},
		{
			name:     "more than three tools shows count",
			summary:  &TranscriptSummary{ActiveTools: []string{"Read", "Bash", "Grep", "Edit"}},
			expected: "4 tools",
		},
		{
			name:     "many tools shows count",
			summary:  &TranscriptSummary{ActiveTools: []string{"A", "B", "C", "D", "E", "F"}},
			expected: "6 tools",
		},
		{
			name:     "mcp tool name over 15 chars is truncated",
			summary:  &TranscriptSummary{ActiveTools: []string{"mcp__some_server__do_thing"}},
			expected: "mcp:some_ser..",
		},
		{
			name:     "mcp tool name at exactly 15 chars is not truncated",
			summary:  &TranscriptSummary{ActiveTools: []string{"mcp__abcdefghijk"}},
			expected: "mcp:abcdefghijk",
		},
		{
			name:     "long mcp tool name is truncated to 12 chars plus two dots",
			summary:  &TranscriptSummary{ActiveTools: []string{"mcp__very_long_server_name__very_long_tool_name"}},
			expected: "mcp:very_lon..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			result := FormatActiveTools(tt.summary)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// FormatTodoProgress
// ---------------------------------------------------------------------------

func TestFormatTodoProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  *TranscriptSummary
		expected string
	}{
		{
			name:     "zero total returns empty string",
			summary:  &TranscriptSummary{TodoTotal: 0, TodoCompleted: 0},
			expected: "",
		},
		{
			name:     "partial completion shows ratio without checkmark",
			summary:  &TranscriptSummary{TodoTotal: 10, TodoCompleted: 3},
			expected: "3/10",
		},
		{
			name:     "none completed shows ratio",
			summary:  &TranscriptSummary{TodoTotal: 5, TodoCompleted: 0},
			expected: "0/5",
		},
		{
			name:     "full completion shows ratio with checkmark",
			summary:  &TranscriptSummary{TodoTotal: 10, TodoCompleted: 10},
			expected: "\u2713 10/10",
		},
		{
			name:     "single todo completed",
			summary:  &TranscriptSummary{TodoTotal: 1, TodoCompleted: 1},
			expected: "\u2713 1/1",
		},
		{
			name:     "single todo not completed",
			summary:  &TranscriptSummary{TodoTotal: 1, TodoCompleted: 0},
			expected: "0/1",
		},
		{
			name:     "almost done still without checkmark",
			summary:  &TranscriptSummary{TodoTotal: 10, TodoCompleted: 9},
			expected: "9/10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			result := FormatTodoProgress(tt.summary)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// FormatAgentInfo
// ---------------------------------------------------------------------------

func TestFormatAgentInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  *TranscriptSummary
		expected string
	}{
		{
			name:     "empty agents returns empty string",
			summary:  &TranscriptSummary{Agents: []AgentInfo{}},
			expected: "",
		},
		{
			name:     "nil agents returns empty string",
			summary:  &TranscriptSummary{Agents: nil},
			expected: "",
		},
		{
			name: "single agent with type only",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{Type: "general-purpose"}},
			},
			expected: "general-purpose",
		},
		{
			name: "single agent with type and description",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{Type: "general-purpose", Desc: "Explore the codebase"}},
			},
			expected: "general-purpose: Explore the codebase",
		},
		{
			name: "multiple agents returns last agent info",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{
					{Type: "general-purpose", Desc: "First agent"},
					{Type: "code", Desc: "Second agent"},
				},
			},
			expected: "code: Second agent",
		},
		{
			name: "description over 20 runes is truncated to 17 runes plus two dots",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type: "general-purpose",
					Desc: "This is a very long description that exceeds twenty characters",
				}},
			},
			expected: "general-purpose: This is a very lo..",
		},
		{
			name: "description at exactly 20 runes is not truncated",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type: "general-purpose",
					Desc: "01234567890123456789", // exactly 20 runes
				}},
			},
			expected: "general-purpose: 01234567890123456789",
		},
		{
			name: "description with Chinese characters at 21 runes is truncated",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type: "general-purpose",
					Desc: "这是一个中文描述用来测试截断功能的正确性啊", // 21 runes
				}},
			},
			expected: "general-purpose: 这是一个中文描述用来测试截断功能的..",
		},
		{
			name: "agent with elapsed time under one hour shows duration",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type:    "general-purpose",
					Desc:    "Working",
					Elapsed: 120,
				}},
			},
			expected: "general-purpose: Working (2m)",
		},
		{
			name: "agent with elapsed time one second shows 1s",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type:    "general-purpose",
					Desc:    "Working",
					Elapsed: 1,
				}},
			},
			expected: "general-purpose: Working (1s)",
		},
		{
			name: "agent with zero elapsed does not append duration",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type:    "general-purpose",
					Desc:    "Working",
					Elapsed: 0,
				}},
			},
			expected: "general-purpose: Working",
		},
		{
			name: "agent with elapsed time over one hour omits duration",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{
					Type:    "general-purpose",
					Desc:    "Working",
					Elapsed: 7200, // 2 hours
				}},
			},
			expected: "general-purpose: Working",
		},
		{
			name: "agent with subagent_type from input",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{{Type: "code"}},
			},
			expected: "code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			result := FormatAgentInfo(tt.summary)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// FormatCompletedTools
// ---------------------------------------------------------------------------

func TestFormatCompletedTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  *TranscriptSummary
		expected string
	}{
		{
			name:     "empty completed tools returns empty string",
			summary:  &TranscriptSummary{CompletedTools: map[string]int{}},
			expected: "",
		},
		{
			name:     "nil completed tools returns empty string",
			summary:  &TranscriptSummary{CompletedTools: nil},
			expected: "",
		},
		{
			name: "single tool with one completion",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{"Read": 1},
			},
			expected: "1 tools",
		},
		{
			name: "single tool with multiple completions",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{"Read": 5},
			},
			expected: "5 tools",
		},
		{
			name: "multiple tools with varying counts",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Read": 3,
					"Bash": 2,
					"Grep": 1,
				},
			},
			expected: "6 tools",
		},
		{
			name: "large count",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{"Bash": 100},
			},
			expected: "100 tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			result := FormatCompletedTools(tt.summary)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// GetProjectName
// ---------------------------------------------------------------------------

func TestGetProjectName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cwd        string
		projectDir string
		expected   string
	}{
		{
			name:       "simple directory name",
			cwd:        "/home/user/myproject",
			projectDir: "",
			expected:   "myproject",
		},
		{
			name:       "projectDir overrides cwd",
			cwd:        "/home/user/other",
			projectDir: "/home/user/myproject",
			expected:   "myproject",
		},
		{
			name:       "empty cwd uses projectDir",
			cwd:        "",
			projectDir: "/home/user/myproject",
			expected:   "myproject",
		},
		{
			name:       "both empty returns empty string",
			cwd:        "",
			projectDir: "",
			expected:   "",
		},
		{
			name:       "windows path with backslashes",
			cwd:        `C:\Users\dev\minimal-mcp`,
			projectDir: "",
			expected:   "minimal-mcp",
		},
		{
			name:       "windows path with forward slashes",
			cwd:        "C:/Users/dev/minimal-mcp",
			projectDir: "",
			expected:   "minimal-mcp",
		},
		{
			name:       "root directory returns empty string last part",
			cwd:        "/",
			projectDir: "",
			expected:   "",
		},
		{
			name:       "name over 20 chars is truncated",
			cwd:        "/home/user/this-is-a-very-long-project-directory-name",
			projectDir: "",
			expected:   "this-is-a-very-lo..",
		},
		{
			name:       "name exactly 20 chars is not truncated",
			cwd:        "/home/user/exactly-twenty-ch",
			projectDir: "",
			expected:   "exactly-twenty-ch",
		},
		{
			name:       "name 21 chars is truncated",
			cwd:        "/home/user/exactly-twenty-charxx",
			projectDir: "",
			expected:   "exactly-twenty-ch..",
		},
		{
			name:       "nested deep path returns last component",
			cwd:        "/a/b/c/d/e/f/g/project",
			projectDir: "",
			expected:   "project",
		},
		{
			name:       "relative path returns last component",
			cwd:        "src/internal/parser",
			projectDir: "",
			expected:   "parser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			result := GetProjectName(tt.cwd, tt.projectDir)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// formatElapsedDuration (unexported, tested indirectly through FormatAgentInfo
// and directly here since we are in the same package).
// ---------------------------------------------------------------------------

func TestFormatElapsedDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration returns less than 1s",
			duration: 0,
			expected: "<1s",
		},
		{
			name:     "sub-second duration returns less than 1s",
			duration: 500 * time.Millisecond,
			expected: "<1s",
		},
		{
			name:     "exactly one second returns 1s",
			duration: 1 * time.Second,
			expected: "1s",
		},
		{
			name:     "30 seconds returns 30s",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "exactly one minute returns 1m",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "90 seconds returns 1m",
			duration: 90 * time.Second,
			expected: "1m",
		},
		{
			name:     "45 minutes returns 45m",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "one hour returns 1h0m",
			duration: 1 * time.Hour,
			expected: "1h0m",
		},
		{
			name:     "one hour thirty minutes returns 1h30m",
			duration: 1*time.Hour + 30*time.Minute,
			expected: "1h30m",
		},
		{
			name:     "three hours fifteen minutes returns 3h15m",
			duration: 3*time.Hour + 15*time.Minute,
			expected: "3h15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := formatElapsedDuration(tt.duration)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge-case combined tests
// ---------------------------------------------------------------------------

func TestFormatFunctionsNilSafety(t *testing.T) {
	t.Parallel()

	// Ensure all formatter functions handle a nil summary gracefully.
	// This is a defensive test: the code may or may not dereference nil,
	// but if it does, the test catches a nil pointer dereference panic.
	t.Run("GetSessionDuration nil summary panics", func(t *testing.T) {
		require.Panics(t, func() {
			GetSessionDuration(nil)
		})
	})

	t.Run("FormatActiveTools nil summary panics", func(t *testing.T) {
		require.Panics(t, func() {
			FormatActiveTools(nil)
		})
	})

	t.Run("FormatTodoProgress nil summary panics", func(t *testing.T) {
		require.Panics(t, func() {
			FormatTodoProgress(nil)
		})
	})

	t.Run("FormatAgentInfo nil summary panics", func(t *testing.T) {
		require.Panics(t, func() {
			FormatAgentInfo(nil)
		})
	})

	t.Run("FormatCompletedTools nil summary panics", func(t *testing.T) {
		require.Panics(t, func() {
			FormatCompletedTools(nil)
		})
	})
}
