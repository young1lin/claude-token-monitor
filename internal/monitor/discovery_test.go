// Package monitor provides session monitoring and management.
package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDiscoverProjects_NoSessions tests discovery with no sessions.
func TestDiscoverProjects_NoSessions(t *testing.T) {
	t.Parallel()

	// Create temporary projects directory
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")

	// Create empty projects directory
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("Failed to create projects dir: %v", err)
	}

	result, err := DiscoverProjects(DiscoverConfig{
		ProjectsDir: projectsDir,
	})

	if err != nil {
		t.Fatalf("DiscoverProjects() error = %v", err)
	}

	if result == nil {
		t.Fatal("DiscoverProjects() returned nil result")
	}

	if len(result.Projects) != 0 {
		t.Errorf("DiscoverProjects() returned %d projects, want 0", len(result.Projects))
	}
}

// TestDiscoverProjects_SingleProject tests discovery with one project.
func TestDiscoverProjects_SingleProject(t *testing.T) {
	t.Parallel()

	// Create temporary projects directory
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")

	// Create project directory
	projectDir := filepath.Join(projectsDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create a session file
	sessionPath := filepath.Join(projectDir, "test-session.jsonl")
	content := `{"timestamp": "2024-01-01T12:00:00Z", "type": "test"}
`
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	result, err := DiscoverProjects(DiscoverConfig{
		ProjectsDir: projectsDir,
	})

	if err != nil {
		t.Fatalf("DiscoverProjects() error = %v", err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 1", len(result.Projects))
	}

	project := result.Projects[0]
	if project.Name != "test-project" {
		t.Errorf("Project name = %s, want 'test-project'", project.Name)
	}

	if project.SessionCount != 1 {
		t.Errorf("Session count = %d, want 1", project.SessionCount)
	}

	if project.MostRecentSession == nil {
		t.Error("MostRecentSession should not be nil")
	}
}

// TestDiscoverProjects_MultipleProjects tests discovery with multiple projects.
func TestDiscoverProjects_MultipleProjects(t *testing.T) {
	t.Parallel()

	// Create temporary projects directory
	tmpDir := t.TempDir()
	projectsDir := filepath.Join(tmpDir, "projects")

	// Create multiple projects with sessions
	projects := []struct {
		name         string
		sessionCount int
		lastMod      time.Time
	}{
		{"project-a", 2, time.Now().Add(-2 * time.Hour)},
		{"project-b", 1, time.Now().Add(-1 * time.Hour)},
		{"project-c", 3, time.Now().Add(-30 * time.Minute)},
	}

	for _, p := range projects {
		projectDir := filepath.Join(projectsDir, p.name)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}

		// Create session files
		for i := 0; i < p.sessionCount; i++ {
			sessionPath := filepath.Join(projectDir, "session.jsonl")
			content := `{"timestamp": "2024-01-01T12:00:00Z", "type": "test"}
`
			if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write session file: %v", err)
			}
			// Set modification time
			if err := os.Chtimes(sessionPath, p.lastMod, p.lastMod); err != nil {
				t.Fatalf("Failed to set file time: %v", err)
			}
		}
	}

	result, err := DiscoverProjects(DiscoverConfig{
		ProjectsDir: projectsDir,
	})

	if err != nil {
		t.Fatalf("DiscoverProjects() error = %v", err)
	}

	if len(result.Projects) != 3 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 3", len(result.Projects))
	}

	// Should be sorted by last activity (newest first)
	if result.Projects[0].Name != "project-c" {
		t.Errorf("First project = %s, want 'project-c' (most recent)", result.Projects[0].Name)
	}
	if result.Projects[2].Name != "project-a" {
		t.Errorf("Last project = %s, want 'project-a' (oldest)", result.Projects[2].Name)
	}
}

// TestFindSessionFiles_OldSessionsIncluded tests that old sessions are included.
// This is important for project selection - we want to show all projects,
// not just recently active ones.
func TestFindSessionFiles_OldSessionsIncluded(t *testing.T) {
	t.Parallel()

	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create an old session file (more than 1 hour ago)
	oldSessionPath := filepath.Join(projectDir, "old-session.jsonl")
	content := `{"timestamp": "2024-01-01T12:00:00Z", "type": "test"}
`
	if err := os.WriteFile(oldSessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Set modification time to 2 days ago
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldSessionPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	// Find sessions
	sessions, err := findSessionFiles(projectDir, "test-project")

	if err != nil {
		t.Fatalf("findSessionFiles() error = %v", err)
	}

	// Old sessions should be included for project selection
	if len(sessions) != 1 {
		t.Errorf("findSessionFiles() returned %d sessions, want 1 (old sessions should be included)", len(sessions))
	}

	if sessions[0].ID != "old-session" {
		t.Errorf("Session ID = %s, want 'old-session'", sessions[0].ID)
	}
}

// TestFindSessionFiles_EmptyFilesSkipped tests that empty files are skipped.
func TestFindSessionFiles_EmptyFilesSkipped(t *testing.T) {
	t.Parallel()

	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create an empty session file
	emptySessionPath := filepath.Join(projectDir, "empty-session.jsonl")
	if err := os.WriteFile(emptySessionPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Create a valid session file
	validSessionPath := filepath.Join(projectDir, "valid-session.jsonl")
	content := `{"timestamp": "2024-01-01T12:00:00Z", "type": "test"}
`
	if err := os.WriteFile(validSessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Find sessions
	sessions, err := findSessionFiles(projectDir, "test-project")

	if err != nil {
		t.Fatalf("findSessionFiles() error = %v", err)
	}

	// Empty files should be skipped
	if len(sessions) != 1 {
		t.Errorf("findSessionFiles() returned %d sessions, want 1 (empty files should be skipped)", len(sessions))
	}

	if sessions[0].ID != "valid-session" {
		t.Errorf("Session ID = %s, want 'valid-session'", sessions[0].ID)
	}
}
