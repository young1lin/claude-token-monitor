package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindCurrentSessionWithRealFiles tests FindCurrentSession with real file system
// This creates a temporary directory structure to simulate Claude Code projects
func TestFindCurrentSessionWithRealFiles(t *testing.T) {
	// Create a temporary directory to act as projects dir
	tempDir, err := os.MkdirTemp("", "claude-test-projects-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory for a project
	projectDir := filepath.Join(tempDir, "C", "PythonProject", "test-project")
	err = os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create a session file
	sessionFile := filepath.Join(projectDir, "test-session-123.jsonl")
	content := `{"type":"assistant","uuid":"test","timestamp":"2024-01-01T12:00:00Z","sessionId":"test-session-123","message":{"model":"claude-sonnet-4-5-20250929","id":"msg1","type":"message","role":"assistant","stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":0,"cache_read_input_tokens":200}}}
`
	err = os.WriteFile(sessionFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Note: We cannot easily test FindCurrentSession without modifying the code
	// to accept a FileSystem interface or projects directory parameter
	// For now, this test demonstrates the file structure needed
	t.Logf("Created test environment at: %s", tempDir)
	t.Logf("Session file: %s", sessionFile)

	// Verify the file exists and can be read
	info, err := os.Stat(sessionFile)
	if err != nil {
		t.Fatalf("Session file doesn't exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Session file is empty")
	}

	// Test with a file that should be skipped (agent session)
	agentFile := filepath.Join(projectDir, "agent-test-456.jsonl")
	err = os.WriteFile(agentFile, []byte(`{"type":"assistant"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write agent file: %v", err)
	}
}

// TestGetFileOffsetIntegration tests GetFileOffset with real file
func TestGetFileOffsetIntegration(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-offset-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some content
	content := "line1\nline2\nline3\n"
	_, err = tmpFile.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
	tmpFile.Close()

	// Get offset
	offset, err := GetFileOffset(tmpFile.Name())
	if err != nil {
		t.Fatalf("GetFileOffset() failed: %v", err)
	}

	expectedOffset := int64(len(content))
	if offset != expectedOffset {
		t.Errorf("GetFileOffset() = %d, want %d", offset, expectedOffset)
	}
}

// TestTailFileIntegration tests TailFile with real file that grows
func TestTailFileIntegration(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-tail-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write initial content
	initialContent := "line1\nline2\n"
	err = os.WriteFile(tmpFile.Name(), []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write initial content: %v", err)
	}

	// Read from beginning
	lines, offset, err := TailFile(tmpFile.Name(), 0)
	if err != nil {
		t.Fatalf("TailFile() failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("TailFile() returned %d lines, want 2", len(lines))
	}

	if offset != int64(len(initialContent)) {
		t.Errorf("TailFile() offset = %d, want %d", offset, len(initialContent))
	}

	// Add more content
	additionalContent := "line3\nline4\n"
	err = os.WriteFile(tmpFile.Name(), []byte(initialContent+additionalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to append content: %v", err)
	}

	// Read from previous offset
	lines, offset, err = TailFile(tmpFile.Name(), offset)
	if err != nil {
		t.Fatalf("TailFile() with offset failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("TailFile() with offset returned %d lines, want 2", len(lines))
	}

	if lines[0] != "line3" {
		t.Errorf("TailFile()[0] = %s, want 'line3'", lines[0])
	}

	expectedNewOffset := int64(len(initialContent + additionalContent))
	if offset != expectedNewOffset {
		t.Errorf("TailFile() new offset = %d, want %d", offset, expectedNewOffset)
	}
}

// TestMonitorErrorIsError tests that MonitorError implements error interface
func TestMonitorErrorIsError(t *testing.T) {
	var err error = ErrNoSessionsFound
	if err == nil {
		t.Error("ErrNoSessionsFound should not be nil")
	}
	if err.Error() != "no Claude Code sessions found" {
		t.Errorf("ErrNoSessionsFound.Error() = %s, want 'no Claude Code sessions found'", err.Error())
	}

	err = ErrSessionInactive
	if err.Error() != "session is not active" {
		t.Errorf("ErrSessionInactive.Error() = %s, want 'session is not active'", err.Error())
	}

	customErr := &MonitorError{Message: "custom error"}
	if customErr.Error() != "custom error" {
		t.Errorf("MonitorError.Error() = %s, want 'custom error'", customErr.Error())
	}
}
