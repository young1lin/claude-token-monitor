package main

import (
	"testing"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/store"
	"github.com/young1lin/claude-token-monitor/tui"
)

// TestRunWatchLoopErrorsChannelCloseFirst tests the case where Errors channel closes first
func TestRunWatchLoopErrorsChannelCloseFirst(t *testing.T) {
	testWatcher := monitor.NewTestWatcher()

	// Create test database
	dbPath := t.TempDir() + "/test_errors_close.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	session := &monitor.SessionInfo{
		ID:       "test-errors-close",
		FilePath: "/fake/path.jsonl",
		Project:  "test-project",
	}
	history := []tui.HistoryEntry{}

	mockSender := &MockProgramSender{}

	done := make(chan bool)
	go func() {
		runWatchLoop(mockSender, testWatcher, db, session, history, false, []tui.ProjectInfo{})
		done <- true
	}()

	// Wait for initialization messages
	time.Sleep(50 * time.Millisecond)

	// Close ONLY the Errors channel, forcing that branch to execute
	testWatcher.CloseErrorsOnly()

	// Verify exit
	select {
	case <-done:
		t.Log("✅ runWatchLoop exited via Errors channel close")
	case <-time.After(1 * time.Second):
		t.Error("runWatchLoop did not exit after Errors channel close")
	}

	// Cleanup: close remaining channels
	testWatcher.Close()
}

// TestRunWatchLoopLinesChannelCloseFirst tests the case where Lines channel closes first
func TestRunWatchLoopLinesChannelCloseFirst(t *testing.T) {
	testWatcher := monitor.NewTestWatcher()

	dbPath := t.TempDir() + "/test_lines_close.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	session := &monitor.SessionInfo{
		ID:       "test-lines-close",
		FilePath: "/fake/path.jsonl",
		Project:  "test-project",
	}
	history := []tui.HistoryEntry{}

	mockSender := &MockProgramSender{}

	done := make(chan bool)
	go func() {
		runWatchLoop(mockSender, testWatcher, db, session, history, false, []tui.ProjectInfo{})
		done <- true
	}()

	time.Sleep(50 * time.Millisecond)

	// Close ONLY the Lines channel
	testWatcher.CloseLinesOnly()

	select {
	case <-done:
		t.Log("✅ runWatchLoop exited via Lines channel close")
	case <-time.After(1 * time.Second):
		t.Error("runWatchLoop did not exit after Lines channel close")
	}

	testWatcher.Close()
}
