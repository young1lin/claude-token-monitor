// Package monitor provides session monitoring and management.
package monitor

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DiscoverConfig holds configuration for session discovery.
type DiscoverConfig struct {
	ProjectsDir string
	// FilterActiveOnly only returns sessions that are currently active
	FilterActiveOnly bool
	// MaxSessions limits the number of sessions returned
	MaxSessions int
}

// DiscoverResult contains the results of session discovery.
type DiscoverResult struct {
	Sessions   []*SessionInfo
	ActiveID   string
	ErrorCount int
}

// DiscoverSessions finds all Claude Code sessions in the projects directory.
func DiscoverSessions(config DiscoverConfig) (*DiscoverResult, error) {
	result := &DiscoverResult{
		Sessions: make([]*SessionInfo, 0),
	}

	// Read all subdirectories in the projects directory
	entries, err := os.ReadDir(config.ProjectsDir)
	if err != nil {
		return result, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check for sessions directory in each project
		projectDir := filepath.Join(config.ProjectsDir, entry.Name())
		sessionsDir := filepath.Join(projectDir, "sessions")

		// Check if sessions directory exists
		if info, err := os.Stat(sessionsDir); err != nil || !info.IsDir() {
			continue
		}

		// Find all .jsonl files in sessions directory
		sessions, err := findSessionFiles(sessionsDir, entry.Name())
		if err != nil {
			result.ErrorCount++
			continue
		}

		result.Sessions = append(result.Sessions, sessions...)
	}

	// Sort by last modified time (newest first)
	sort.Slice(result.Sessions, func(i, j int) bool {
		return result.Sessions[i].LastMod.After(result.Sessions[j].LastMod)
	})

	// Find the most recently active session
	if len(result.Sessions) > 0 {
		result.ActiveID = result.Sessions[0].ID
	}

	// Apply max sessions limit
	if config.MaxSessions > 0 && len(result.Sessions) > config.MaxSessions {
		result.Sessions = result.Sessions[:config.MaxSessions]
	}

	return result, nil
}

// findSessionFiles finds all session JSONL files in a directory.
func findSessionFiles(sessionsDir, project string) ([]*SessionInfo, error) {
	var sessions []*SessionInfo

	err := filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if d.IsDir() {
			return nil
		}

		// Check for .jsonl files
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip empty files or very old files (not active)
		if info.Size() == 0 {
			return nil
		}

		// Check if file was modified recently (within last hour)
		modTime := info.ModTime()
		if time.Since(modTime) > time.Hour {
			return nil
		}

		// Extract session ID from filename
		sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

		sessions = append(sessions, &SessionInfo{
			ID:       sessionID,
			FilePath: path,
			Project:  project,
			LastMod:  modTime,
		})

		return nil
	})

	return sessions, err
}

// FindActiveSession finds the currently active Claude Code session.
// This is the original single-session finder for backward compatibility.
func FindActiveSession(projectsDir string) (*SessionInfo, error) {
	result, err := DiscoverSessions(DiscoverConfig{
		ProjectsDir:      projectsDir,
		FilterActiveOnly: true,
		MaxSessions:      1,
	})

	if err != nil {
		return nil, err
	}

	if len(result.Sessions) == 0 {
		return nil, os.ErrNotExist
	}

	return result.Sessions[0], nil
}

// WatchForNewSessions watches for new session files appearing.
// This is useful for detecting when a new Claude Code conversation starts.
func WatchForNewSessions(projectsDir string, callback func(*SessionInfo)) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	knownSessions := make(map[string]bool)

	// Initial scan
	result, _ := DiscoverSessions(DiscoverConfig{
		ProjectsDir: projectsDir,
	})
	for _, session := range result.Sessions {
		knownSessions[session.ID] = true
	}

	for range ticker.C {
		result, err := DiscoverSessions(DiscoverConfig{
			ProjectsDir: projectsDir,
		})
		if err != nil {
			continue
		}

		for _, session := range result.Sessions {
			if !knownSessions[session.ID] {
				knownSessions[session.ID] = true
				callback(session)
			}
		}
	}
}
