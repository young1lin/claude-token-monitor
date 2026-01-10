package monitor

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/config"
)

// SessionInfo contains information about a Claude Code session
type SessionInfo struct {
	ID        string
	FilePath  string
	Project   string
	Model     string
	Timestamp time.Time
	LastMod   time.Time
}

// FindCurrentSession finds the most recently modified JSONL session file
func FindCurrentSession() (*SessionInfo, error) {
	return FindCurrentSessionWithFS(OSFileSystem{})
}

// FindCurrentSessionWithFS finds the most recently modified JSONL session file using a custom FileSystem
func FindCurrentSessionWithFS(fs FileSystem) (*SessionInfo, error) {
	projectsDir := config.ProjectsDir()

	// Check if projects directory exists
	if _, err := fs.Stat(projectsDir); os.IsNotExist(err) {
		return nil, ErrNoSessionsFound
	}

	var latest SessionInfo
	found := false

	// Walk through all project directories
	err := fs.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Only process .jsonl files (not agent-*.jsonl)
		if !strings.HasSuffix(path, ".jsonl") || info.IsDir() {
			return nil
		}

		// Skip agent session files
		if strings.Contains(filepath.Base(path), "agent-") {
			return nil
		}

		// Get file modification time
		modTime := info.ModTime()

		// Parse session ID from filename (remove .jsonl extension)
		sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

		// Extract project path from directory structure
		projectPath := "unknown"
		if dir := filepath.Dir(path); dir != "." && dir != path {
			// Try to get relative path
			relPath, err := filepath.Rel(projectsDir, dir)
			if err == nil {
				projectPath = denormalizePath(relPath)
			}
		}

		// Read first line to get summary info
		model := "unknown"
		file, err := fs.Open(path)
		if err == nil {
			defer file.Close()
			reader := bufio.NewReader(file)
			line, _ := reader.ReadString('\n')
			// Could parse model from first assistant message here
			// For now, we'll update this when we parse messages
			_ = line
		}

		// Check if this is the latest file
		if !found || modTime.After(latest.LastMod) {
			latest = SessionInfo{
				ID:        sessionID,
				FilePath:  path,
				Project:   projectPath,
				Model:     model,
				Timestamp: modTime,
				LastMod:   modTime,
			}
			found = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrNoSessionsFound
	}

	return &latest, nil
}

// denormalizePath converts a normalized path back to a regular path
// Claude Code normalizes paths by replacing special chars with -
func denormalizePath(path string) string {
	// Simple denormalization - replace leading - with /
	// This is a basic implementation; Claude Code's normalization is more complex
	if strings.HasPrefix(path, "-") {
		path = strings.TrimPrefix(path, "-")
		// On Windows, paths start with drive letter like "C:"
		// On Unix, paths start with /
		if len(path) > 0 && path[0] != '-' {
			path = "/" + path
		}
	}
	// Replace remaining - with spaces or just keep as-is for display
	return strings.ReplaceAll(path, "-", " ")
}

// Errors
var (
	ErrNoSessionsFound = &MonitorError{Message: "no Claude Code sessions found"}
	ErrSessionInactive = &MonitorError{Message: "session is not active"}
)

// MonitorError represents an error in the monitor package
type MonitorError struct {
	Message string
}

func (e *MonitorError) Error() string {
	return e.Message
}

// GetFileOffset returns the current size of a file for starting tail operations
func GetFileOffset(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// TailFile reads new lines from a file starting from a given offset
func TailFile(filePath string, offset int64) ([]string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	// Seek to offset
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, offset, err
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Return new offset
	newOffset, _ := file.Seek(0, io.SeekCurrent)

	return lines, newOffset, scanner.Err()
}
