package content

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserSkillsCount(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  int
	}{
		{
			name:  "no skills directory",
			setup: func(t *testing.T, dir string) {},
			want:  0,
		},
		{
			name: "empty skills directory",
			setup: func(t *testing.T, dir string) {
				os.MkdirAll(filepath.Join(dir, ".claude", "skills"), 0755)
			},
			want: 0,
		},
		{
			name: "multiple skill directories",
			setup: func(t *testing.T, dir string) {
				skillsDir := filepath.Join(dir, ".claude", "skills")
				os.MkdirAll(filepath.Join(skillsDir, "frontend-design"), 0755)
				os.MkdirAll(filepath.Join(skillsDir, "pdf"), 0755)
				os.MkdirAll(filepath.Join(skillsDir, "webapp-testing"), 0755)
			},
			want: 3,
		},
		{
			name: "hidden directories are skipped",
			setup: func(t *testing.T, dir string) {
				skillsDir := filepath.Join(dir, ".claude", "skills")
				os.MkdirAll(filepath.Join(skillsDir, "pdf"), 0755)
				os.MkdirAll(filepath.Join(skillsDir, ".hidden-skill"), 0755)
			},
			want: 1,
		},
		{
			name: "files are skipped",
			setup: func(t *testing.T, dir string) {
				skillsDir := filepath.Join(dir, ".claude", "skills")
				os.MkdirAll(filepath.Join(skillsDir, "pdf"), 0755)
				os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("readme"), 0644)
				os.WriteFile(filepath.Join(skillsDir, "config.json"), []byte("{}"), 0644)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			tt.setup(t, tmpDir)

			t.Setenv("HOME", tmpDir)
			t.Setenv("USERPROFILE", tmpDir)

			// Act
			got := getUserSkillsCount()

			// Assert
			if got != tt.want {
				t.Errorf("getUserSkillsCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetProjectSkillsCount(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  int
	}{
		{
			name:  "no commands directory",
			setup: func(t *testing.T, dir string) {},
			want:  0,
		},
		{
			name: "empty commands directory",
			setup: func(t *testing.T, dir string) {
				os.MkdirAll(filepath.Join(dir, ".claude", "commands"), 0755)
			},
			want: 0,
		},
		{
			name: "multiple command files",
			setup: func(t *testing.T, dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				os.WriteFile(filepath.Join(cmdDir, "commit-push.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(cmdDir, "setup.md"), []byte("# setup"), 0644)
			},
			want: 2,
		},
		{
			name: "non-md files are skipped",
			setup: func(t *testing.T, dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				os.WriteFile(filepath.Join(cmdDir, "commit-push.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(cmdDir, "config.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(cmdDir, "script.sh"), []byte("#!/bin/sh"), 0644)
			},
			want: 1,
		},
		{
			name: "hidden files are skipped",
			setup: func(t *testing.T, dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				os.WriteFile(filepath.Join(cmdDir, "setup.md"), []byte("# setup"), 0644)
				os.WriteFile(filepath.Join(cmdDir, ".draft.md"), []byte("# draft"), 0644)
			},
			want: 1,
		},
		{
			name: "subdirectories are skipped",
			setup: func(t *testing.T, dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(filepath.Join(cmdDir, "subdir"), 0755)
				os.WriteFile(filepath.Join(cmdDir, "setup.md"), []byte("# setup"), 0644)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			tt.setup(t, tmpDir)

			// Act
			got := getProjectSkillsCount(tmpDir)

			// Assert
			if got != tt.want {
				t.Errorf("getProjectSkillsCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatSkillsDisplay(t *testing.T) {
	tests := []struct {
		name    string
		project int
		user    int
		want    string
	}{
		{
			name:    "both project and user skills",
			project: 2,
			user:    7,
			want:    "🎯 9 skills(2 proj + 7 user)",
		},
		{
			name:    "only project skills",
			project: 3,
			user:    0,
			want:    "🎯 3 proj skills",
		},
		{
			name:    "only user skills",
			project: 0,
			user:    5,
			want:    "🎯 5 user skills",
		},
		{
			name:    "all zero returns empty",
			project: 0,
			user:    0,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatSkillsDisplay(tt.project, tt.user)

			// Assert
			if got != tt.want {
				t.Errorf("formatSkillsDisplay(%d, %d) = %q, want %q", tt.project, tt.user, got, tt.want)
			}
		})
	}
}

func TestCountSkillDirs(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{
			name: "non-existent directory returns zero",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			want: 0,
		},
		{
			name: "empty directory returns zero",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return dir
			},
			want: 0,
		},
		{
			name: "only files no directories returns zero",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644)
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
				return dir
			},
			want: 0,
		},
		{
			name: "mixed entries counts only non-hidden directories",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Visible directories
				require.NoError(t, os.Mkdir(filepath.Join(dir, "frontend-design"), 0755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, "pdf"), 0755))
				// Hidden directories
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden-dir"), 0755))
				// Files (should be ignored)
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644)
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
				return dir
			},
			want: 2,
		},
		{
			name: "all hidden directories returns zero",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden1"), 0755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden2"), 0755))
				return dir
			},
			want: 0,
		},
		{
			name: "only directories no files",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(dir, "skill1"), 0755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, "skill2"), 0755))
				require.NoError(t, os.Mkdir(filepath.Join(dir, "skill3"), 0755))
				return dir
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := tt.setup(t)

			// Act
			got := countSkillDirs(dir)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCountSkillFiles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{
			name: "non-existent directory returns zero",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			want: 0,
		},
		{
			name: "empty directory returns zero",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: 0,
		},
		{
			name: "only .md files",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(dir, "review.md"), []byte("# review"), 0644)
				os.WriteFile(filepath.Join(dir, "deploy.md"), []byte("# deploy"), 0644)
				return dir
			},
			want: 3,
		},
		{
			name: "non-.md files are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "script.sh"), []byte("#!/bin/sh"), 0644)
				os.WriteFile(filepath.Join(dir, "data.yaml"), []byte("key: val"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "hidden files are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(dir, ".draft.md"), []byte("# draft"), 0644)
				os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "subdirectories are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# commit"), 0644)
				require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
				os.WriteFile(filepath.Join(dir, "subdir", "nested.md"), []byte("nested"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "mixed entries",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# commit"), 0644)
				os.WriteFile(filepath.Join(dir, "review.md"), []byte("# review"), 0644)
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("hidden"), 0644)
				require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
				return dir
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := tt.setup(t)

			// Act
			got := countSkillFiles(dir)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
