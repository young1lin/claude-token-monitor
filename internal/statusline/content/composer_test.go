// Package content provides content composition for the statusline
package content

import (
	"testing"
)

func TestBaseComposer_Compose(t *testing.T) {
	tests := []struct {
		name     string
		composer *BaseComposer
		contents map[ContentType]string
		want     string
	}{
		{
			name: "simple template with dot notation",
			composer: NewBaseComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}, "{{.model}} {{.token-info}}"),
			contents: map[ContentType]string{
				ContentModel:     "GLM-4.7",
				ContentTokenInfo: "75K/200K",
			},
			want: "GLM-4.7 75K/200K",
		},
		{
			name: "template with prefix/suffix",
			composer: NewBaseComposer("test", []ContentType{
				ContentModel,
			}, "[{{.model}}]"),
			contents: map[ContentType]string{
				ContentModel: "Sonnet 4.5",
			},
			want: "[Sonnet 4.5]",
		},
		{
			name: "empty template",
			composer: NewBaseComposer("test", []ContentType{
				ContentModel,
			}, ""),
			contents: map[ContentType]string{
				ContentModel: "GLM-4.7",
			},
			want: "",
		},
		{
			name: "missing content uses fallback",
			composer: NewBaseComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}, "{{.model}} {{.token-info}}"),
			contents: map[ContentType]string{
				ContentModel: "GLM-4.7",
				// token-info missing
			},
			want: "GLM-4.7",
		},
		{
			name: "complex template with conditional using index notation",
			composer: NewBaseComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
			}, "🌿 {{index . \"git-branch\"}}{{if (index . \"git-status\")}} {{index . \"git-status\"}}{{end}}"),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				ContentGitStatus: "+3 ~2",
			},
			want: "🌿 main +3 ~2",
		},
		{
			name: "conditional template section using index notation",
			composer: NewBaseComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
			}, "🌿 {{index . \"git-branch\"}}{{if (index . \"git-status\")}} {{index . \"git-status\"}}{{end}}"),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				// git-status empty
			},
			want: "🌿 main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.composer.Compose(tt.contents)
			if got != tt.want {
				t.Errorf("Compose() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBaseComposer_Name(t *testing.T) {
	c := NewBaseComposer("test-composer", []ContentType{ContentModel}, "")
	if got := c.Name(); got != "test-composer" {
		t.Errorf("Name() = %q, want %q", got, "test-composer")
	}
}

func TestBaseComposer_InputTypes(t *testing.T) {
	types := []ContentType{ContentModel, ContentTokenBar, ContentTokenInfo}
	c := NewBaseComposer("test", types, "")

	got := c.InputTypes()
	if len(got) != len(types) {
		t.Errorf("InputTypes() length = %d, want %d", len(got), len(types))
	}

	for i, ct := range got {
		if ct != types[i] {
			t.Errorf("InputTypes()[%d] = %v, want %v", i, ct, types[i])
		}
	}
}

func TestSimpleComposer_NameAndInputTypes(t *testing.T) {
	c := NewSimpleComposer("git-info", []ContentType{ContentGitBranch, ContentGitStatus}, " | ", "", "")
	if c.Name() != "git-info" {
		t.Errorf("Name() = %q, want %q", c.Name(), "git-info")
	}
	types := c.InputTypes()
	if len(types) != 2 || types[0] != ContentGitBranch || types[1] != ContentGitStatus {
		t.Errorf("InputTypes() = %v, want [git-branch, git-status]", types)
	}
}

func TestSimpleComposer_Compose(t *testing.T) {
	tests := []struct {
		name     string
		composer *SimpleComposer
		contents map[ContentType]string
		want     string
	}{
		{
			name: "join with space",
			composer: NewSimpleComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}, " ", "", ""),
			contents: map[ContentType]string{
				ContentModel:     "GLM-4.7",
				ContentTokenInfo: "75K/200K",
			},
			want: "GLM-4.7 75K/200K",
		},
		{
			name: "join with pipe",
			composer: NewSimpleComposer("test", []ContentType{
				ContentFolder,
				ContentModel,
			}, " | ", "", ""),
			contents: map[ContentType]string{
				ContentFolder: "📁 project",
				ContentModel:  "GLM-4.7",
			},
			want: "📁 project | GLM-4.7",
		},
		{
			name: "with prefix and suffix",
			composer: NewSimpleComposer("test", []ContentType{
				ContentModel,
			}, "", "[", "]"),
			contents: map[ContentType]string{
				ContentModel: "Sonnet 4.5",
			},
			want: "[Sonnet 4.5]",
		},
		{
			name: "skip empty values",
			composer: NewSimpleComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
				ContentGitRemote,
			}, " ", "", ""),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				ContentGitStatus: "+3",
				// git-remote empty
			},
			want: "main +3",
		},
		{
			name: "all empty returns empty",
			composer: NewSimpleComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}, " ", "", ""),
			contents: map[ContentType]string{
				ContentModel:     "",
				ContentTokenInfo: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.composer.Compose(tt.contents)
			if got != tt.want {
				t.Errorf("Compose() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatComposer_NameAndInputTypes(t *testing.T) {
	c := NewFormatComposer("fmt", []ContentType{ContentModel}, func(map[ContentType]string) string { return "" })
	if c.Name() != "fmt" {
		t.Errorf("Name() = %q, want %q", c.Name(), "fmt")
	}
	if len(c.InputTypes()) != 1 || c.InputTypes()[0] != ContentModel {
		t.Errorf("InputTypes() = %v, want [model]", c.InputTypes())
	}
}

func TestFormatComposer_Compose(t *testing.T) {
	tests := []struct {
		name     string
		composer *FormatComposer
		contents map[ContentType]string
		want     string
	}{
		{
			name: "custom format function",
			composer: NewFormatComposer("test", []ContentType{
				ContentModel,
				ContentTokenBar,
				ContentTokenInfo,
			}, func(contents map[ContentType]string) string {
				model := contents[ContentModel]
				bar := contents[ContentTokenBar]
				info := contents[ContentTokenInfo]
				result := model
				if bar != "" {
					result += " " + bar
				}
				if info != "" {
					result += " " + info
				}
				return "[" + result + "]"
			}),
			contents: map[ContentType]string{
				ContentModel:     "GLM-4.7",
				ContentTokenBar:  "███░░░░",
				ContentTokenInfo: "75K/200K",
			},
			want: "[GLM-4.7 ███░░░░ 75K/200K]",
		},
		{
			name: "nil format function",
			composer: NewFormatComposer("test", []ContentType{
				ContentModel,
			}, nil),
			contents: map[ContentType]string{
				ContentModel: "GLM-4.7",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.composer.Compose(tt.contents)
			if got != tt.want {
				t.Errorf("Compose() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConditionalComposer_NameAndInputTypes(t *testing.T) {
	c := NewConditionalComposer("cond", []ContentType{ContentModel, ContentTokenBar}, nil)
	if c.Name() != "cond" {
		t.Errorf("Name() = %q, want %q", c.Name(), "cond")
	}
	types := c.InputTypes()
	if len(types) != 2 {
		t.Errorf("InputTypes() length = %d, want 2", len(types))
	}
}

func TestConditionalComposer_Compose(t *testing.T) {
	tests := []struct {
		name     string
		composer *ConditionalComposer
		contents map[ContentType]string
		want     string
	}{
		{
			name: "matches first pattern with all fields",
			composer: NewConditionalComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
				ContentGitRemote,
			}, []ConditionalPattern{
				{
					Required: []ContentType{ContentGitBranch},
					Optional: []ContentType{ContentGitStatus, ContentGitRemote},
					Format:   "🌿 {{index . \"git-branch\"}} {{index . \"git-status\"}} {{index . \"git-remote\"}}",
				},
			}),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				ContentGitStatus: "+3",
				ContentGitRemote: "🔄",
			},
			want: "🌿 main +3 🔄",
		},
		{
			name: "matches first pattern with partial fields",
			composer: NewConditionalComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
				ContentGitRemote,
			}, []ConditionalPattern{
				{
					Required: []ContentType{ContentGitBranch},
					Optional: []ContentType{ContentGitStatus, ContentGitRemote},
					Format:   "🌿 {{index . \"git-branch\"}}{{if (index . \"git-status\")}} {{index . \"git-status\"}}{{end}}{{if (index . \"git-remote\")}} {{index . \"git-remote\"}}{{end}}",
				},
			}),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				ContentGitStatus: "+3",
				// git-remote empty
			},
			want: "🌿 main +3",
		},
		{
			name: "no match returns empty",
			composer: NewConditionalComposer("test", []ContentType{
				ContentGitBranch,
				ContentGitStatus,
			}, []ConditionalPattern{
				{
					Required: []ContentType{ContentGitBranch, ContentGitStatus},
					Optional: nil,
					Format:   "{{index . \"git-branch\"}} {{index . \"git-status\"}}",
				},
			}),
			contents: map[ContentType]string{
				ContentGitBranch: "main",
				// git-status missing (required)
			},
			want: "",
		},
		{
			name: "tries patterns in order",
			composer: NewConditionalComposer("test", []ContentType{
				ContentModel,
				ContentTokenBar,
				ContentTokenInfo,
			}, []ConditionalPattern{
				{
					Required: []ContentType{ContentModel, ContentTokenBar, ContentTokenInfo},
					Optional: nil,
					Format:   "FULL: {{index . \"model\"}} {{index . \"token-bar\"}} {{index . \"token-info\"}}",
				},
				{
					Required: []ContentType{ContentModel},
					Optional: []ContentType{ContentTokenBar},
					Format:   "PARTIAL: {{index . \"model\"}}{{if (index . \"token-bar\")}} {{index . \"token-bar\"}}{{end}}",
				},
			}),
			contents: map[ContentType]string{
				ContentModel:    "GLM-4.7",
				ContentTokenBar: "███░░░░",
				// token-info missing (first pattern fails)
			},
			want: "PARTIAL: GLM-4.7 ███░░░░",
		},
		{
			name: "empty format skips pattern",
			composer: NewConditionalComposer("test", []ContentType{
				ContentModel,
			}, []ConditionalPattern{
				{
					Required: []ContentType{ContentModel},
					Optional: nil,
					Format:   "", // Empty format = skip
				},
			}),
			contents: map[ContentType]string{
				ContentModel: "GLM-4.7",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.composer.Compose(tt.contents)
			if got != tt.want {
				t.Errorf("Compose() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPassthroughComposer_NameAndInputTypes(t *testing.T) {
	c := NewPassthroughComposer("pass", []ContentType{ContentModel, ContentTokenInfo})
	if c.Name() != "pass" {
		t.Errorf("Name() = %q, want %q", c.Name(), "pass")
	}
	if len(c.InputTypes()) != 2 || c.InputTypes()[1] != ContentTokenInfo {
		t.Errorf("InputTypes() = %v, want [model, token-info]", c.InputTypes())
	}
}

func TestPassthroughComposer_Compose(t *testing.T) {
	tests := []struct {
		name     string
		composer *PassthroughComposer
		contents map[ContentType]string
		want     string
	}{
		{
			name: "returns first non-empty",
			composer: NewPassthroughComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}),
			contents: map[ContentType]string{
				ContentModel:     "GLM-4.7",
				ContentTokenInfo: "75K/200K",
			},
			want: "GLM-4.7",
		},
		{
			name: "skips empty first value",
			composer: NewPassthroughComposer("test", []ContentType{
				ContentTokenInfo,
				ContentModel,
			}),
			contents: map[ContentType]string{
				ContentTokenInfo: "",
				ContentModel:     "GLM-4.7",
			},
			want: "GLM-4.7",
		},
		{
			name: "all empty returns empty",
			composer: NewPassthroughComposer("test", []ContentType{
				ContentModel,
				ContentTokenInfo,
			}),
			contents: map[ContentType]string{
				ContentModel:     "",
				ContentTokenInfo: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.composer.Compose(tt.contents)
			if got != tt.want {
				t.Errorf("Compose() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewRegistry()
		composer := NewSimpleComposer("test", []ContentType{ContentModel}, " ", "", "")

		registry.Register(composer)

		got, ok := registry.Get("test")
		if !ok {
			t.Fatal("Get() returned false for registered composer")
		}
		if got.Name() != "test" {
			t.Errorf("Got composer with name %q, want %q", got.Name(), "test")
		}
	})

	t.Run("Get returns false for missing composer", func(t *testing.T) {
		registry := NewRegistry()

		_, ok := registry.Get("nonexistent")
		if ok {
			t.Error("Get() returned true for non-existent composer")
		}
	})

	t.Run("List returns all composer names", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(NewSimpleComposer("composer1", []ContentType{ContentModel}, " ", "", ""))
		registry.Register(NewSimpleComposer("composer2", []ContentType{ContentTokenInfo}, " ", "", ""))

		names := registry.List()

		if len(names) != 2 {
			t.Errorf("List() returned %d names, want 2", len(names))
		}

		// Check that both names are present
		nameMap := make(map[string]bool)
		for _, name := range names {
			nameMap[name] = true
		}

		if !nameMap["composer1"] || !nameMap["composer2"] {
			t.Error("List() did not return all registered composers")
		}
	})

	t.Run("MustGet panics for missing composer", func(t *testing.T) {
		registry := NewRegistry()

		defer func() {
			if r := recover(); r == nil {
				t.Error("MustGet() did not panic for missing composer")
			}
		}()

		registry.MustGet("nonexistent")
	})

	t.Run("MustGet returns existing composer", func(t *testing.T) {
		registry := NewRegistry()
		composer := NewSimpleComposer("test", []ContentType{ContentModel}, " ", "", "")
		registry.Register(composer)

		got := registry.MustGet("test")
		if got.Name() != "test" {
			t.Errorf("MustGet() returned composer with name %q, want %q", got.Name(), "test")
		}
	})
}

func TestBaseComposer_FallbackCompose(t *testing.T) {
	// Template with syntax error -> parsed will be nil, runtime parse also fails -> fallback
	composer := NewBaseComposer("fallback-test", []ContentType{
		ContentModel,
		ContentTokenBar,
	}, "{{unclosed")

	got := composer.Compose(map[ContentType]string{
		ContentModel:    "GLM-4.7",
		ContentTokenBar: "██░░░░",
	})
	// fallbackCompose joins non-empty values with space
	want := "GLM-4.7 ██░░░░"
	if got != want {
		t.Errorf("fallback Compose() = %q, want %q", got, want)
	}
}

func TestBaseComposer_FallbackCompose_AllEmpty(t *testing.T) {
	composer := NewBaseComposer("fallback-empty", []ContentType{
		ContentModel,
		ContentTokenBar,
	}, "{{unclosed")

	got := composer.Compose(map[ContentType]string{
		ContentModel:    "",
		ContentTokenBar: "",
	})
	if got != "" {
		t.Errorf("fallback Compose() with all empty = %q, want empty", got)
	}
}

func TestConditionalComposer_ExecuteError(t *testing.T) {
	// Template that parses but fails to execute (e.g., calling a non-existent function)
	composer := NewConditionalComposer("exec-test", []ContentType{
		ContentModel,
	}, []ConditionalPattern{
		{
			Required: []ContentType{ContentModel},
			Format:   `{{call .nonexistent}}`,
		},
	})

	got := composer.Compose(map[ContentType]string{
		ContentModel: "GLM-4.7",
	})
	// Execute error -> continue to next pattern -> no match -> ""
	if got != "" {
		t.Errorf("Execute error should return empty, got %q", got)
	}
}

func TestConditionalComposer_ExecuteErrorFallback(t *testing.T) {
	// Pattern matches but template execution fails → skips to next pattern
	composer := NewConditionalComposer("exec-test2", []ContentType{
		ContentModel,
	}, []ConditionalPattern{
		{
			Required: []ContentType{ContentModel},
			Format:   "{{.model.Method}}", // will fail: string has no method
		},
		{
			Required: []ContentType{ContentModel},
			Format:   "fallback: {{.model}}",
		},
	})

	got := composer.Compose(map[ContentType]string{
		ContentModel: "test",
	})
	want := "fallback: test"
	if got != want {
		t.Errorf("execute error skips to next pattern = %q, want %q", got, want)
	}
}

func TestConditionalComposer_RuntimeParseSuccess(t *testing.T) {
	// Create a composer where Parsed is explicitly nil (Format is valid but
	// we force Parsed to nil by using a pattern with Format but clearing Parsed).
	// This tests the path where tmpl == nil, runtime parse succeeds, and execute succeeds.
	composer := NewConditionalComposer("runtime-success", []ContentType{
		ContentModel,
	}, []ConditionalPattern{
		{
			Required: []ContentType{ContentModel},
			Format:   "{{.model}}",
		},
	})
	// Force Parsed to nil to simulate pre-parse failure
	composer.formatPatterns[0].Parsed = nil

	got := composer.Compose(map[ContentType]string{
		ContentModel: "GLM-4.7",
	})
	want := "GLM-4.7"
	if got != want {
		t.Errorf("Compose() = %q, want %q", got, want)
	}
}

func TestBaseComposer_ExecuteError(t *testing.T) {
	// Template that triggers execute error by calling method on nil interface.
	// Go templates with missingkey=zero don't error on missing keys,
	// but calling a method on a <nil> interface does.
	composer := NewBaseComposer("exec-err", []ContentType{
		ContentModel,
	}, "{{.model.Method}}")

	got := composer.Compose(map[ContentType]string{
		ContentModel: "test",
	})
	// Execute error → fallback compose returns "test"
	if got != "test" {
		t.Errorf("execute error fallback = %q, want %q", got, "test")
	}
}

func TestConditionalComposer_RuntimeParseFail(t *testing.T) {
	// Test the pattern where Format is empty after match:
	composer := NewConditionalComposer("runtime-fail", []ContentType{
		ContentModel,
	}, []ConditionalPattern{
		{
			Required: []ContentType{ContentModel},
			Format:   "", // empty -> skip (continue)
		},
		{
			Required: []ContentType{ContentModel},
			Format:   "result: {{.model}}", // this one should match
		},
	})

	got := composer.Compose(map[ContentType]string{
		ContentModel: "GLM-4.7",
	})
	want := "result: GLM-4.7"
	if got != want {
		t.Errorf("Compose() = %q, want %q", got, want)
	}
}

func TestConditionalComposer_RuntimeParseError(t *testing.T) {
	// Parsed is nil, and Format has invalid template syntax → runtime parse fails → continue
	composer := NewConditionalComposer("runtime-parse-err", []ContentType{
		ContentModel,
	}, []ConditionalPattern{
		{
			Required: []ContentType{ContentModel},
			Format:   "{{unclosed", // invalid syntax, parse fails at runtime
		},
		{
			Required: []ContentType{ContentModel},
			Format:   "fallback: {{.model}}",
		},
	})
	// Force Parsed to nil so the runtime parse path is exercised
	composer.formatPatterns[0].Parsed = nil

	got := composer.Compose(map[ContentType]string{
		ContentModel: "test",
	})
	want := "fallback: test"
	if got != want {
		t.Errorf("runtime parse error should skip to next pattern = %q, want %q", got, want)
	}
}
