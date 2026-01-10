package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	model := NewModel(false)

	if model.ready != false {
		t.Errorf("NewModel().ready = %v, want false", model.ready)
	}

	if model.quitting != false {
		t.Errorf("NewModel().quitting = %v, want false", model.quitting)
	}

	if model.history == nil {
		t.Error("NewModel().history should be initialized, got nil")
	}

	if cap(model.history) != 10 {
		t.Errorf("NewModel().history capacity = %d, want 10", cap(model.history))
	}

	if len(model.history) != 0 {
		t.Errorf("NewModel().history length = %d, want 0", len(model.history))
	}
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()

	// Test that all styles are properly initialized by checking they can render
	// We can't compare lipgloss.Style directly as it contains functions
	testString := "test"
	rendered := styles.Border.Render(testString)
	if rendered == "" {
		t.Error("DefaultStyles().Border should render something")
	}

	rendered = styles.Header.Render(testString)
	if rendered == "" {
		t.Error("DefaultStyles().Header should render something")
	}

	rendered = styles.Title.Render(testString)
	if rendered == "" {
		t.Error("DefaultStyles().Title should render something")
	}

	rendered = styles.Error.Render(testString)
	if rendered == "" {
		t.Error("DefaultStyles().Error should render something")
	}

	rendered = styles.Cost.Render(testString)
	if rendered == "" {
		t.Error("DefaultStyles().Cost should render something")
	}
}

func TestModelInit(t *testing.T) {
	model := NewModel(false)
	cmd := model.Init()

	if cmd != nil {
		t.Error("Model.Init() should return nil, got non-nil")
	}
}

func TestModelUpdate(t *testing.T) {
	model := NewModel(false)

	// Test with a simple tick message
	newModel, cmd := model.Update(TickMsg{Time: "12:00:00"})

	// TickMsg returns tickCmd(), not nil
	if cmd == nil {
		t.Error("Update(TickMsg) should return tickCmd, got nil")
	}

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.quitting != false {
		t.Error("Update(TickMsg) should not set quitting to true")
	}

	if m.lastUpdate != "12:00:00" {
		t.Errorf("Update(TickMsg).lastUpdate = %s, want '12:00:00'", m.lastUpdate)
	}
}

func TestModelUpdateQuitKey(t *testing.T) {
	model := NewModel(false)

	// Test quit key message (ctrl+c)
	quitMsg := tea.KeyMsg{
		Type: tea.KeyCtrlC,
	}

	newModel, cmd := model.Update(quitMsg)

	if cmd == nil {
		t.Error("Update(quit key) should return tea.Quit cmd, got nil")
	}

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.quitting != true {
		t.Error("Update(quit key) should set quitting to true")
	}
}

func TestModelUpdateCtrlC(t *testing.T) {
	model := NewModel(false)

	// Test ctrl+c message
	ctrlcMsg := tea.KeyMsg{
		Type: tea.KeyCtrlC,
	}

	newModel, cmd := model.Update(ctrlcMsg)

	if cmd == nil {
		t.Error("Update(ctrl+c) should return tea.Quit cmd, got nil")
	}

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.quitting != true {
		t.Error("Update(ctrl+c) should set quitting to true")
	}
}

func TestModelUpdateTokenUpdate(t *testing.T) {
	model := NewModel(false)
	model.ready = true // TokenUpdateMsg doesn't change ready state

	msg := TokenUpdateMsg{
		SessionID:    "test-session",
		Model:        "Sonnet 4.5",
		InputTokens:  1000,
		OutputTokens: 500,
		CacheTokens:  200,
		TotalTokens:  1500,
		Cost:         0.05,
		ContextPct:   7.5,
	}

	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.sessionID != "test-session" {
		t.Errorf("Update(TokenUpdateMsg).sessionID = %s, want 'test-session'", m.sessionID)
	}

	if m.model != "Sonnet 4.5" {
		t.Errorf("Update(TokenUpdateMsg).model = %s, want 'Sonnet 4.5'", m.model)
	}

	if m.totalTokens != 1500 {
		t.Errorf("Update(TokenUpdateMsg).totalTokens = %d, want 1500", m.totalTokens)
	}

	if m.cost != 0.05 {
		t.Errorf("Update(TokenUpdateMsg).cost = %f, want 0.05", m.cost)
	}

	if m.contextPct != 7.5 {
		t.Errorf("Update(TokenUpdateMsg).contextPct = %f, want 7.5", m.contextPct)
	}

	// TokenUpdateMsg preserves ready state, doesn't set it to true
	if m.ready != true {
		t.Error("Update(TokenUpdateMsg).ready should preserve existing state")
	}
}

func TestModelUpdateHistoryLoaded(t *testing.T) {
	model := NewModel(false)

	history := []HistoryEntry{
		{ID: "1", Timestamp: "2024-01-01 12:00", Tokens: 1000, Cost: 0.01, Project: "test"},
		{ID: "2", Timestamp: "2024-01-01 13:00", Tokens: 2000, Cost: 0.02, Project: "test"},
	}

	msg := HistoryLoadedMsg{History: history}
	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if len(m.history) != 2 {
		t.Errorf("Update(HistoryLoadedMsg).history length = %d, want 2", len(m.history))
	}

	if m.history[0].ID != "1" {
		t.Errorf("Update(HistoryLoadedMsg).history[0].ID = %s, want '1'", m.history[0].ID)
	}
}

func TestModelUpdateSessionFound(t *testing.T) {
	model := NewModel(false)

	msg := SessionFoundMsg{
		SessionID: "session-123",
		Project:   "my-project",
	}

	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.sessionID != "session-123" {
		t.Errorf("Update(SessionFoundMsg).sessionID = %s, want 'session-123'", m.sessionID)
	}

	if m.project != "my-project" {
		t.Errorf("Update(SessionFoundMsg).project = %s, want 'my-project'", m.project)
	}
}

func TestModelUpdateErrorMsg(t *testing.T) {
	model := NewModel(false)

	testErr := &TestError{msg: "test error"}
	msg := ErrorMsg{Err: testErr}

	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.err != testErr {
		t.Errorf("Update(ErrorMsg).err = %v, want %v", m.err, testErr)
	}
}

func TestModelUpdateWatcherStarted(t *testing.T) {
	model := NewModel(false)

	newModel, _ := model.Update(WatcherStartedMsg{})

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	// WatcherStarted should set ready to true
	if m.ready != true {
		t.Error("Update(WatcherStartedMsg).ready should be true")
	}
}

func TestModelUpdateWatcherFailed(t *testing.T) {
	model := NewModel(false)

	testErr := &TestError{msg: "watcher failed"}
	msg := WatcherFailedMsg{Err: testErr}

	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update() should return a Model")
	}

	if m.err != testErr {
		t.Errorf("Update(WatcherFailedMsg).err = %v, want %v", m.err, testErr)
	}
}

// TestError is a simple error implementation for testing
type TestError struct {
	msg string
}

func (e *TestError) Error() string {
	return e.msg
}
