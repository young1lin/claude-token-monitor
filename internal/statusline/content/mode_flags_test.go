package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripANSI removes ANSI colour escapes so assertions can compare on the
// visible text. We keep the colour codes in the rendered output (they're
// what users see in their terminal) but the test only cares about content.
func stripANSI(s string) string {
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			return s
		}
		end := strings.Index(s[start:], "m")
		if end < 0 {
			return s
		}
		s = s[:start] + s[start+end+1:]
	}
}

func TestBuildModeFlags_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		effort   string
		thinking bool
		fastMode bool
		want     string // expected output with ANSI stripped
	}{
		{
			name: "all defaults — hide",
			want: "",
		},
		{
			name:   "medium effort alone — hide (medium is the implicit default)",
			effort: "medium",
			want:   "",
		},
		{
			name:     "thinking only",
			thinking: true,
			want:     "💭",
		},
		{
			name:     "thinking + xhigh — the captured CC payload shape",
			thinking: true,
			effort:   "xhigh",
			want:     "💭 xhigh",
		},
		{
			name:     "thinking + high",
			thinking: true,
			effort:   "high",
			want:     "💭 high",
		},
		{
			name:     "thinking + low",
			thinking: true,
			effort:   "low",
			want:     "💭 low",
		},
		{
			name:     "fast mode only",
			fastMode: true,
			want:     "⚡",
		},
		{
			name:     "full combo",
			thinking: true,
			fastMode: true,
			effort:   "xhigh",
			want:     "💭 ⚡ xhigh",
		},
		{
			name:   "unknown future tier value falls through to no-chip",
			effort: "ultra-mega",
			want:   "",
		},
		{
			name:   "case-insensitive tier match",
			effort: "XHIGH",
			want:   "xhigh",
		},
		{
			name:   "whitespace tolerated",
			effort: "  high  ",
			want:   "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in StatusLineInput
			in.Effort.Level = tt.effort
			in.Thinking.Enabled = tt.thinking
			in.FastMode = tt.fastMode

			got := buildModeFlags(&in)
			assert.Equal(t, tt.want, stripANSI(got))
		})
	}
}

func TestBuildModeFlags_ColorsAppliedToEffort(t *testing.T) {
	// xhigh / high / low must each carry their distinct ANSI colour so the
	// user sees the tier as a visual signal before reading the word.
	cases := map[string]string{
		"xhigh": colorEffortXHigh,
		"high":  colorEffortHigh,
		"low":   colorEffortLow,
	}
	for level, wantPrefix := range cases {
		t.Run(level, func(t *testing.T) {
			var in StatusLineInput
			in.Effort.Level = level
			got := buildModeFlags(&in)
			assert.Contains(t, got, wantPrefix, "%s should carry its colour", level)
			assert.Contains(t, got, colorReset, "every chip must end with reset")
		})
	}
}

func TestModeFlagsCollector_Collect_FromRealCCPayload(t *testing.T) {
	// Lock the integration to the captured CC 2.1.150 fixture: it carries
	// thinking=true and effort=xhigh, so the chip MUST surface "💭 xhigh".
	// If CC ever renames the JSON keys, this test catches it.
	input := loadRealCCInput(t)

	got, err := NewModeFlagsCollector().Collect(input, nil)
	require.NoError(t, err)
	assert.Equal(t, "💭 xhigh", stripANSI(got))
}

func TestModeFlagsCollector_Collect_InvalidInput(t *testing.T) {
	_, err := NewModeFlagsCollector().Collect("not a StatusLineInput", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input type")
}

func TestModeFlagsCollector_Properties(t *testing.T) {
	c := NewModeFlagsCollector()
	assert.Equal(t, ContentModeFlags, c.Type())
	assert.True(t, c.Optional(), "mode-flags must be optional so the cell hides when output is empty")
}
