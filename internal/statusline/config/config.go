// Package config provides YAML configuration support for the statusline plugin
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the statusline configuration
type Config struct {
	Display DisplayConfig `yaml:"display"`
	Format  FormatConfig  `yaml:"format"`
	Content ContentConfig `yaml:"content"`
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
	Composers []ComposerConfig `yaml:"composers"`
	Use       map[string]string `yaml:"use"` // Override default composers
}

// ComposerConfig defines a custom composer
type ComposerConfig struct {
	Name   string   `yaml:"name"`
	Input  []string `yaml:"input"`
	Format string   `yaml:"format"` // Go template format
}

// Load loads configuration from file with priority:
// 1. Project-level: .claude/statusline.yaml
// 2. Global: ~/.claude/statusline.yaml
// 3. Default: built-in defaults
func Load(projectDir string) (*Config, error) {
	// Try project-level config first
	projectConfig := filepath.Join(projectDir, ".claude", "statusline.yaml")
	if info, err := os.Stat(projectConfig); err == nil && !info.IsDir() {
		return loadFile(projectConfig)
	}

	// Try global config
	home, err := os.UserHomeDir()
	if err == nil {
		globalConfig := filepath.Join(home, ".claude", "statusline.yaml")
		if info, err := os.Stat(globalConfig); err == nil && !info.IsDir() {
			return loadFile(globalConfig)
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
	}
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
