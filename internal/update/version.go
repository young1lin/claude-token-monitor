// Package update provides auto-update functionality for the monitor.
package update

import "time"

// Version information injected by ldflags during build.
var (
	// Version is the current version (e.g., "1.0.0")
	Version = "dev"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the build timestamp
	BuildDate = "unknown"
)

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset (binary file).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}
