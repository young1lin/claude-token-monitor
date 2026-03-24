package content

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentCollector_Collect(t *testing.T) {
	collector := NewAgentCollector()

	tests := []struct {
		name    string
		summary *TranscriptSummary
		want    string
		wantErr bool
	}{
		{
			name: "no agents returns empty",
			summary: &TranscriptSummary{
				Agents: nil,
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "empty agents slice returns empty",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "single agent without description",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{
					{Type: "Explore"},
				},
			},
			want:    "\U0001f916 Explore",
			wantErr: false,
		},
		{
			name: "single agent with description",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{
					{Type: "Code", Desc: "writing tests"},
				},
			},
			want:    "\U0001f916 Code: writing tests",
			wantErr: false,
		},
		{
			name: "agent with long description gets truncated",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{
					{Type: "Code", Desc: "this is a very long description that exceeds twenty characters"},
				},
			},
			want:    "\U0001f916 Code: this is a very lo..",
			wantErr: false,
		},
		{
			name: "multiple agents returns last one",
			summary: &TranscriptSummary{
				Agents: []AgentInfo{
					{Type: "Explore"},
					{Type: "Code", Desc: "refactor"},
				},
			},
			want:    "\U0001f916 Code: refactor",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(nil, tt.summary)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAgentCollector_Collect_InvalidSummary(t *testing.T) {
	collector := NewAgentCollector()

	// Act
	_, err := collector.Collect(nil, "invalid")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary type")
}

func TestTodoCollector_Collect(t *testing.T) {
	collector := NewTodoCollector()

	tests := []struct {
		name    string
		summary *TranscriptSummary
		want    string
		wantErr bool
	}{
		{
			name: "zero total returns empty",
			summary: &TranscriptSummary{
				TodoTotal:     0,
				TodoCompleted: 0,
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "partial progress",
			summary: &TranscriptSummary{
				TodoTotal:     10,
				TodoCompleted: 3,
			},
			want:    "\U0001f4cb 3/10",
			wantErr: false,
		},
		{
			name: "full completion shows checkmark",
			summary: &TranscriptSummary{
				TodoTotal:     5,
				TodoCompleted: 5,
			},
			want:    "\U0001f4cb \u2713 5/5",
			wantErr: false,
		},
		{
			name: "no completed items",
			summary: &TranscriptSummary{
				TodoTotal:     8,
				TodoCompleted: 0,
			},
			want:    "\U0001f4cb 0/8",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(nil, tt.summary)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoCollector_Collect_InvalidSummary(t *testing.T) {
	collector := NewTodoCollector()

	// Act
	_, err := collector.Collect(nil, "invalid")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary type")
}

func TestToolsCollector_Collect(t *testing.T) {
	collector := NewToolsCollector()

	tests := []struct {
		name    string
		summary *TranscriptSummary
		want    string
		wantErr bool
	}{
		{
			name: "no tools returns empty",
			summary: &TranscriptSummary{
				CompletedTools: nil,
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "empty tools map returns empty",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "single tool",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Read": 5,
				},
			},
			want:    "\U0001f527 5 tools",
			wantErr: false,
		},
		{
			name: "multiple tools with counts",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Read":  10,
					"Write": 3,
					"Bash":  7,
				},
			},
			want:    "\U0001f527 20 tools",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(nil, tt.summary)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolsCollector_Collect_InvalidSummary(t *testing.T) {
	collector := NewToolsCollector()

	// Act
	_, err := collector.Collect(nil, 123)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary type")
}

func TestSessionDurationCollector_Collect(t *testing.T) {
	collector := NewSessionDurationCollector()

	tests := []struct {
		name    string
		summary *TranscriptSummary
		want    string
		wantErr bool
	}{
		{
			name: "zero start time returns empty",
			summary: &TranscriptSummary{
				SessionStart: time.Time{},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "with start but no end",
			summary: &TranscriptSummary{
				SessionStart: time.Now().Add(-5 * time.Minute),
				SessionEnd:   time.Time{},
			},
			want:    fmt.Sprintf("\u23f1\ufe0f %s", formatDuration(5*time.Minute)),
			wantErr: false,
		},
		{
			name: "with start and end",
			summary: &TranscriptSummary{
				SessionStart: time.Now().Add(-1 * time.Hour).Add(-30 * time.Minute),
				SessionEnd:   time.Now().Add(-30 * time.Minute),
			},
			want:    fmt.Sprintf("\u23f1\ufe0f %s", formatDuration(1*time.Hour)),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(nil, tt.summary)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// For "with start but no end", the duration is relative to now,
				// so just check the prefix
				if tt.summary.SessionEnd.IsZero() && !tt.summary.SessionStart.IsZero() {
					assert.Contains(t, got, "\u23f1\ufe0f ")
				} else {
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}

func TestSessionDurationCollector_Collect_InvalidSummary(t *testing.T) {
	collector := NewSessionDurationCollector()

	// Act
	_, err := collector.Collect(nil, "invalid")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary type")
}

func TestToolStatusDetailCollector_Collect(t *testing.T) {
	collector := NewToolStatusDetailCollector()

	tests := []struct {
		name    string
		summary *TranscriptSummary
		want    string
		wantErr bool
	}{
		{
			name: "no tools returns empty",
			summary: &TranscriptSummary{
				CompletedTools: nil,
				FailedTools:    nil,
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "only successful tools",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Read":  10,
					"Bash":  5,
					"Write": 3,
				},
				FailedTools: nil,
			},
			wantErr: false,
		},
		{
			name: "only failed tools",
			summary: &TranscriptSummary{
				CompletedTools: nil,
				FailedTools: map[string]int{
					"Read": 1,
					"Exec": 2,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed successful and failed tools",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Read": 10,
					"Bash": 5,
				},
				FailedTools: map[string]int{
					"Write": 2,
				},
			},
			wantErr: false,
		},
		{
			name: "sorted by count descending then name ascending",
			summary: &TranscriptSummary{
				CompletedTools: map[string]int{
					"Bash":  10,
					"Read":  5,
					"Agent": 10,
				},
				FailedTools: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := collector.Collect(nil, tt.summary)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.want != "" {
					assert.Equal(t, tt.want, got)
				} else if tt.summary.CompletedTools == nil && tt.summary.FailedTools == nil {
					assert.Empty(t, got)
				} else {
					// Just verify non-empty for non-specified output
					assert.NotEmpty(t, got)
				}
			}
		})
	}

	// Test sorted output more precisely
	t.Run("sorted output verification", func(t *testing.T) {
		// Arrange - Agent(10) and Bash(10) should both come first (same count, alphabetical),
		// then Read(5)
		summary := &TranscriptSummary{
			CompletedTools: map[string]int{
				"Bash":  10,
				"Read":  5,
				"Agent": 10,
			},
		}

		// Act
		got, err := collector.Collect(nil, summary)

		// Assert
		require.NoError(t, err)
		// Agent comes before Bash alphabetically when counts are equal
		assert.Contains(t, got, "\x1b[1;32m\u2713\x1b[0m Agent(10)")
		assert.Contains(t, got, "\x1b[1;32m\u2713\x1b[0m Bash(10)")
		assert.Contains(t, got, "\x1b[1;32m\u2713\x1b[0m Read(5)")

		// Agent should appear before Bash in the output
		agentIdx := findSubstringIndex(got, "Agent(10)")
		bashIdx := findSubstringIndex(got, "Bash(10)")
		require.NotEqual(t, -1, agentIdx)
		require.NotEqual(t, -1, bashIdx)
		assert.Less(t, agentIdx, bashIdx, "Agent should come before Bash (same count, alphabetical)")
	})
}

func TestToolStatusDetailCollector_Collect_InvalidSummary(t *testing.T) {
	collector := NewToolStatusDetailCollector()

	// Act
	_, err := collector.Collect(nil, "invalid")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary type")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero duration", d: 0, want: "0s"},
		{name: "30 seconds", d: 30 * time.Second, want: "30s"},
		{name: "59 seconds", d: 59 * time.Second, want: "59s"},
		{name: "1 minute", d: 1 * time.Minute, want: "1m"},
		{name: "5 minutes", d: 5 * time.Minute, want: "5m"},
		{name: "59 minutes", d: 59 * time.Minute, want: "59m"},
		{name: "1 hour", d: 1 * time.Hour, want: "1h0m"},
		{name: "1 hour 30 minutes", d: 1*time.Hour + 30*time.Minute, want: "1h30m"},
		{name: "3 hours 45 minutes", d: 3*time.Hour + 45*time.Minute, want: "3h45m"},
		{name: "24 hours", d: 24 * time.Hour, want: "24h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatDuration(tt.d)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// findSubstringIndex returns the index of a substring in s, or -1 if not found.
func findSubstringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestToolStatusDetailCollector_EmptyMaps(t *testing.T) {
	collector := NewToolStatusDetailCollector()

	// Both maps are empty (not nil)
	got, err := collector.Collect(nil, &TranscriptSummary{
		CompletedTools: map[string]int{},
		FailedTools:    map[string]int{},
	})

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestToolStatusDetailCollector_FailedToolsSorted(t *testing.T) {
	// Arrange: multiple failed tools with same count should be sorted alphabetically
	collector := NewToolStatusDetailCollector()
	summary := &TranscriptSummary{
		FailedTools: map[string]int{
			"Exec":  3,
			"Read":  3,
			"Bash":  5,
			"Agent": 1,
		},
	}

	// Act
	got, err := collector.Collect(nil, summary)

	// Assert
	require.NoError(t, err)
	// Bash(5) should come first (highest count)
	execIdx := findSubstringIndex(got, "Exec(3)")
	readIdx := findSubstringIndex(got, "Read(3)")
	bashIdx := findSubstringIndex(got, "Bash(5)")
	require.NotEqual(t, -1, bashIdx)
	require.NotEqual(t, -1, execIdx)
	require.NotEqual(t, -1, readIdx)
	// Bash(5) first, then Exec and Read (same count=3, alphabetical)
	assert.Less(t, bashIdx, execIdx, "Bash(5) should come before Exec(3)")
	assert.Less(t, execIdx, readIdx, "Exec(3) should come before Read(3) alphabetically")
}

func TestToolStatusDetailCollector_MixedWithANSICodes(t *testing.T) {
	// Verify output contains ANSI color codes
	collector := NewToolStatusDetailCollector()
	summary := &TranscriptSummary{
		CompletedTools: map[string]int{"Read": 1},
		FailedTools:    map[string]int{"Bash": 2},
	}

	got, err := collector.Collect(nil, summary)
	require.NoError(t, err)
	assert.Contains(t, got, "\x1b[1;32m") // green for success
	assert.Contains(t, got, "\x1b[1;31m") // red for failure
}
