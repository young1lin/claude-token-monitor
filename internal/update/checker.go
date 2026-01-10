// Package update provides auto-update functionality for the monitor.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	// releasesURL is the GitHub API endpoint for the latest release
	releasesURL = "https://api.github.com/repos/young1lin/minimal-mcp/releases/latest"
	// checkInterval is how often to check for updates
	checkInterval = 24 * time.Hour
)

// UpdateState tracks the last update check.
type UpdateState struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
	OptOut        bool      `json:"opt_out"`
}

// Checker checks for updates.
type Checker struct {
	currentVersion string
	stateFile      string
	httpClient     *http.Client
}

// NewChecker creates a new update checker.
func NewChecker(version string) *Checker {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	stateFile := filepath.Join(cacheDir, "claude-token-monitor", "update-state.json")

	return &Checker{
		currentVersion: version,
		stateFile:      stateFile,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Check checks for updates and returns release info if an update is available.
func (c *Checker) Check() (*ReleaseInfo, error) {
	state, err := c.loadState()
	if err != nil {
		state = &UpdateState{}
	}

	// Skip if opted out or checked recently
	if state.OptOut || time.Since(state.LastCheck) < checkInterval {
		return nil, nil
	}

	release, err := c.fetchLatest()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	latest := c.parseVersion(release.TagName)
	state.LastCheck = time.Now()
	state.LatestVersion = latest
	_ = c.saveState(state)

	if c.needsUpdate(latest) {
		return release, nil
	}

	return nil, nil
}

// fetchLatest fetches the latest release from GitHub.
func (c *Checker) fetchLatest() (*ReleaseInfo, error) {
	req, err := http.NewRequest("GET", releasesURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "claude-token-monitor")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// needsUpdate returns true if the current version is older than the latest.
func (c *Checker) needsUpdate(latest string) bool {
	// Dev version always needs update
	if c.currentVersion == "dev" {
		return true
	}

	currentV, err := semver.NewVersion(c.currentVersion)
	if err != nil {
		return false
	}

	latestV, err := semver.NewVersion(latest)
	if err != nil {
		return false
	}

	return latestV.GreaterThan(currentV)
}

// parseVersion extracts semantic version from tag name (e.g., "v1.2.3" -> "1.2.3").
func (c *Checker) parseVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// loadState loads the update state from disk.
func (c *Checker) loadState() (*UpdateState, error) {
	data, err := os.ReadFile(c.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &UpdateState{}, nil
		}
		return nil, err
	}

	var state UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return &UpdateState{}, nil
	}

	return &state, nil
}

// saveState saves the update state to disk.
func (c *Checker) saveState(state *UpdateState) error {
	if err := os.MkdirAll(filepath.Dir(c.stateFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.stateFile, data, 0644)
}

// SetOptOut sets the opt-out preference for update checks.
func (c *Checker) SetOptOut(optOut bool) error {
	state, err := c.loadState()
	if err != nil {
		state = &UpdateState{}
	}
	state.OptOut = optOut
	return c.saveState(state)
}
