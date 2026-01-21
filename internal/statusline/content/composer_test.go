// Package content provides content composition for the statusline
package content

import (
	"testing"
)

func TestBaseComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *BaseComposer
		contents   map[ContentType]string
		want       string
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

func TestSimpleComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *SimpleComposer
		contents   map[ContentType]string
		want       string
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

func TestFormatComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *FormatComposer
		contents   map[ContentType]string
		want       string
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

func TestConditionalComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *ConditionalComposer
		contents   map[ContentType]string
		want       string
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

func TestPassthroughComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *PassthroughComposer
		contents   map[ContentType]string
		want       string
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
