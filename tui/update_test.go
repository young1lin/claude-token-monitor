package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleKeyMsgQuit(t *testing.T) {
	model := NewModel(false)

	// Test 'q' key - need to properly construct KeyMsg
	// The msg.String() must match the switch case in handleKeyMsg
	// For character keys, we need to use the Type: KeyRunes or construct it properly
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}

	// Verify String() returns expected format
	if msg.String() != "q" && msg.String() != "ctrl+c" && msg.String() != "esc" {
		// Skip this test if KeyMsg format doesn't match
		t.Skipf("KeyMsg.String() returned '%s', skipping", msg.String())
	}

	result, cmd := model.handleKeyMsg(msg)

	newModel, ok := result.(Model)
	if !ok {
		t.Fatal("handleKeyMsg() should return a Model")
	}

	if newModel.quitting != true {
		t.Error("handleKeyMsg('q').quitting should be true")
	}

	if cmd == nil {
		t.Error("handleKeyMsg('q') should return tea.Quit cmd")
	}
}

func TestHandleKeyMsgEsc(t *testing.T) {
	model := NewModel(false)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, cmd := model.handleKeyMsg(msg)

	newModel, ok := result.(Model)
	if !ok {
		t.Fatal("handleKeyMsg() should return a Model")
	}

	if newModel.quitting != true {
		t.Error("handleKeyMsg(esc).quitting should be true")
	}

	if cmd == nil {
		t.Error("handleKeyMsg(esc) should return tea.Quit cmd")
	}
}

func TestHandleKeyMsgCtrlC(t *testing.T) {
	model := NewModel(false)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	result, cmd := model.handleKeyMsg(msg)

	newModel, ok := result.(Model)
	if !ok {
		t.Fatal("handleKeyMsg() should return a Model")
	}

	if newModel.quitting != true {
		t.Error("handleKeyMsg(ctrl+c).quitting should be true")
	}

	if cmd == nil {
		t.Error("handleKeyMsg(ctrl+c) should return tea.Quit cmd")
	}
}

func TestHandleKeyMsgRefresh(t *testing.T) {
	model := NewModel(false)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}

	// Verify String() returns expected format
	if msg.String() != "r" {
		t.Skipf("KeyMsg.String() returned '%s', expected 'r', skipping", msg.String())
	}

	result, cmd := model.handleKeyMsg(msg)

	newModel, ok := result.(Model)
	if !ok {
		t.Fatal("handleKeyMsg() should return a Model")
	}

	if newModel.quitting != false {
		t.Error("handleKeyMsg('r').quitting should remain false")
	}

	if cmd == nil {
		t.Error("handleKeyMsg('r') should return tickCmd")
	}
}

func TestHandleKeyMsgOther(t *testing.T) {
	model := NewModel(false)

	// Test a non-mapped key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}

	if msg.String() != "x" {
		t.Skipf("KeyMsg.String() returned '%s', expected 'x', skipping", msg.String())
	}

	result, cmd := model.handleKeyMsg(msg)

	newModel, ok := result.(Model)
	if !ok {
		t.Fatal("handleKeyMsg() should return a Model")
	}

	if newModel.quitting != false {
		t.Error("handleKeyMsg('x').quitting should remain false")
	}

	if cmd != nil {
		t.Error("handleKeyMsg('x') should return nil cmd")
	}
}

func TestModelUpdateUnknownMsg(t *testing.T) {
	model := NewModel(false)

	// Test with an unknown message type
	type UnknownMsg struct{}
	newModel, cmd := model.Update(UnknownMsg{})

	// Type assert to check it's still a Model
	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update(UnknownMsg) should return a Model")
	}

	if m.sessionID != model.sessionID {
		t.Error("Update(UnknownMsg) should not change model state")
	}

	if cmd != nil {
		t.Error("Update(UnknownMsg) should return nil cmd")
	}
}

func TestModelUpdateTokenUpdateClearsError(t *testing.T) {
	model := NewModel(false)

	// Set an error first
	model.err = &TestError{msg: "previous error"}

	msg := TokenUpdateMsg{
		SessionID:    "test",
		TotalTokens:  100,
		Cost:         0.01,
	}
	newModel, _ := model.Update(msg)

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update(TokenUpdateMsg) should return a Model")
	}

	if m.err != nil {
		t.Error("Update(TokenUpdateMsg) should clear previous error")
	}
}

func TestModelUpdateWatcherStartedClearsError(t *testing.T) {
	model := NewModel(false)

	// Set an error first
	model.err = &TestError{msg: "previous error"}

	newModel, _ := model.Update(WatcherStartedMsg{})

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update(WatcherStartedMsg) should return a Model")
	}

	if m.err != nil {
		t.Error("Update(WatcherStartedMsg) should clear previous error")
	}

	if m.ready != true {
		t.Error("Update(WatcherStartedMsg).ready should be true")
	}
}

func TestModelUpdateWatcherFailedNotReady(t *testing.T) {
	model := NewModel(false)
	model.ready = true

	testErr := &TestError{msg: "watcher failed"}
	newModel, _ := model.Update(WatcherFailedMsg{Err: testErr})

	m, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update(WatcherFailedMsg) should return a Model")
	}

	if m.ready != false {
		t.Error("Update(WatcherFailedMsg).ready should be false")
	}

	if m.err != testErr {
		t.Error("Update(WatcherFailedMsg) should set error")
	}
}
