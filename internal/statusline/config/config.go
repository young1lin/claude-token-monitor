// Package config provides YAML configuration support for the statusline plugin
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the statusline configuration
type Config struct {
	Display DisplayConfig `yaml:"display"`
	Format  FormatConfig  `yaml:"format"`
	Content ContentConfig `yaml:"content"`
	Cache   CacheConfig   `yaml:"cache"`
	Network NetworkConfig `yaml:"network"`
}

// NetworkConfig controls outbound network behavior.
// All fields default to "no proxy"; only the api.anthropic.com OAuth-usage call
// is affected — never general HTTP traffic from other tools.
type NetworkConfig struct {
	// ClaudeAPIProxy is the proxy URL applied ONLY to requests targeting
	// api.anthropic.com. Empty (default) → direct connection, no proxy.
	// Example: "http://127.0.0.1:7890" or "socks5://127.0.0.1:1080".
	//
	// This YAML value is the lowest-precedence source — see
	// (*Config).ResolveClaudeAPIProxy for the full chain
	// (--proxy flag > STATUSLINE_CLAUDE_PROXY env > this YAML field).
	ClaudeAPIProxy string `yaml:"claudeAPIProxy"`
}

// CacheConfig controls caching behavior.
//
// UsageTTLSeconds is the time-to-live, in seconds, for the OAuth-usage cache
// (~/.claude/.usage-cache.json). Within the TTL window the statusline serves
// cached data and skips the api.anthropic.com HTTP call, so a larger value
// means fewer requests. Failure-path and 429 backoff timings are deliberately
// not configurable.
type CacheConfig struct {
	UsageTTLSeconds int `yaml:"usageTTLSeconds"` // OAuth-usage cache TTL (default: 60)
}

// DisplayConfig controls what content is displayed
type DisplayConfig struct {
	SingleLine bool     `yaml:"singleLine"`
	Show       []string `yaml:"show"`
	Hide       []string `yaml:"hide"`
}

// FormatConfig controls formatting options
type FormatConfig struct {
	ProgressBar string `yaml:"progressBar"` // "ascii" or "braille"
	TimeFormat  string `yaml:"timeFormat"`  // "12h" or "24h"
	Compact     bool   `yaml:"compact"`
}

// ContentConfig controls content composition
type ContentConfig struct {
	Composers []ComposerConfig  `yaml:"composers"`
	Use       map[string]string `yaml:"use"` // Override default composers
}

// ComposerConfig defines a custom composer
type ComposerConfig struct {
	Name   string   `yaml:"name"`
	Input  []string `yaml:"input"`
	Format string   `yaml:"format"` // Go template format
}

// configFileNames lists accepted config file names in priority order.
// Both .yml and .yaml are first-class — .yml is checked first so that users
// who prefer the shorter extension hit a fast path.
var configFileNames = []string{"statusline.yml", "statusline.yaml"}

// Load loads configuration from file with priority:
//  1. Project-level: .claude/statusline.yml then .claude/statusline.yaml
//  2. Global:        ~/.claude/statusline.yml then ~/.claude/statusline.yaml
//  3. Default:       built-in defaults
//
// The first existing regular file wins; subsequent candidates are skipped.
func Load(projectDir string) (*Config, error) {
	// Try project-level configs first
	for _, name := range configFileNames {
		p := filepath.Join(projectDir, ".claude", name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return loadFile(p)
		}
	}

	// Then try global configs
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range configFileNames {
			p := filepath.Join(home, ".claude", name)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return loadFile(p)
			}
		}
	}

	// Return default config
	return DefaultConfig(), nil
}

// loadFile loads configuration from a specific file
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate format options
	if cfg.Format.ProgressBar != "" && cfg.Format.ProgressBar != "ascii" && cfg.Format.ProgressBar != "braille" {
		cfg.Format.ProgressBar = "braille" // Default to braille
	}
	if cfg.Format.TimeFormat != "" && cfg.Format.TimeFormat != "12h" && cfg.Format.TimeFormat != "24h" {
		cfg.Format.TimeFormat = "24h" // Default to 24h
	}

	// Validate composer configurations
	for i, comp := range cfg.Content.Composers {
		if comp.Name == "" {
			return nil, fmt.Errorf("composer at index %d: name is required", i)
		}
		if len(comp.Input) == 0 {
			return nil, fmt.Errorf("composer %q: input is required", comp.Name)
		}
		// Format is optional - if empty, uses simple concatenation
	}

	return cfg, nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Display: DisplayConfig{
			SingleLine: false,
			Show:       nil,
			Hide:       nil,
		},
		Format: FormatConfig{
			ProgressBar: "braille",
			TimeFormat:  "24h",
			Compact:     false,
		},
		Content: ContentConfig{
			Composers: nil, // Use default built-in composers
			Use:       nil, // No overrides
		},
		Cache: CacheConfig{
			UsageTTLSeconds: 60, // Default 60s — one OAuth-usage request per minute
		},
		Network: NetworkConfig{
			ClaudeAPIProxy: "", // Default: no proxy
		},
	}
}

// ResolveClaudeAPIProxy returns the effective proxy URL for api.anthropic.com
// requests after applying the configured precedence:
//
//  1. cliFlag                        (highest — e.g. --proxy=…)
//  2. STATUSLINE_CLAUDE_PROXY env    (middle)
//  3. network.claudeAPIProxy YAML    (lowest)
//
// Returns "" when nothing is configured. All inputs are whitespace-trimmed,
// so a blank/whitespace value at one layer falls through to the next.
func (c *Config) ResolveClaudeAPIProxy(cliFlag string) string {
	if cli := strings.TrimSpace(cliFlag); cli != "" {
		return cli
	}
	if env := strings.TrimSpace(os.Getenv("STATUSLINE_CLAUDE_PROXY")); env != "" {
		return env
	}
	return strings.TrimSpace(c.Network.ClaudeAPIProxy)
}

// ShouldShow returns true if the given content type should be displayed
func (c *Config) ShouldShow(contentType string) bool {
	hideSet := make(map[string]bool)
	for _, h := range c.Display.Hide {
		hideSet[h] = true
	}

	// If in hide list, don't show
	if hideSet[contentType] {
		return false
	}

	// If show is empty, show everything (except hide list)
	if len(c.Display.Show) == 0 {
		return true
	}

	// Only show if in show list
	showSet := make(map[string]bool)
	for _, s := range c.Display.Show {
		showSet[s] = true
	}
	return showSet[contentType]
}

// IsSingleLine returns true if single-line mode is enabled
func (c *Config) IsSingleLine() bool {
	return c.Display.SingleLine
}

// GetProgressBarStyle returns the progress bar style
func (c *Config) GetProgressBarStyle() string {
	if c.Format.ProgressBar == "" {
		return "braille"
	}
	return c.Format.ProgressBar
}

// GetTimeFormat returns the time format
func (c *Config) GetTimeFormat() string {
	if c.Format.TimeFormat == "" {
		return "24h"
	}
	return c.Format.TimeFormat
}

// IsCompact returns true if compact mode is enabled
func (c *Config) IsCompact() bool {
	return c.Format.Compact
}

// GetComposerOverride returns the composer to use for a given content type
// Returns empty string if no override is specified
func (c *Config) GetComposerOverride(contentType string) string {
	if c.Content.Use == nil {
		return ""
	}
	return c.Content.Use[contentType]
}

// HasCustomComposers returns true if the config defines custom composers
func (c *Config) HasCustomComposers() bool {
	return len(c.Content.Composers) > 0
}

// GetUsageCacheTTL returns the OAuth-usage cache TTL duration.
// Non-positive YAML values fall back to the 60s default so a misconfigured
// file can never accidentally hammer api.anthropic.com on every refresh.
func (c *Config) GetUsageCacheTTL() time.Duration {
	if c.Cache.UsageTTLSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.Cache.UsageTTLSeconds) * time.Second
}

// GetComposerConfig returns the configuration for a custom composer by name
// Returns nil if the composer is not found
func (c *Config) GetComposerConfig(name string) *ComposerConfig {
	for i := range c.Content.Composers {
		if c.Content.Composers[i].Name == name {
			return &c.Content.Composers[i]
		}
	}
	return nil
}
