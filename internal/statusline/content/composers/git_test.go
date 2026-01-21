// Package composers provides built-in content composers for the statusline
package composers

import (
	"testing"

	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

func TestGitComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *GitComposer
		contents   map[content.ContentType]string
		want       string
	}{
		{
			name:     "default composer with all fields",
			composer: NewGitComposer(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
				content.ContentGitStatus: "+3 ~2",
				content.ContentGitRemote: "🔄",
			},
			want: "🌿 main +3 ~2 🔄",
		},
		{
			name:     "default composer with branch and status",
			composer: NewGitComposer(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
				content.ContentGitStatus: "+5",
				content.ContentGitRemote: "",
			},
			want: "🌿 main +5",
		},
		{
			name:     "default composer with branch only",
			composer: NewGitComposer(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
				content.ContentGitStatus: "",
				content.ContentGitRemote: "",
			},
			want: "🌿 main",
		},
		{
			name:     "default composer with status only",
			composer: NewGitComposer(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "",
				content.ContentGitStatus: "+3",
				content.ContentGitRemote: "",
			},
			want: "+3",
		},
		{
			name:     "default composer empty",
			composer: NewGitComposer(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "",
				content.ContentGitStatus: "",
				content.ContentGitRemote: "",
			},
			want: "",
		},
		{
			name:     "branch only composer",
			composer: NewGitComposerBranchOnly(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
			},
			want: "🌿 main",
		},
		{
			name:     "branch only with empty branch",
			composer: NewGitComposerBranchOnly(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "",
			},
			want: "",
		},
		{
			name:     "branch and status composer",
			composer: NewGitComposerWithStatus(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "feature-branch",
				content.ContentGitStatus: "*2",
			},
			want: "🌿 feature-branch *2",
		},
		{
			name:     "branch and status with empty status",
			composer: NewGitComposerWithStatus(),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
				content.ContentGitStatus: "",
			},
			want: "🌿 main",
		},
		{
			name: "custom config without branch prefix",
			composer: NewGitComposerFromConfig(GitComposerConfig{
				Name:         "custom",
				ShowStatus:   false,
				ShowRemote:   false,
				BranchPrefix: "",
			}),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
			},
			want: "main",
		},
		{
			name: "custom config with custom branch prefix",
			composer: NewGitComposerFromConfig(GitComposerConfig{
				Name:         "custom",
				ShowStatus:   true,
				ShowRemote:   false,
				BranchPrefix: "⎇ ",
			}),
			contents: map[content.ContentType]string{
				content.ContentGitBranch: "main",
				content.ContentGitStatus: "+3",
			},
			want: "⎇ main +3",
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

func TestGitComposer_Name(t *testing.T) {
	tests := []struct {
		name     string
		composer *GitComposer
		want     string
	}{
		{
			name:     "default composer name",
			composer: NewGitComposer(),
			want:     "git",
		},
		{
			name:     "branch only name",
			composer: NewGitComposerBranchOnly(),
			want:     "git-branch-only",
		},
		{
			name:     "branch and status name",
			composer: NewGitComposerWithStatus(),
			want:     "git-branch-status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.composer.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitComposer_InputTypes(t *testing.T) {
	tests := []struct {
		name        string
		composer    *GitComposer
		wantLength  int
		wantContains []content.ContentType
	}{
		{
			name:       "default composer input types",
			composer:   NewGitComposer(),
			wantLength: 3,
			wantContains: []content.ContentType{
				content.ContentGitBranch,
				content.ContentGitStatus,
				content.ContentGitRemote,
			},
		},
		{
			name:       "branch only input types",
			composer:   NewGitComposerBranchOnly(),
			wantLength: 1,
			wantContains: []content.ContentType{
				content.ContentGitBranch,
			},
		},
		{
			name:       "branch and status input types",
			composer:   NewGitComposerWithStatus(),
			wantLength: 2,
			wantContains: []content.ContentType{
				content.ContentGitBranch,
				content.ContentGitStatus,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types := tt.composer.InputTypes()
			if len(types) != tt.wantLength {
				t.Errorf("InputTypes() length = %d, want %d", len(types), tt.wantLength)
			}
			for _, ct := range tt.wantContains {
				found := false
				for _, t := range types {
					if t == ct {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("InputTypes() does not contain %v", ct)
				}
			}
		})
	}
}
