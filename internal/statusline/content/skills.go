package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkillsCollector collects skills information
type SkillsCollector struct {
	*BaseCollector
}

// NewSkillsCollector creates a new skills collector
func NewSkillsCollector() *SkillsCollector {
	return &SkillsCollector{
		BaseCollector: NewBaseCollector(ContentSkills, 60*time.Second, true),
	}
}

// Collect returns skills display string
func (c *SkillsCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}

	userCount := getUserSkillsCount()
	projectCount := getProjectSkillsCount(statusInput.Cwd)

	return formatSkillsDisplay(projectCount, userCount), nil
}

// getUserSkillsCount counts user-level skills in ~/.claude/skills/
func getUserSkillsCount() int {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	skillsDir := filepath.Join(homeDir, ".claude", "skills")
	return countSkillDirs(skillsDir)
}

// getProjectSkillsCount counts project-level skills in .claude/commands/
func getProjectSkillsCount(cwd string) int {
	commandsDir := filepath.Join(cwd, ".claude", "commands")
	return countSkillFiles(commandsDir)
}

// countSkillDirs counts non-hidden subdirectories (for user skills)
func countSkillDirs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") && entry.IsDir() {
			count++
		}
	}
	return count
}

// countSkillFiles counts .md files in a directory (for project skills/commands)
func countSkillFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".md") {
			count++
		}
	}
	return count
}

// formatSkillsDisplay formats skills count with project/user breakdown
func formatSkillsDisplay(project, user int) string {
	total := project + user
	if total == 0 {
		return ""
	}

	if project > 0 && user > 0 {
		return fmt.Sprintf("🎯 %d skills(%d proj + %d user)", total, project, user)
	}
	if project > 0 {
		return fmt.Sprintf("🎯 %d proj skills", project)
	}
	return fmt.Sprintf("🎯 %d user skills", user)
}
