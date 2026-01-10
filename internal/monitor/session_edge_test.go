package monitor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFindCurrentSessionWithFSNoDirectory tests when projects directory doesn't exist
func TestFindCurrentSessionWithFSNoDirectory(t *testing.T) {
	fs := &ErrorFileSystem{statErr: os.ErrNotExist}

	_, err := FindCurrentSessionWithFS(fs)
	if err != ErrNoSessionsFound {
		t.Errorf("Expected ErrNoSessionsFound, got %v", err)
	}
}

// TestFindCurrentSessionWithFSNoSessions tests when directory exists but has no sessions
func TestFindCurrentSessionWithFSNoSessions(t *testing.T) {
	fs := &NoFilesFileSystem{}

	_, err := FindCurrentSessionWithFS(fs)
	if err != ErrNoSessionsFound {
		t.Errorf("Expected ErrNoSessionsFound, got %v", err)
	}
}

// TestFindCurrentSessionWithFSWalkError tests when walk returns an error
func TestFindCurrentSessionWithFSWalkError(t *testing.T) {
	walkErr := errors.New("walk error")
	fs := &WalkErrorFileSystem{err: walkErr}

	_, err := FindCurrentSessionWithFS(fs)
	if err != walkErr {
		t.Errorf("Expected walk error, got %v", err)
	}
}

// TestTailFileEdgeCases tests edge cases in TailFile
func TestTailFileEdgeCases(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, _, err := TailFile("/nonexistent/file.txt", 0)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		lines, offset, err := TailFile(tmpFile.Name(), 0)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(lines) != 0 {
			t.Errorf("Expected 0 lines, got %d", len(lines))
		}
		if offset != 0 {
			t.Errorf("Expected offset 0, got %d", offset)
		}
	})

	t.Run("read from middle", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		content := "line1\nline2\nline3\n"
		if _, err := tmpFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		tmpFile.Close()

		// Start from after "line1\n"
		offset := int64(len("line1\n"))
		lines, newOffset, err := TailFile(tmpFile.Name(), offset)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(lines) != 2 {
			t.Errorf("Expected 2 lines, got %d", len(lines))
		}
		if newOffset <= offset {
			t.Error("Offset should advance")
		}
	})

	t.Run("partial line at end", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write content without final newline
		content := "line1\nline2\npartial"
		if _, err := tmpFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		tmpFile.Close()

		lines, _, err := TailFile(tmpFile.Name(), 0)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Scanner should still read the partial line
		if len(lines) < 2 {
			t.Errorf("Expected at least 2 lines, got %d", len(lines))
		}
	})
}

// TestGetFileOffsetErrors tests error cases in GetFileOffset
func TestGetFileOffsetErrors(t *testing.T) {
	_, err := GetFileOffset("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// ErrorFileSystem is a mock FileSystem that always returns an error on Stat
type ErrorFileSystem struct {
	statErr error
}

func (e *ErrorFileSystem) Stat(name string) (os.FileInfo, error) {
	return nil, e.statErr
}

func (e *ErrorFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return nil, os.ErrNotExist
}

func (e *ErrorFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return walkFn(root, nil, nil)
}

func (e *ErrorFileSystem) Open(name string) (*os.File, error) {
	return nil, os.ErrNotExist
}

func (e *ErrorFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.ErrNotExist
}

// NoFilesFileSystem is a mock FileSystem that has valid directory but no files
type NoFilesFileSystem struct{}

func (n *NoFilesFileSystem) Stat(name string) (os.FileInfo, error) {
	return &MockFileInfo{
		name:    filepath.Base(name),
		mode:    os.ModeDir,
		isDir:   true,
		modTime: time.Now(),
	}, nil
}

func (n *NoFilesFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return []os.DirEntry{}, nil
}

func (n *NoFilesFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	// Directory exists but has no files
	return walkFn(root, &MockFileInfo{
		name:    filepath.Base(root),
		mode:    os.ModeDir,
		isDir:   true,
		modTime: time.Now(),
	}, nil)
}

func (n *NoFilesFileSystem) Open(name string) (*os.File, error) {
	return nil, os.ErrNotExist
}

func (n *NoFilesFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

// WalkErrorFileSystem is a mock FileSystem that returns error on Walk
type WalkErrorFileSystem struct {
	err error
}

func (w *WalkErrorFileSystem) Stat(name string) (os.FileInfo, error) {
	return &MockFileInfo{
		name:    filepath.Base(name),
		mode:    os.ModeDir,
		isDir:   true,
		modTime: time.Now(),
	}, nil
}

func (w *WalkErrorFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return nil, nil
}

func (w *WalkErrorFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return w.err
}

func (w *WalkErrorFileSystem) Open(name string) (*os.File, error) {
	return nil, os.ErrNotExist
}

func (w *WalkErrorFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

// MockFileInfo implements os.FileInfo for testing
type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return m.size }
func (m *MockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *MockFileInfo) ModTime() time.Time { return m.modTime }
func (m *MockFileInfo) IsDir() bool        { return m.isDir }
func (m *MockFileInfo) Sys() any           { return nil }
