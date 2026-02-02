package content

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Memory files cache
var (
	memoryFilesCache     *MemoryFilesInfo
	memoryFilesCacheMu   sync.RWMutex
	memoryFilesCacheTime time.Time
	memoryFilesCacheTTL  = 60 * time.Second
)

// MemoryFilesInfo stores memory files statistics
type MemoryFilesInfo struct {
	CLAUDEMdCount int
	RulesCount    int
	MCPCount      int
	HooksCount    int
}

// MemoryFilesCollector collects memory files information
type MemoryFilesCollector struct {
	*BaseCollector
}

// NewMemoryFilesCollector creates a new memory files collector
func NewMemoryFilesCollector() *MemoryFilesCollector {
	return &MemoryFilesCollector{
		BaseCollector: NewBaseCollector(ContentMemoryFiles, 60*time.Second, true),
	}
}

// Collect returns memory files information
func (c *MemoryFilesCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	info := getMemoryFilesInfoCached(statusInput.Cwd)
	return formatMemoryFilesDisplay(info), nil
}

// getMemoryFilesInfoCached returns cached memory files info
func getMemoryFilesInfoCached(cwd string) MemoryFilesInfo {
	now := time.Now()

	memoryFilesCacheMu.RLock()
	if memoryFilesCache != nil && now.Sub(memoryFilesCacheTime) < memoryFilesCacheTTL {
		cached := *memoryFilesCache
		memoryFilesCacheMu.RUnlock()
		return cached
	}
	memoryFilesCacheMu.RUnlock()

	info := getMemoryFilesInfo(cwd)

	memoryFilesCacheMu.Lock()
	memoryFilesCache = &info
	memoryFilesCacheTime = now
	memoryFilesCacheMu.Unlock()

	return info
}

// getMemoryFilesInfo scans all Claude Code memory file locations
func getMemoryFilesInfo(cwd string) MemoryFilesInfo {
	info := MemoryFilesInfo{}

	// 1. Check Enterprise policy (Windows)
	if runtime.GOOS == "windows" {
		enterprisePath := filepath.Join("C:", "Program Files", "ClaudeCode", "CLAUDE.md")
		if _, err := os.Stat(enterprisePath); err == nil {
			info.CLAUDEMdCount++
		}
	}

	// 2 & 5. Recursive search for CLAUDE.md and CLAUDE.local.md
	info.CLAUDEMdCount += countClaudeMdUpward(cwd)

	// 3. Scan .claude/rules/ directories
	info.RulesCount += countRulesUpward(cwd)

	// 4. Check User memory: ~/.claude/CLAUDE.md
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalClaudeMd := filepath.Join(homeDir, ".claude", "CLAUDE.md")
		if _, err := os.Stat(globalClaudeMd); err == nil {
			info.CLAUDEMdCount++
		}
		globalRulesDir := filepath.Join(homeDir, ".claude", "rules")
		info.RulesCount += countRulesRecursive(globalRulesDir)
	}

	// Get MCP count
	info.MCPCount = getMCPCount(cwd)

	return info
}

// countRulesUpward searches upward for .claude/rules/ directories
func countRulesUpward(cwd string) int {
	totalCount := 0
	seen := make(map[string]bool)

	cwd = filepath.Clean(cwd)

	for i := 0; i < 20; i++ {
		rulesDir := filepath.Join(cwd, ".claude", "rules")
		if _, err := os.Stat(rulesDir); err == nil {
			if !seen[rulesDir] {
				totalCount += countRulesRecursive(rulesDir)
				seen[rulesDir] = true
			}
		}

		parent := filepath.Dir(cwd)
		if parent == cwd || parent == "" {
			break
		}
		cwd = parent
	}

	return totalCount
}

// countClaudeMdUpward searches upward for CLAUDE.md files
func countClaudeMdUpward(cwd string) int {
	count := 0
	seen := make(map[string]bool)

	cwd = filepath.Clean(cwd)

	for i := 0; i < 20; i++ {
		rootPath := filepath.Join(cwd, "CLAUDE.md")
		if _, err := os.Stat(rootPath); err == nil {
			if !seen[rootPath] {
				count++
				seen[rootPath] = true
			}
		}

		claudePath := filepath.Join(cwd, ".claude", "CLAUDE.md")
		if _, err := os.Stat(claudePath); err == nil {
			if !seen[claudePath] {
				count++
				seen[claudePath] = true
			}
		}

		localPath := filepath.Join(cwd, "CLAUDE.local.md")
		if _, err := os.Stat(localPath); err == nil {
			if !seen[localPath] {
				count++
				seen[localPath] = true
			}
		}

		parent := filepath.Dir(cwd)
		if parent == cwd || parent == "" {
			break
		}
		cwd = parent
	}

	return count
}

// countRulesRecursive recursively counts .md files in a rules directory
func countRulesRecursive(rulesDir string) int {
	count := 0

	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return count
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}

		if entry.IsDir() {
			subDir := filepath.Join(rulesDir, name)
			count += countRulesRecursive(subDir)
		} else if strings.HasSuffix(name, ".md") {
			count++
		}
	}

	return count
}

// getMCPCount reads and parses MCP servers configuration
func getMCPCount(cwd string) int {
	count := 0

	// Method 1: Check .claude/mcp_servers.json
	mcpPath := filepath.Join(cwd, ".claude", "mcp_servers.json")
	if data, err := os.ReadFile(mcpPath); err == nil {
		var mcpServers map[string]interface{}
		if err := json.Unmarshal(data, &mcpServers); err == nil {
			if servers, ok := mcpServers["mcpServers"].([]interface{}); ok {
				count = len(servers)
			} else if len(mcpServers) > 0 {
				count = len(mcpServers)
			}
		}
	}

	// Method 2: Check ~/.claude/settings.json for mcpServers
	if count == 0 {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
			if data, err := os.ReadFile(settingsPath); err == nil {
				var settings map[string]interface{}
				if err := json.Unmarshal(data, &settings); err == nil {
					if mcpServers, ok := settings["mcpServers"].(map[string]interface{}); ok {
						count = len(mcpServers)
					}
				}
			}
		}
	}

	return count
}

// formatMemoryFilesDisplay formats memory files display text
func formatMemoryFilesDisplay(info MemoryFilesInfo) string {
	if info.CLAUDEMdCount == 0 && info.RulesCount == 0 && info.MCPCount == 0 {
		return ""
	}

	parts := []string{}

	if info.CLAUDEMdCount > 0 {
		if info.CLAUDEMdCount == 1 {
			parts = append(parts, "CLAUDE.md")
		} else {
			parts = append(parts, fmt.Sprintf("%d CLAUDE.md", info.CLAUDEMdCount))
		}
	}

	if info.RulesCount > 0 {
		parts = append(parts, fmt.Sprintf("%d rules", info.RulesCount))
	}

	if info.MCPCount > 0 {
		parts = append(parts, fmt.Sprintf("%d MCPs", info.MCPCount))
	}

	return "📦 " + strings.Join(parts, " + ")
}
