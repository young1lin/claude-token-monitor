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

// ProjectInfo contains information about a project for discovery results.
type ProjectInfo struct {
	Name              string
	SessionCount      int
	LastActivity      time.Time
	MostRecentSession *SessionInfo
}

// DiscoverProjectsResult contains the results of project discovery.
type DiscoverProjectsResult struct {
	Projects   []*ProjectInfo
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

		// Each project directory contains jsonl files directly (not in a sessions subdirectory)
		projectDir := filepath.Join(config.ProjectsDir, entry.Name())

		// Find all .jsonl files directly in project directory
		sessions, err := findSessionFiles(projectDir, entry.Name())
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

		// Skip empty files
		if info.Size() == 0 {
			return nil
		}

		// For project selection, show all sessions (not just recent ones)
		// This ensures all projects appear in the selection list
		modTime := info.ModTime()

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

// DiscoverProjects finds all unique projects with Claude Code sessions.
func DiscoverProjects(config DiscoverConfig) (*DiscoverProjectsResult, error) {
	// Get all sessions
	sessionResult, err := DiscoverSessions(config)
	if err != nil {
		return nil, err
	}

	// Group sessions by project name
	projectMap := make(map[string]*ProjectInfo)
	for _, session := range sessionResult.Sessions {
		if _, exists := projectMap[session.Project]; !exists {
			projectMap[session.Project] = &ProjectInfo{
				Name: session.Project,
			}
		}
		project := projectMap[session.Project]
		project.SessionCount++

		// Track most recent session
		if session.LastMod.After(project.LastActivity) {
			project.LastActivity = session.LastMod
			project.MostRecentSession = session
		}
	}

	// Convert to slice
	projects := make([]*ProjectInfo, 0, len(projectMap))
	for _, p := range projectMap {
		projects = append(projects, p)
	}

	// Sort by last activity (newest first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastActivity.After(projects[j].LastActivity)
	})

	return &DiscoverProjectsResult{
		Projects:   projects,
		ErrorCount: sessionResult.ErrorCount,
	}, nil
}
