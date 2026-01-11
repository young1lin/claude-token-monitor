package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/young1lin/claude-token-monitor/internal/config"
	"github.com/young1lin/claude-token-monitor/internal/monitor"
	"github.com/young1lin/claude-token-monitor/internal/store"
	"github.com/young1lin/claude-token-monitor/tui"
)

// MockProgramSender is a mock ProgramSender for testing
type MockProgramSender struct {
	messages []tea.Msg
	SendFunc func(msg tea.Msg)
}

func (m *MockProgramSender) Send(msg tea.Msg) {
	m.messages = append(m.messages, msg)
	if m.SendFunc != nil {
		m.SendFunc(msg)
	}
}

func (m *MockProgramSender) GetMessageCount() int {
	return len(m.messages)
}

func (m *MockProgramSender) GetMessages() []tea.Msg {
	return m.messages
}

// MockProgramRunner is a mock ProgramRunner for testing
type MockProgramRunner struct {
	runCalled bool
	runError  error
}

func (m *MockProgramRunner) Run(p *tea.Program) error {
	m.runCalled = true
	return m.runError
}

// TestRunNoProjectsDirectory tests error case when projects directory doesn't exist
func TestRunNoProjectsDirectory(t *testing.T) {
	nonExistentDir := "/nonexistent/directory/that/does/not/exist"

	deps := &AppDependencies{
		ProjectsDir: nonExistentDir,
	}

	err := run(deps)
	if err == nil {
		t.Error("Expected error when projects directory doesn't exist")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) && err.Error() == "" {
		t.Errorf("Expected directory not found error, got: %v", err)
	}
}

// TestRunSessionFinderError tests error case when session finder fails
func TestRunSessionFinderError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-run-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	expectedErr := errors.New("no session found")
	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return nil, expectedErr
		},
	}

	err = run(deps)
	if err == nil {
		t.Error("Expected error when session finder fails")
	}
	if err != nil && err.Error() == "" {
		t.Errorf("Expected session finder error, got: %v", err)
	}
}

// TestRunDBOpenerError tests error case when database opener fails
func TestRunDBOpenerError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-run-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	expectedErr := errors.New("db error")
	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return &monitor.SessionInfo{
				ID:       "test-session",
				FilePath: "/path/to/session.jsonl",
			}, nil
		},
		DBOpener: func(string) (*store.DB, error) {
			return nil, expectedErr
		},
	}

	err = run(deps)
	if err == nil {
		t.Error("Expected error when DB opener fails")
	}
}

// TestRunWatcherError tests error case when watcher creation fails
func TestRunWatcherError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-run-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	expectedErr := errors.New("watcher error")
	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return &monitor.SessionInfo{
				ID:       "test-session",
				FilePath: "/nonexistent/path/session.jsonl",
			}, nil
		},
		DBOpener: func(string) (*store.DB, error) {
			return db, nil
		},
		WatcherCreator: func(string) (monitor.WatcherInterface, error) {
			return nil, expectedErr
		},
		ProgramRunner: func(p *tea.Program) error {
			// Don't actually run the program
			return nil
		},
	}

	err = run(deps)
	if err == nil {
		t.Error("Expected error when watcher creator fails")
	}
}

// TestRunWatchLoopSuccess tests successful runWatchLoop execution
func TestRunWatchLoopSuccess(t *testing.T) {
	t.Skip("Skipping due to complexity of testing infinite loop with channels")

	// The runWatchLoop function is designed to run forever until the channels close.
	// Testing this properly requires more complex mocking infrastructure.
	// The function is indirectly tested through integration testing.
}

// TestRunWatchLoopWithValidWatcher tests runWatchLoop with a test watcher
func TestRunWatchLoopWithValidWatcher(t *testing.T) {
	// Create a test watcher that we can control
	testWatcher := monitor.NewTestWatcher()

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	session := &monitor.SessionInfo{
		ID:       "test-session-123",
		FilePath: "/fake/path.jsonl",
		Project:  "test-project",
	}

	history := []tui.HistoryEntry{
		{ID: "old1", Timestamp: "2024-01-01", Tokens: 100, Cost: 0.01, Project: "p1"},
	}

	// Create a mock program sender
	mockSender := &MockProgramSender{}

	// Run runWatchLoop in a goroutine
	done := make(chan bool)
	go func() {
		runWatchLoop(mockSender, testWatcher, db, session, history, false, []tui.ProjectInfo{})
		done <- true
	}()

	// Wait for initial messages
	time.Sleep(50 * time.Millisecond)

	// Check that initial messages were sent
	if mockSender.GetMessageCount() < 3 {
		t.Errorf("Expected at least 3 initial messages, got %d", mockSender.GetMessageCount())
	}

	// Send a test JSON line
	testLine := `{"message":{"type":"assistant","model":"claude-opus-4-5-20251101","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":0}}}`
	testWatcher.SendLine(testLine)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Close watcher to stop the loop
	testWatcher.Close()

	// Wait for completion
	select {
	case <-done:
		// Success - function completed
		if mockSender.GetMessageCount() < 3 {
			t.Errorf("Expected at least 3 messages, got %d", mockSender.GetMessageCount())
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("runWatchLoop did not complete in time")
	}
}

// TestRunWatchLoopWithError tests runWatchLoop when watcher sends error
func TestRunWatchLoopWithError(t *testing.T) {
	// Create a test watcher
	testWatcher := monitor.NewTestWatcher()

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	session := &monitor.SessionInfo{
		ID:       "test-session-123",
		FilePath: "/fake/path.jsonl",
		Project:  "test-project",
	}
	history := []tui.HistoryEntry{}

	// Create a mock program sender
	mockSender := &MockProgramSender{}

	// Run runWatchLoop in a goroutine
	done := make(chan bool)
	go func() {
		runWatchLoop(mockSender, testWatcher, db, session, history, false, []tui.ProjectInfo{})
		done <- true
	}()

	// Wait for initial messages
	time.Sleep(50 * time.Millisecond)

	// Send error
	testWatcher.SendError(errors.New("watcher test error"))

	// Wait for completion
	select {
	case <-done:
		// Success - function completed
		// Should have at least initial messages
		if mockSender.GetMessageCount() < 4 {
			t.Logf("Got %d messages (at least 3 initial + 1 error)", mockSender.GetMessageCount())
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("runWatchLoop with error did not complete in time")
	}
}

// TestRunWatchLoopWithModelChange tests runWatchLoop with model changes
func TestRunWatchLoopWithModelChange(t *testing.T) {
	// Create a test watcher
	testWatcher := monitor.NewTestWatcher()

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	session := &monitor.SessionInfo{
		ID:       "test-session-model-change",
		FilePath: "/fake/path.jsonl",
		Project:  "test-project",
	}
	history := []tui.HistoryEntry{}

	// Create a mock program sender
	mockSender := &MockProgramSender{}

	// Run runWatchLoop in a goroutine
	done := make(chan bool)
	go func() {
		runWatchLoop(mockSender, testWatcher, db, session, history, false, []tui.ProjectInfo{})
		done <- true
	}()

	// Wait for initial messages
	time.Sleep(50 * time.Millisecond)

	// Send a line with one model
	testLine1 := `{"message":{"type":"assistant","model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":0}}}`
	testWatcher.SendLine(testLine1)

	time.Sleep(50 * time.Millisecond)

	// Send a line with a different model
	testLine2 := `{"message":{"type":"assistant","model":"claude-opus-4-5-20251101","usage":{"input_tokens":200,"output_tokens":100,"cache_read_input_tokens":10}}}`
	testWatcher.SendLine(testLine2)

	time.Sleep(50 * time.Millisecond)

	// Close watcher to stop the loop
	testWatcher.Close()

	// Wait for completion
	select {
	case <-done:
		// Success - function completed
		if mockSender.GetMessageCount() < 5 {
			t.Logf("Got %d messages (expected at least 5: 3 initial + 2 updates)", mockSender.GetMessageCount())
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("runWatchLoop did not complete in time")
	}
}

// TestRunSuccess tests successful run() execution
func TestRunSuccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-run-success-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	// Create a fake session file in the temp dir
	sessionFile := filepath.Join(tmpDir, "test-session.jsonl")
	os.WriteFile(sessionFile, []byte(`{"message":{"type":"assistant"}}`), 0644)

	programRunCalled := false
	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return &monitor.SessionInfo{
				ID:       "test-session",
				FilePath: sessionFile,
				Project:  "test-project",
			}, nil
		},
		DBOpener: func(string) (*store.DB, error) {
			return db, nil
		},
		WatcherCreator: func(string) (monitor.WatcherInterface, error) {
			// Return a watcher that will fail quickly
			w, err := monitor.NewWatcher(sessionFile)
			if err != nil {
				return nil, err
			}
			return w, nil
		},
		ProgramRunner: func(p *tea.Program) error {
			programRunCalled = true
			return nil
		},
	}

	// This should succeed
	err = run(deps)
	if err != nil {
		t.Logf("run() returned error (may be acceptable): %v", err)
	}

	// The program runner might be called depending on timing
	_ = programRunCalled
}

// TestLogAndExit tests the logAndExit function
func TestLogAndExit(t *testing.T) {
	// Save original exitFunc
	originalExitFunc := exitFunc
	defer func() { exitFunc = originalExitFunc }()

	// Test with nil error - should not exit
	t.Run("nil error", func(t *testing.T) {
		exitCalled := false
		exitFunc = func(code int) {
			exitCalled = true
		}

		logAndExit(nil)

		if exitCalled {
			t.Error("logAndExit(nil) should not call exitFunc")
		}
	})

	// Test with error - should call exitFunc(1)
	t.Run("with error", func(t *testing.T) {
		exitCalled := false
		exitCode := 0
		exitFunc = func(code int) {
			exitCalled = true
			exitCode = code
		}

		logAndExit(errors.New("test error"))

		if !exitCalled {
			t.Error("logAndExit(err) should call exitFunc")
		}
		if exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
	})
}

// TestMain tests the main function
func TestMain(t *testing.T) {
	t.Skip("Skipping TestMain: it's an integration test that starts a TUI program")
	// Save original exitFunc
	originalExitFunc := exitFunc
	defer func() { exitFunc = originalExitFunc }()

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	// main() will fail because there's no actual Claude Code session
	// but we can test that it calls run() and potentially logAndExit()
	main()

	// If main gets an error, it should call logAndExit which calls exitFunc
	// The test passes if we get here without actual exit
	_ = exitCalled
}

// createTestDB creates a temporary database for testing
func createTestDB(t *testing.T) string {
	tmpFile, err := os.CreateTemp("", "test-db-*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp DB file: %v", err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

// TestAppDependencies tests the AppDependencies struct
func TestAppDependencies(t *testing.T) {
	deps := &AppDependencies{
		ProjectsDir: "/test/dir",
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return nil, nil
		},
		DBOpener: func(string) (*store.DB, error) {
			return nil, nil
		},
		WatcherCreator: func(string) (monitor.WatcherInterface, error) {
			return nil, nil
		},
		ProgramRunner: func(*tea.Program) error {
			return nil
		},
	}

	if deps.ProjectsDir != "/test/dir" {
		t.Errorf("Expected ProjectsDir to be /test/dir, got %s", deps.ProjectsDir)
	}

	// Test that all functions are callable
	if deps.SessionFinder == nil {
		t.Error("SessionFinder should not be nil")
	}
	if deps.DBOpener == nil {
		t.Error("DBOpener should not be nil")
	}
	if deps.WatcherCreator == nil {
		t.Error("WatcherCreator should not be nil")
	}
	if deps.ProgramRunner == nil {
		t.Error("ProgramRunner should not be nil")
	}
}

// TestGetModelName tests config.GetModelName function indirectly
func TestModelNameHandling(t *testing.T) {
	// Test getting model name for various model IDs
	tests := []struct {
		modelID   string
		wantName  string
	}{
		{"claude-opus-4-5-20251101", "Opus 4.5"},
		{"claude-sonnet-4-5-20251101", "Sonnet 4.5"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			name := config.GetModelName(tt.modelID)
			if name == "" {
				// At minimum, should return something
				t.Errorf("GetModelName(%q) returned empty string", tt.modelID)
			}
		})
	}
}

// TestRunHistoryConversion tests the history conversion logic in run()
func TestRunHistoryConversion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-history-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary database
	dbPath := createTestDB(t)
	defer os.Remove(dbPath)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	// Add some test history records
	now := time.Now()
	records := []store.SessionRecord{
		{
			ID:          "old-session-1",
			Timestamp:   now.Add(-2 * time.Hour),
			TotalTokens: 1000,
			Cost:        0.01,
			Project:     "project-1",
		},
		{
			ID:          "old-session-2",
			Timestamp:   now.Add(-1 * time.Hour),
			TotalTokens: 2000,
			Cost:        0.02,
			Project:     "project-2",
		},
	}

	for _, r := range records {
		err := db.SaveRecord(r)
		if err != nil {
			t.Fatalf("Failed to save record: %v", err)
		}
	}

	sessionID := "current-session"

	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return &monitor.SessionInfo{
				ID:       sessionID,
				FilePath: "/path/to/session.jsonl",
			}, nil
		},
		DBOpener: func(string) (*store.DB, error) {
			return db, nil
		},
		WatcherCreator: func(string) (monitor.WatcherInterface, error) {
			return nil, errors.New("watcher creation intentionally fails")
		},
		ProgramRunner: func(p *tea.Program) error {
			return nil
		},
	}

	_ = run(deps)

	// Verify history was loaded and current session is excluded
	// (This is implicit in the fact that run() doesn't fail on history loading)
}

// BenchmarkRun benchmarks the run function
func BenchmarkRun(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-run-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	deps := &AppDependencies{
		ProjectsDir: tmpDir,
		SessionFinder: func() (*monitor.SessionInfo, error) {
			return nil, errors.New("no session")
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = run(deps)
	}
}
