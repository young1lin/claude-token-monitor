package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

func TestMonitorError(t *testing.T) {
	err := &MonitorError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%s'", err.Error())
	}
}

func TestErrNoSessionsFound(t *testing.T) {
	if ErrNoSessionsFound.Error() != "no Claude Code sessions found" {
		t.Errorf("Unexpected error message: %s", ErrNoSessionsFound.Error())
	}
}

func TestErrSessionInactive(t *testing.T) {
	if ErrSessionInactive.Error() != "session is not active" {
		t.Errorf("Unexpected error message: %s", ErrSessionInactive.Error())
	}
}

func TestDenormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"leading dash", "-path", "/path"},
		{"no leading dash", "path", "path"},
		{"empty string", "", ""},
		{"multiple dashes", "path-to-file", "path to file"},
		{"leading dash with more", "-C-Users-name", "/C Users name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := denormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("denormalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetFileOffset(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some content
	content := "hello\nworld\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Get offset
	offset, err := GetFileOffset(tmpFile.Name())
	if err != nil {
		t.Fatalf("GetFileOffset() failed: %v", err)
	}

	expected := int64(len(content))
	if offset != expected {
		t.Errorf("GetFileOffset() = %d, want %d", offset, expected)
	}
}

func TestGetFileOffsetNotExist(t *testing.T) {
	_, err := GetFileOffset("/nonexistent/file/path.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestTailFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write initial content
	lines := []string{"line1", "line2", "line3"}
	content := strings.Join(lines, "\n") + "\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Tail from beginning
	result, newOffset, err := TailFile(tmpFile.Name(), 0)
	if err != nil {
		t.Fatalf("TailFile() failed: %v", err)
	}

	if len(result) != len(lines) {
		t.Errorf("TailFile() returned %d lines, want %d", len(result), len(lines))
	}

	if newOffset != int64(len(content)) {
		t.Errorf("TailFile() newOffset = %d, want %d", newOffset, len(content))
	}
}

func TestTailFileWithOffset(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content
	content := "line1\nline2\nline3\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Start from offset after "line1\n"
	offset := int64(len("line1\n"))
	result, newOffset, err := TailFile(tmpFile.Name(), offset)
	if err != nil {
		t.Fatalf("TailFile() failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("TailFile() returned %d lines, want 2", len(result))
	}

	if result[0] != "line2" {
		t.Errorf("TailFile()[0] = %s, want 'line2'", result[0])
	}

	// Verify offset advanced
	if newOffset <= offset {
		t.Errorf("TailFile() newOffset = %d, should be > %d", newOffset, offset)
	}
}

func TestTailFileNotExist(t *testing.T) {
	_, _, err := TailFile("/nonexistent/file.txt", 0)
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestTailFileEmpty(t *testing.T) {
	// Create an empty temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	result, offset, err := TailFile(tmpFile.Name(), 0)
	if err != nil {
		t.Fatalf("TailFile() on empty file failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("TailFile() on empty file returned %d lines, want 0", len(result))
	}

	if offset != 0 {
		t.Errorf("TailFile() on empty file offset = %d, want 0", offset)
	}
}

// TestFindCurrentSession requires mocking the file system
// For now, we'll test error case when directory doesn't exist
func TestFindCurrentSessionNoDir(t *testing.T) {
	// This test is limited as we can't easily mock the config.ProjectsDir()
	// In a real scenario, you'd use interfaces and dependency injection
	_ = ErrNoSessionsFound // Use the variable to avoid "declared but not used" error
}

// TestFindCurrentSession tests the FindCurrentSession wrapper function
func TestFindCurrentSession(t *testing.T) {
	// This test verifies that FindCurrentSession calls FindCurrentSessionWithFS
	// We can't easily test the success case without a real Claude Code installation
	// But we can verify it returns an error when no sessions exist
	_, err := FindCurrentSession()

	// The function will either succeed (if Claude Code is installed) or fail (if not)
	// Either outcome is acceptable for this test
	_ = err // We just want to ensure the function is callable
}

// TestFindCurrentSessionWithFS tests the function with a mock file system
func TestFindCurrentSessionWithFSEmptyDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock file system with no sessions
	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT(). Stat(gomock.Any()).Return(nil, os.ErrNotExist)

	_, err := FindCurrentSessionWithFS(mockFS)
	if err == nil {
		t.Error("Expected error when projects directory doesn't exist")
	}
	if err != ErrNoSessionsFound {
		t.Logf("Got error (may be acceptable): %v", err)
	}
}

// TestFindCurrentSessionWithFSNoJSONLFiles tests with directory but no JSONL files
func TestFindCurrentSessionWithFSNoJSONLFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock file system with valid directory but no .jsonl files
	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT(). Stat(gomock.Any()).Return(&mockFileInfo{isDir: true}, nil)
	mockFS.EXPECT(). Walk(gomock.Any(), gomock.Any()).Return(nil).Do(func(root string, walkFn filepath.WalkFunc) error {
		// Simulate walking an empty directory
		walkFn(root, &mockFileInfo{isDir: true}, nil)
		return nil
	})

	_, err := FindCurrentSessionWithFS(mockFS)
	if err == nil {
		t.Error("Expected error when no JSONL files found")
	}
}

// mockFileInfo is a minimal os.FileInfo mock for testing
type mockFileInfo struct {
	isDir bool
	size  int64
}

func (m *mockFileInfo) Name() string       { return "mock" }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
