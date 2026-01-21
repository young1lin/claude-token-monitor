// Package composers provides built-in content composers for the statusline
package composers

import (
	"testing"

	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

func TestTokenComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *TokenComposer
		contents   map[content.ContentType]string
		want       string
	}{
		{
			name:     "default composer with all fields",
			composer: NewTokenComposer(),
			contents: map[content.ContentType]string{
				content.ContentModel:     "GLM-4.7",
				content.ContentTokenBar:  "███░░░░",
				content.ContentTokenInfo: "75K/200K",
			},
			want: "[GLM-4.7 ███░░░░ 75K/200K]",
		},
		{
			name:     "default composer without token bar",
			composer: NewTokenComposer(),
			contents: map[content.ContentType]string{
				content.ContentModel:     "Sonnet 4.5",
				content.ContentTokenBar:  "",
				content.ContentTokenInfo: "50K/200K",
			},
			want: "[Sonnet 4.5 50K/200K]",
		},
		{
			name:     "default composer without token info",
			composer: NewTokenComposer(),
			contents: map[content.ContentType]string{
				content.ContentModel:     "GLM-4.7",
				content.ContentTokenBar:  "███░░░░",
				content.ContentTokenInfo: "",
			},
			want: "[GLM-4.7 ███░░░░]",
		},
		{
			name:     "default composer with empty model uses default",
			composer: NewTokenComposer(),
			contents: map[content.ContentType]string{
				content.ContentModel:     "",
				content.ContentTokenBar:  "███░░░░",
				content.ContentTokenInfo: "75K/200K",
			},
			want: "[Claude ███░░░░ 75K/200K]",
		},
		{
			name:     "simple composer with model and bar",
			composer: NewTokenComposerSimple(),
			contents: map[content.ContentType]string{
				content.ContentModel:    "GLM-4.7",
				content.ContentTokenBar: "███░░░░",
			},
			want: "[GLM-4.7 ███░░░░]",
		},
		{
			name:     "simple composer with empty bar",
			composer: NewTokenComposerSimple(),
			contents: map[content.ContentType]string{
				content.ContentModel:    "Sonnet 4.5",
				content.ContentTokenBar: "",
			},
			want: "[Sonnet 4.5]",
		},
		{
			name:     "model only composer",
			composer: NewTokenComposerModelOnly(),
			contents: map[content.ContentType]string{
				content.ContentModel: "GLM-4.7",
			},
			want: "[GLM-4.7]",
		},
		{
			name:     "model only with empty model uses default",
			composer: NewTokenComposerModelOnly(),
			contents: map[content.ContentType]string{
				content.ContentModel: "",
			},
			want: "[Claude]",
		},
		{
			name: "custom config with model prefix",
			composer: NewTokenComposerFromConfig(TokenComposerConfig{
				Name:        "custom",
				ShowBar:     false,
				ShowInfo:    false,
				ModelPrefix: "🤖 ",
			}),
			contents: map[content.ContentType]string{
				content.ContentModel: "GLM-4.7",
			},
			want: "🤖 GLM-4.7",
		},
		{
			name: "custom config with custom prefix/suffix",
			composer: NewTokenComposerFromConfig(TokenComposerConfig{
				Name:    "custom",
				ShowBar: true,
				ShowInfo: false,
				Prefix:  "{",
				Suffix:  "}",
			}),
			contents: map[content.ContentType]string{
				content.ContentModel:    "GLM-4.7",
				content.ContentTokenBar: "███░░░░",
			},
			want: "{GLM-4.7 ███░░░░}",
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

func TestTokenComposer_Name(t *testing.T) {
	tests := []struct {
		name     string
		composer *TokenComposer
		want     string
	}{
		{
			name:     "default composer name",
			composer: NewTokenComposer(),
			want:     "token",
		},
		{
			name:     "simple composer name",
			composer: NewTokenComposerSimple(),
			want:     "token-simple",
		},
		{
			name:     "model only name",
			composer: NewTokenComposerModelOnly(),
			want:     "token-model-only",
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

func TestTokenComposer_InputTypes(t *testing.T) {
	tests := []struct {
		name        string
		composer    *TokenComposer
		wantLength  int
		wantContains []content.ContentType
	}{
		{
			name:       "default composer input types",
			composer:   NewTokenComposer(),
			wantLength: 3,
			wantContains: []content.ContentType{
				content.ContentModel,
				content.ContentTokenBar,
				content.ContentTokenInfo,
			},
		},
		{
			name:       "simple composer input types",
			composer:   NewTokenComposerSimple(),
			wantLength: 2,
			wantContains: []content.ContentType{
				content.ContentModel,
				content.ContentTokenBar,
			},
		},
		{
			name:       "model only input types",
			composer:   NewTokenComposerModelOnly(),
			wantLength: 1,
			wantContains: []content.ContentType{
				content.ContentModel,
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
