// Package config provides YAML configuration support for the statusline plugin
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Display.SingleLine {
		t.Error("Default SingleLine should be false")
	}

	if cfg.Display.Show != nil {
		t.Error("Default Show should be nil")
	}

	if cfg.Display.Hide != nil {
		t.Error("Default Hide should be nil")
	}

	if cfg.Format.ProgressBar != "braille" {
		t.Errorf("Default ProgressBar should be 'braille', got '%s'", cfg.Format.ProgressBar)
	}

	if cfg.Format.TimeFormat != "24h" {
		t.Errorf("Default TimeFormat should be '24h', got '%s'", cfg.Format.TimeFormat)
	}

	if cfg.Format.Compact {
		t.Error("Default Compact should be false")
	}

	// Test default cache config
	if cfg.Cache.UsageTTLSeconds != 30 {
		t.Errorf("Default Cache.UsageTTLSeconds should be 30, got %d", cfg.Cache.UsageTTLSeconds)
	}
}

func TestShouldShow(t *testing.T) {
	tests := []struct {
		name        string
		show        []string
		hide        []string
		contentType string
		want        bool
	}{
		{
			name:        "empty show/hide - shows everything",
			show:        nil,
			hide:        nil,
			contentType: "model",
			want:        true,
		},
		{
			name:        "show list - only shows listed items",
			show:        []string{"model", "token-bar"},
			hide:        nil,
			contentType: "model",
			want:        true,
		},
		{
			name:        "show list - hides unlisted items",
			show:        []string{"model", "token-bar"},
			hide:        nil,
			contentType: "git-branch",
			want:        false,
		},
		{
			name:        "hide list - hides listed items",
			show:        nil,
			hide:        []string{"claude-version"},
			contentType: "claude-version",
			want:        false,
		},
		{
			name:        "hide list - shows unlisted items",
			show:        nil,
			hide:        []string{"claude-version"},
			contentType: "model",
			want:        true,
		},
		{
			name:        "both lists - hide takes priority",
			show:        []string{"model", "claude-version"},
			hide:        []string{"claude-version"},
			contentType: "claude-version",
			want:        false,
		},
		{
			name:        "both lists - show listed but not hidden",
			show:        []string{"model", "token-bar"},
			hide:        []string{"claude-version"},
			contentType: "model",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Display: DisplayConfig{
					Show: tt.show,
					Hide: tt.hide,
				},
			}
			got := cfg.ShouldShow(tt.contentType)
			if got != tt.want {
				t.Errorf("ShouldShow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	// Create a temporary directory for test config files
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		configYAML   string
		wantSingle   bool
		wantShow     []string
		wantHide     []string
		wantProgress string
		wantTime     string
		wantCompact  bool
		wantErr      bool
	}{
		{
			name: "valid minimal config",
			configYAML: `
display:
  singleLine: true
`,
			wantSingle:   true,
			wantProgress: "braille",
			wantTime:     "24h",
		},
		{
			name: "valid full config",
			configYAML: `
display:
  singleLine: true
  show:
    - folder
    - model
  hide:
    - claude-version
format:
  progressBar: ascii
  timeFormat: 12h
  compact: true
`,
			wantSingle:   true,
			wantShow:     []string{"folder", "model"},
			wantHide:     []string{"claude-version"},
			wantProgress: "ascii",
			wantTime:     "12h",
			wantCompact:  true,
		},
		{
			name: "invalid progress bar falls back to default",
			configYAML: `
format:
  progressBar: invalid
`,
			wantProgress: "braille",
		},
		{
			name: "invalid time format falls back to default",
			configYAML: `
format:
  timeFormat: invalid
`,
			wantTime: "24h",
		},
		{
			name:         "empty config uses defaults",
			configYAML:   `{}`,
			wantProgress: "braille",
			wantTime:     "24h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.name+".yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := loadFile(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if cfg == nil {
				if !tt.wantErr {
					t.Error("loadFile() returned nil config")
				}
				return
			}

			if cfg.Display.SingleLine != tt.wantSingle {
				t.Errorf("SingleLine = %v, want %v", cfg.Display.SingleLine, tt.wantSingle)
			}

			if tt.wantShow != nil {
				if len(cfg.Display.Show) != len(tt.wantShow) {
					t.Errorf("Show length = %d, want %d", len(cfg.Display.Show), len(tt.wantShow))
				} else {
					for i, want := range tt.wantShow {
						if cfg.Display.Show[i] != want {
							t.Errorf("Show[%d] = %v, want %v", i, cfg.Display.Show[i], want)
						}
					}
				}
			}

			if tt.wantHide != nil {
				if len(cfg.Display.Hide) != len(tt.wantHide) {
					t.Errorf("Hide length = %d, want %d", len(cfg.Display.Hide), len(tt.wantHide))
				} else {
					for i, want := range tt.wantHide {
						if cfg.Display.Hide[i] != want {
							t.Errorf("Hide[%d] = %v, want %v", i, cfg.Display.Hide[i], want)
						}
					}
				}
			}

			// Only check format options if explicitly specified
			// Zero values in wantProgress/wantTime/wantCompact mean "don't check"
			if tt.wantProgress != "" && cfg.Format.ProgressBar != tt.wantProgress {
				t.Errorf("ProgressBar = %v, want %v", cfg.Format.ProgressBar, tt.wantProgress)
			}

			if tt.wantTime != "" && cfg.Format.TimeFormat != tt.wantTime {
				t.Errorf("TimeFormat = %v, want %v", cfg.Format.TimeFormat, tt.wantTime)
			}

			// For Compact, only check if wantCompact is explicitly set
			// Since bool zero is false, we can't distinguish between "don't check" and "want false"
			// So we check if the test name contains "compact"
			if strings.Contains(tt.name, "compact") && cfg.Format.Compact != tt.wantCompact {
				t.Errorf("Compact = %v, want %v", cfg.Format.Compact, tt.wantCompact)
			}
		})
	}
}

func TestLoadPriority(t *testing.T) {
	// Create temporary directories
	tempHome := t.TempDir()
	tempProject := t.TempDir()

	// Create global config
	globalConfigPath := filepath.Join(tempHome, ".claude")
	if err := os.MkdirAll(globalConfigPath, 0755); err != nil {
		t.Fatal(err)
	}
	globalConfig := filepath.Join(globalConfigPath, "statusline.yaml")
	globalYAML := `
display:
  singleLine: false
  hide:
    - global-test
`
	if err := os.WriteFile(globalConfig, []byte(globalYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create project config
	projectConfigPath := filepath.Join(tempProject, ".claude")
	if err := os.MkdirAll(projectConfigPath, 0755); err != nil {
		t.Fatal(err)
	}
	projectConfig := filepath.Join(projectConfigPath, "statusline.yaml")
	projectYAML := `
display:
  singleLine: true
  hide:
    - project-test
`
	if err := os.WriteFile(projectConfig, []byte(projectYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Note: We can't fully test Load() without mocking os.UserHomeDir
	// This test just verifies the file structure is created correctly
	t.Run("files exist", func(t *testing.T) {
		if _, err := os.Stat(globalConfig); err != nil {
			t.Errorf("Global config file does not exist: %v", err)
		}
		if _, err := os.Stat(projectConfig); err != nil {
			t.Errorf("Project config file does not exist: %v", err)
		}
	})
}

func TestGetProgressBarStyle(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "braille style",
			cfg: &Config{
				Format: FormatConfig{ProgressBar: "braille"},
			},
			want: "braille",
		},
		{
			name: "ascii style",
			cfg: &Config{
				Format: FormatConfig{ProgressBar: "ascii"},
			},
			want: "ascii",
		},
		{
			name: "empty defaults to braille",
			cfg: &Config{
				Format: FormatConfig{ProgressBar: ""},
			},
			want: "braille",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetProgressBarStyle(); got != tt.want {
				t.Errorf("GetProgressBarStyle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTimeFormat(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "24h format",
			cfg: &Config{
				Format: FormatConfig{TimeFormat: "24h"},
			},
			want: "24h",
		},
		{
			name: "12h format",
			cfg: &Config{
				Format: FormatConfig{TimeFormat: "12h"},
			},
			want: "12h",
		},
		{
			name: "empty defaults to 24h",
			cfg: &Config{
				Format: FormatConfig{TimeFormat: ""},
			},
			want: "24h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetTimeFormat(); got != tt.want {
				t.Errorf("GetTimeFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUsageCacheTTL(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantSec int
	}{
		{
			name: "default 30 seconds",
			cfg: &Config{
				Cache: CacheConfig{UsageTTLSeconds: 30},
			},
			wantSec: 30,
		},
		{
			name: "custom 60 seconds",
			cfg: &Config{
				Cache: CacheConfig{UsageTTLSeconds: 60},
			},
			wantSec: 60,
		},
		{
			name: "zero defaults to 30 seconds",
			cfg: &Config{
				Cache: CacheConfig{UsageTTLSeconds: 0},
			},
			wantSec: 30,
		},
		{
			name: "negative defaults to 30 seconds",
			cfg: &Config{
				Cache: CacheConfig{UsageTTLSeconds: -1},
			},
			wantSec: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetUsageCacheTTL()
			want := time.Duration(tt.wantSec) * time.Second
			if got != want {
				t.Errorf("GetUsageCacheTTL() = %v, want %v", got, want)
			}
		})
	}
}

func TestComposerConfig(t *testing.T) {
	t.Run("GetComposerOverride", func(t *testing.T) {
		tests := []struct {
			name        string
			use         map[string]string
			contentType string
			want        string
		}{
			{
				name:        "nil use map returns empty",
				use:         nil,
				contentType: "token",
				want:        "",
			},
			{
				name:        "empty use map returns empty",
				use:         map[string]string{},
				contentType: "token",
				want:        "",
			},
			{
				name:        "returns override for matching type",
				use:         map[string]string{"token": "token-simple"},
				contentType: "token",
				want:        "token-simple",
			},
			{
				name:        "returns empty for non-matching type",
				use:         map[string]string{"git": "git-simple"},
				contentType: "token",
				want:        "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &Config{
					Content: ContentConfig{
						Use: tt.use,
					},
				}
				got := cfg.GetComposerOverride(tt.contentType)
				if got != tt.want {
					t.Errorf("GetComposerOverride() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("HasCustomComposers", func(t *testing.T) {
		tests := []struct {
			name  string
			comps []ComposerConfig
			want  bool
		}{
			{
				name:  "nil composers returns false",
				comps: nil,
				want:  false,
			},
			{
				name:  "empty composers returns false",
				comps: []ComposerConfig{},
				want:  false,
			},
			{
				name: "has composers returns true",
				comps: []ComposerConfig{
					{Name: "custom", Input: []string{"model"}},
				},
				want: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &Config{
					Content: ContentConfig{
						Composers: tt.comps,
					},
				}
				got := cfg.HasCustomComposers()
				if got != tt.want {
					t.Errorf("HasCustomComposers() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("GetComposerConfig", func(t *testing.T) {
		comps := []ComposerConfig{
			{Name: "custom1", Input: []string{"model"}},
			{Name: "custom2", Input: []string{"git-branch"}},
		}
		cfg := &Config{
			Content: ContentConfig{
				Composers: comps,
			},
		}

		tests := []struct {
			name         string
			composerName string
			want         *ComposerConfig
		}{
			{
				name:         "finds existing composer",
				composerName: "custom1",
				want:         &comps[0],
			},
			{
				name:         "finds another existing composer",
				composerName: "custom2",
				want:         &comps[1],
			},
			{
				name:         "returns nil for non-existent composer",
				composerName: "nonexistent",
				want:         nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cfg.GetComposerConfig(tt.composerName)
				if got != tt.want {
					t.Errorf("GetComposerConfig() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestIsSingleLine(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		cfg := &Config{Display: DisplayConfig{SingleLine: true}}
		if !cfg.IsSingleLine() {
			t.Error("expected true")
		}
	})
	t.Run("false", func(t *testing.T) {
		cfg := &Config{Display: DisplayConfig{SingleLine: false}}
		if cfg.IsSingleLine() {
			t.Error("expected false")
		}
	})
}

func TestIsCompact(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		cfg := &Config{Format: FormatConfig{Compact: true}}
		if !cfg.IsCompact() {
			t.Error("expected true")
		}
	})
	t.Run("false", func(t *testing.T) {
		cfg := &Config{Format: FormatConfig{Compact: false}}
		if cfg.IsCompact() {
			t.Error("expected false")
		}
	})
}

func TestLoadFileWithComposerConfig(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		configYAML   string
		wantComps    int
		wantOverride map[string]string
		wantErr      bool
	}{
		{
			name: "config with custom composer",
			configYAML: `
content:
  composers:
    - name: token-simple
      input: [model, token-bar]
      format: "[{{.model}} {{.token-bar}}]"
`,
			wantComps: 1,
		},
		{
			name: "config with composer override",
			configYAML: `
content:
  use:
    token: token-simple
    git: git-branch-only
`,
			wantComps:    0,
			wantOverride: map[string]string{"token": "token-simple", "git": "git-branch-only"},
		},
		{
			name: "config with both composers and overrides",
			configYAML: `
content:
  composers:
    - name: my-token
      input: [model]
      format: "🤖 {{.model}}"
  use:
    token: my-token
`,
			wantComps:    1,
			wantOverride: map[string]string{"token": "my-token"},
		},
		{
			name: "composer without name returns error",
			configYAML: `
content:
  composers:
    - input: [model]
`,
			wantErr: true,
		},
		{
			name: "composer without input returns error",
			configYAML: `
content:
  composers:
    - name: test
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.name+".yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := loadFile(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(cfg.Content.Composers) != tt.wantComps {
				t.Errorf("Composers length = %d, want %d", len(cfg.Content.Composers), tt.wantComps)
			}

			if tt.wantOverride != nil {
				for k, v := range tt.wantOverride {
					got := cfg.GetComposerOverride(k)
					if got != v {
						t.Errorf("GetComposerOverride(%q) = %q, want %q", k, got, v)
					}
				}
			}
		})
	}
}

func TestLoad_DefaultConfig(t *testing.T) {
	// No config files exist -> should return default
	tempDir := t.TempDir()

	cfg, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	if cfg.Format.ProgressBar != "braille" {
		t.Errorf("Default ProgressBar = %q, want %q", cfg.Format.ProgressBar, "braille")
	}
}

func TestLoad_ProjectConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create .claude/statusline.yaml in project dir
	claudeDir := filepath.Join(tempDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `
display:
  singleLine: true
`
	if err := os.WriteFile(filepath.Join(claudeDir, "statusline.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Display.SingleLine {
		t.Error("Load() should return project config with SingleLine=true")
	}
}

func TestLoad_GlobalConfig(t *testing.T) {
	tempDir := t.TempDir()

	// No project config, create global config
	homeDir := t.TempDir()
	globalClaudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(globalClaudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `
format:
  progressBar: ascii
`
	if err := os.WriteFile(filepath.Join(globalClaudeDir, "statusline.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// We can't easily override os.UserHomeDir(), so this tests the project-not-found path
	cfg, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Should return default since neither project nor global config is found by os.UserHomeDir
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte("display:\n  singleLine: [not valid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadFile(configPath)
	if err == nil {
		t.Error("loadFile() should return error for invalid YAML")
	}
}

func TestLoadFile_NonexistentFile(t *testing.T) {
	_, err := loadFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("loadFile() should return error for nonexistent file")
	}
}

func TestLoad_ProjectConfigIsDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create .claude as a directory named "statusline.yaml" (not possible),
	// but we can create a directory at the config path.
	// Actually, we create .claude as a directory to test that a directory
	// at the config path is skipped (not treated as a file).
	claudeDir := filepath.Join(tempDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a directory named statusline.yaml inside .claude
	statuslineDir := filepath.Join(claudeDir, "statusline.yaml")
	if err := os.MkdirAll(statuslineDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Load should skip the directory and fall back to default
	cfg, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.Display.SingleLine {
		t.Error("Load() should return default config when .claude/statusline.yaml is a directory")
	}
}

func TestLoad_GlobalConfigFound(t *testing.T) {
	// Arrange: set HOME to a temp dir with global config, no project config
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	globalClaudeDir := filepath.Join(homeDir, ".claude")
	requireGoDir(t, os.MkdirAll(globalClaudeDir, 0755))

	configYAML := `
format:
  progressBar: ascii
`
	requireGoDir(t, os.WriteFile(filepath.Join(globalClaudeDir, "statusline.yaml"), []byte(configYAML), 0644))

	projectDir := t.TempDir()

	// Act
	cfg, err := Load(projectDir)

	// Assert
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.Format.ProgressBar != "ascii" {
		t.Errorf("Load() should return global config, got ProgressBar=%q", cfg.Format.ProgressBar)
	}
}

// requireGoDir is a minimal helper (config tests don't use testify).
func requireGoDir(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_GlobalConfigIsDir(t *testing.T) {
	// Arrange: global config path is a directory, should skip to default
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	globalClaudeDir := filepath.Join(homeDir, ".claude")
	requireGoDir(t, os.MkdirAll(globalClaudeDir, 0755))

	// Create statusline.yaml as a directory
	globalConfigDir := filepath.Join(globalClaudeDir, "statusline.yaml")
	requireGoDir(t, os.MkdirAll(globalConfigDir, 0755))

	projectDir := t.TempDir()

	// Act
	cfg, err := Load(projectDir)

	// Assert: should fall back to default
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.Format.ProgressBar != "braille" {
		t.Errorf("Load() should return default config, got ProgressBar=%q", cfg.Format.ProgressBar)
	}
}
