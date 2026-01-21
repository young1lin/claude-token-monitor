// Package composers provides built-in content composers for the statusline
package composers

import (
	"testing"

	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

func TestTimeQuotaComposer_Compose(t *testing.T) {
	tests := []struct {
		name       string
		composer   *TimeQuotaComposer
		contents   map[content.ContentType]string
		want       string
	}{
		{
			name:     "default composer with time and quota",
			composer: NewTimeQuotaComposer(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
				content.ContentQuota:       "⚡ 115/120 req",
			},
			want: "14:30 | ⚡ 115/120 req",
		},
		{
			name:     "default composer with time only",
			composer: NewTimeQuotaComposer(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
				content.ContentQuota:       "",
			},
			want: "14:30",
		},
		{
			name:     "default composer with quota only",
			composer: NewTimeQuotaComposer(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "",
				content.ContentQuota:       "⚡ 115/120 req",
			},
			want: "⚡ 115/120 req",
		},
		{
			name:     "default composer empty",
			composer: NewTimeQuotaComposer(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "",
				content.ContentQuota:       "",
			},
			want: "",
		},
		{
			name:     "time only composer",
			composer: NewTimeQuotaComposerTimeOnly(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
			},
			want: "14:30",
		},
		{
			name:     "time only composer with empty time",
			composer: NewTimeQuotaComposerTimeOnly(),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "",
			},
			want: "",
		},
		{
			name: "custom config with custom separator",
			composer: NewTimeQuotaComposerFromConfig(TimeQuotaComposerConfig{
				Name:      "custom",
				ShowQuota: true,
				Separator: " | ",
			}),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
				content.ContentQuota:       "⚡ 115/120 req",
			},
			want: "14:30 | ⚡ 115/120 req",
		},
		{
			name: "custom config with different separator",
			composer: NewTimeQuotaComposerFromConfig(TimeQuotaComposerConfig{
				Name:      "custom",
				ShowQuota: true,
				Separator: " • ",
			}),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
				content.ContentQuota:       "⚡ 115/120 req",
			},
			want: "14:30 • ⚡ 115/120 req",
		},
		{
			name: "custom config without quota",
			composer: NewTimeQuotaComposerFromConfig(TimeQuotaComposerConfig{
				Name:      "custom",
				ShowQuota: false,
			}),
			contents: map[content.ContentType]string{
				content.ContentCurrentTime: "14:30",
				content.ContentQuota:       "⚡ 115/120 req",
			},
			want: "14:30",
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

func TestTimeQuotaComposer_Name(t *testing.T) {
	tests := []struct {
		name     string
		composer *TimeQuotaComposer
		want     string
	}{
		{
			name:     "default composer name",
			composer: NewTimeQuotaComposer(),
			want:     "time-quota",
		},
		{
			name:     "time only name",
			composer: NewTimeQuotaComposerTimeOnly(),
			want:     "time-only",
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

func TestTimeQuotaComposer_InputTypes(t *testing.T) {
	tests := []struct {
		name        string
		composer    *TimeQuotaComposer
		wantLength  int
		wantContains []content.ContentType
	}{
		{
			name:       "default composer input types",
			composer:   NewTimeQuotaComposer(),
			wantLength: 2,
			wantContains: []content.ContentType{
				content.ContentCurrentTime,
				content.ContentQuota,
			},
		},
		{
			name:       "time only input types",
			composer:   NewTimeQuotaComposerTimeOnly(),
			wantLength: 1,
			wantContains: []content.ContentType{
				content.ContentCurrentTime,
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
