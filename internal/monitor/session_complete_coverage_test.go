package monitor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

// testFileInfo is a complete os.FileInfo mock for testing with all fields
type testFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (t testFileInfo) Name() string       { return t.name }
func (t testFileInfo) Size() int64        { return t.size }
func (t testFileInfo) Mode() os.FileMode  { return t.mode }
func (t testFileInfo) ModTime() time.Time { return t.modTime }
func (t testFileInfo) IsDir() bool        { return t.isDir }
func (t testFileInfo) Sys() interface{}   { return nil }

// TestFindCurrentSessionWithFS_WalkError tests Walk returning an error
func TestFindCurrentSessionWithFS_WalkError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).Return(errors.New("walk failed"))

	_, err := FindCurrentSessionWithFS(mockFS)
	if err == nil {
		t.Error("Expected error when Walk fails, got nil")
	}
	if err.Error() != "walk failed" {
		t.Errorf("Expected 'walk failed' error, got: %v", err)
	}
}

// TestFindCurrentSessionWithFS_FileOpenError tests graceful handling of file open errors
func TestFindCurrentSessionWithFS_FileOpenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			info := testFileInfo{
				name:    "session.jsonl",
				isDir:   false,
				size:    100,
				modTime: time.Now(),
			}
			return walkFn("/path/session.jsonl", info, nil)
		},
	)
	mockFS.EXPECT().Open(gomock.Any()).Return(nil, errors.New("permission denied"))

	// Should still find session even if file open fails
	session, err := FindCurrentSessionWithFS(mockFS)
	if err != nil {
		t.Errorf("Should handle file open error gracefully: %v", err)
	}
	if session == nil {
		t.Error("Expected session despite file open error")
	}
	if session.ID != "session" {
		t.Errorf("Expected session ID 'session', got: %v", session.ID)
	}
}

// TestFindCurrentSessionWithFS_MultipleFiles tests selecting the most recent file
func TestFindCurrentSessionWithFS_MultipleFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// Walk multiple files
			walkFn("/old-session.jsonl", testFileInfo{
				name:    "old-session.jsonl",
				modTime: oldTime,
			}, nil)
			walkFn("/new-session.jsonl", testFileInfo{
				name:    "new-session.jsonl",
				modTime: now,
			}, nil)
			return nil
		},
	)
	mockFS.EXPECT().Open(gomock.Any()).Return(nil, errors.New("skip")).AnyTimes()

	session, err := FindCurrentSessionWithFS(mockFS)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should select the most recent file
	if session.FilePath != "/new-session.jsonl" {
		t.Errorf("Expected newest session, got: %v", session.FilePath)
	}
	if session.ID != "new-session" {
		t.Errorf("Expected ID 'new-session', got: %v", session.ID)
	}
}

// TestFindCurrentSessionWithFS_SkipsAgentFiles tests that agent-*.jsonl files are skipped
func TestFindCurrentSessionWithFS_SkipsAgentFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// Only agent files
			walkFn("/agent-123.jsonl", testFileInfo{name: "agent-123.jsonl"}, nil)
			walkFn("/agent-456.jsonl", testFileInfo{name: "agent-456.jsonl"}, nil)
			return nil
		},
	)

	_, err := FindCurrentSessionWithFS(mockFS)
	if err != ErrNoSessionsFound {
		t.Errorf("Expected ErrNoSessionsFound, got: %v", err)
	}
}

// TestFindCurrentSessionWithFS_DirectoryCheck tests that directories are skipped
func TestFindCurrentSessionWithFS_DirectoryCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// Directory should be skipped
			walkFn("/subdir/session.jsonl", testFileInfo{
				name:  "session.jsonl",
				isDir: true,
			}, nil)
			// Real file
			walkFn("/real-session.jsonl", testFileInfo{
				name:  "real-session.jsonl",
				isDir: false,
			}, nil)
			return nil
		},
	)
	mockFS.EXPECT().Open(gomock.Any()).Return(nil, errors.New("skip"))

	session, err := FindCurrentSessionWithFS(mockFS)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if session.FilePath != "/real-session.jsonl" {
		t.Errorf("Should skip directories, got: %v", session.FilePath)
	}
}

// TestFindCurrentSessionWithFS_NonJSONLFiles tests that non-jsonl files are skipped
func TestFindCurrentSessionWithFS_NonJSONLFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// Non-jsonl files should be skipped
			walkFn("/file.txt", testFileInfo{name: "file.txt"}, nil)
			walkFn("/file.json", testFileInfo{name: "file.json"}, nil)
			walkFn("/session.jsonl", testFileInfo{name: "session.jsonl"}, nil)
			return nil
		},
	)
	mockFS.EXPECT().Open(gomock.Any()).Return(nil, errors.New("skip"))

	session, err := FindCurrentSessionWithFS(mockFS)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if session.FilePath != "/session.jsonl" {
		t.Errorf("Should only find .jsonl files, got: %v", session.FilePath)
	}
}

// TestFindCurrentSessionWithFS_WalkErrorInCallback tests Walk callback error handling
func TestFindCurrentSessionWithFS_WalkErrorInCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// Pass error to callback - should be skipped
			walkFn("/error-path.jsonl", nil, errors.New("callback error"))
			// Valid file
			walkFn("/valid-session.jsonl", testFileInfo{name: "valid-session.jsonl"}, nil)
			return nil
		},
	)
	mockFS.EXPECT().Open(gomock.Any()).Return(nil, errors.New("skip"))

	session, err := FindCurrentSessionWithFS(mockFS)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if session.FilePath != "/valid-session.jsonl" {
		t.Errorf("Should skip files with errors, got: %v", session.FilePath)
	}
}

// TestFindCurrentSessionWithFS_ProjectsDirNotExists tests missing projects directory
func TestFindCurrentSessionWithFS_ProjectsDirNotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist)

	_, err := FindCurrentSessionWithFS(mockFS)
	if err != ErrNoSessionsFound {
		t.Errorf("Expected ErrNoSessionsFound when directory doesn't exist, got: %v", err)
	}
}

// TestFindCurrentSessionWithFS_EmptyDirectory tests directory with no matching files
func TestFindCurrentSessionWithFS_EmptyDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockFS.EXPECT().Stat(gomock.Any()).Return(&testFileInfo{isDir: true}, nil)
	mockFS.EXPECT().Walk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(root string, walkFn filepath.WalkFunc) error {
			// No files
			return nil
		},
	)

	_, err := FindCurrentSessionWithFS(mockFS)
	if err != ErrNoSessionsFound {
		t.Errorf("Expected ErrNoSessionsFound for empty directory, got: %v", err)
	}
}
