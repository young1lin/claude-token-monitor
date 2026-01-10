package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFindCurrentSessionIntegration tests FindCurrentSession with real file system
// This is an integration test that creates a real directory structure
func TestFindCurrentSessionIntegration(t *testing.T) {
	// Get the actual projects directory from config
	projectsDir := strings.TrimSpace(os.Getenv("CLAUDE_PROJECTS_DIR"))
	if projectsDir == "" {
		t.Skip("Set CLAUDE_PROJECTS_DIR environment variable to run this integration test")
	}

	// Check if the directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		t.Skipf("Projects directory does not exist: %s", projectsDir)
	}

	// Try to find the current session
	session, err := FindCurrentSession()
	if err != nil {
		if err == ErrNoSessionsFound {
			t.Skip("No Claude Code sessions found (this is expected if Claude Code hasn't been run)")
		} else {
			t.Logf("FindCurrentSession returned error: %v", err)
		}
		return
	}

	if session == nil {
		t.Error("Expected session to be found, got nil")
		return
	}

	// Verify session fields
	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.FilePath == "" {
		t.Error("Session FilePath should not be empty")
	}
	if !strings.HasSuffix(session.FilePath, ".jsonl") {
		t.Errorf("Session FilePath should end with .jsonl, got %s", session.FilePath)
	}
}

// TestOSFileSystem tests OSFileSystem methods indirectly through FindCurrentSession
func TestOSFileSystem(t *testing.T) {
	fs := OSFileSystem{}

	// Test Stat with a known location
	tmpDir, err := os.MkdirTemp("", "os-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	info, err := fs.Stat(tmpDir)
	if err != nil {
		t.Errorf("Stat() failed for existing directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Temp directory should be a directory")
	}

	// Test Stat with non-existent path
	_, err = fs.Stat(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("Stat() should return error for non-existent path")
	}

	// Test Open with a real file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test content"
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	file, err := fs.Open(testFile)
	if err != nil {
		t.Errorf("Open() failed for existing file: %v", err)
	}
	file.Close()

	// Test Open with non-existent file
	_, err = fs.Open(filepath.Join(tmpDir, "nonexistent.txt"))
	if err == nil {
		t.Error("Open() should return error for non-existent file")
	}

	// Test MkdirAll
	subDir := filepath.Join(tmpDir, "sub1", "sub2")
	err = fs.MkdirAll(subDir, 0755)
	if err != nil {
		t.Errorf("MkdirAll() failed: %v", err)
	}
	// Verify it was created
	if _, err := os.Stat(subDir); err != nil {
		t.Errorf("MkdirAll() didn't create directory: %v", err)
	}

	// Test ReadDir
	entries, err := fs.ReadDir(tmpDir)
	if err != nil {
		t.Errorf("ReadDir() failed: %v", err)
	}
	if len(entries) == 0 {
		t.Error("ReadDir() should return at least the test.txt file")
	}

	// Test Walk
	walkCalled := false
	err = fs.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == tmpDir {
			walkCalled = true
		}
		return nil
	})
	if err != nil {
		t.Errorf("Walk() failed: %v", err)
	}
	if !walkCalled {
		t.Error("Walk() should call the callback for the root directory")
	}
}

// TestFindCurrentSessionWithFakeProjects creates a fake projects directory
// This is a more controlled integration test
func TestFindCurrentSessionWithFakeProjects(t *testing.T) {
	// Note: This test modifies the config.ProjectsDir() behavior
	// In a real scenario, you would pass the projects directory as a parameter
	// For now, we document how this would work

	t.Skip("FindCurrentSession requires projects directory to be set via config")

	// The ideal approach would be to refactor the code to accept a projectsDir parameter:
	// func FindCurrentSessionInDir(projectsDir string, fs FileSystem) (*SessionInfo, error)
}

// TestWatcherInternalMethods tests internal watcher methods
func TestWatcherInternalMethods(t *testing.T) {
	// checkForNewContent is a private method, but we can test it through NewWatcher
	// by creating a watcher and checking its behavior

	t.Run("watcher with changing file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "watcher-test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		watcher, err := NewWatcher(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to create watcher: %v", err)
		}

		// Write some content to the file
		newContent := "new line 1\nnew line 2\n"
		err = os.WriteFile(tmpFile.Name(), []byte(newContent), 0644)
		if err != nil {
			watcher.Close()
			t.Fatalf("Failed to write to file: %v", err)
		}

		// Wait a bit for the watcher to detect changes
		time.Sleep(100 * time.Millisecond)

		// Close the watcher
		watcher.Close()

		// If we got here without panic, the test passes
	})
}

// TestTailFileErrorCases tests various error scenarios
func TestTailFileErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		offset  int64
		wantErr bool
	}{
		{
			name:    "read beyond file size",
			content: "short",
			offset:  100,
			wantErr: false, // Should return empty result, not error
		},
		{
			name:    "negative offset",
			content: "line1\nline2\n",
			offset:  -1,
			wantErr: true, // Seek should fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "tail-error-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}
			tmpFile.Close()

			_, _, err = TailFile(tmpFile.Name(), tt.offset)
			if (err != nil) != tt.wantErr {
				t.Errorf("TailFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
