package tui

import (
	"testing"
)

func TestViewQuitting(t *testing.T) {
	model := NewModel(false)
	model.quitting = true

	view := model.View()

	if view != "Goodbye!\n" {
		t.Errorf("View() when quitting = %s, want 'Goodbye!\\n'", view)
	}
}

func TestViewError(t *testing.T) {
	model := NewModel(false)
	model.err = &TestError{msg: "test error"}

	view := model.View()

	if view == "" {
		t.Error("View() with error should not be empty")
	}

	// Should contain the error message
	if !contains(view, "Error:") {
		t.Error("View() with error should contain 'Error:'")
	}
}

func TestViewLoading(t *testing.T) {
	model := NewModel(false)
	// Not ready, no error
	model.ready = false
	model.err = nil

	view := model.View()

	if view == "" {
		t.Error("View() when loading should not be empty")
	}

	// Should contain loading text
	if !contains(view, "Loading") {
		t.Error("View() when loading should contain 'Loading'")
	}
}

func TestViewReady(t *testing.T) {
	model := NewModel(false)
	model.ready = true
	model.sessionID = "test-session-id-12345"
	model.model = "Sonnet 4.5"
	model.inputTokens = 1000
	model.outputTokens = 500
	model.cacheTokens = 200
	model.totalTokens = 1500
	model.contextPct = 7.5
	model.cost = 0.05

	view := model.View()

	if view == "" {
		t.Error("View() when ready should not be empty")
	}

	// Should contain header
	if !contains(view, "Claude Token Monitor") {
		t.Error("View() should contain 'Claude Token Monitor'")
	}
}

func TestViewWithHistory(t *testing.T) {
	model := NewModel(false)
	model.ready = true
	model.history = []HistoryEntry{
		{ID: "1", Timestamp: "2024-01-01 12:00", Tokens: 1000, Cost: 0.01, Project: "test"},
		{ID: "2", Timestamp: "2024-01-01 13:00", Tokens: 2000, Cost: 0.02, Project: "test"},
	}

	view := model.View()

	if !contains(view, "Recent Sessions") {
		t.Error("View() with history should contain 'Recent Sessions'")
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"zero", 0, "0"},
		{"small", 999, "999"},
		{"thousand", 1234, "1.2K"},
		{"exact thousand", 1000, "1.0K"},
		{"million", 1234567, "1.2M"},
		{"exact million", 1000000, "1.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatNumber(tt.input); got != tt.want {
				t.Errorf("formatNumber(%d) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"zero", 0.0, "$0.0000"},
		{"small", 0.001, "$0.0010"},
		{"dollar", 1.0, "$1.00"},
		{"large", 10.5, "$10.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCost(tt.input); got != tt.want {
				t.Errorf("formatCost(%f) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestModelUpdateReturnsModel(t *testing.T) {
	model := NewModel(false)

	// Test that Update returns a Model that can be type-asserted
	newModel, _ := model.Update(TickMsg{Time: "12:00:00"})

	// Should be able to assert to Model
	if _, ok := newModel.(Model); !ok {
		t.Error("Update() should return a Model")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
