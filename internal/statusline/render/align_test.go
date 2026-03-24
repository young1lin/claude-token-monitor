package render

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMeasure verifies that Measure correctly returns display width for
// ASCII, empty strings, emoji, and CJK characters.
func TestMeasure(t *testing.T) {
	// Ensure consistent emoji width across platforms by enabling EastAsianWidth.
	original := runewidth.EastAsianWidth
	runewidth.EastAsianWidth = true
	defer func() { runewidth.EastAsianWidth = original }()

	tests := []struct {
		name  string
		input string
		// We use a callback instead of a fixed want to make tests resilient
		// to runewidth version differences while still validating invariants.
		check func(t *testing.T, got int)
	}{
		{
			name:  "empty string",
			input: "",
			check: func(t *testing.T, got int) {
				assert.Equal(t, 0, got, "empty string should have width 0")
			},
		},
		{
			name:  "simple ASCII",
			input: "hello",
			check: func(t *testing.T, got int) {
				assert.Equal(t, 5, got, "ASCII characters should have width 1 each")
			},
		},
		{
			name:  "ASCII with spaces",
			input: "a b c",
			check: func(t *testing.T, got int) {
				assert.Equal(t, 5, got, "spaces count as width 1")
			},
		},
		{
			name:  "single emoji",
			input: "\U0001F4C1",
			check: func(t *testing.T, got int) {
				// Emoji width depends on terminal; with EastAsianWidth=true it is 2.
				assert.GreaterOrEqual(t, got, 1, "emoji should have positive width")
			},
		},
		{
			name:  "two emoji",
			input: "\U0001F4C1\U0001F3AF",
			check: func(t *testing.T, got int) {
				singleWidth := Measure("\U0001F4C1")
				assert.Equal(t, 2*singleWidth, got, "two emoji should be double one emoji width")
			},
		},
		{
			name:  "CJK characters",
			input: "\u4e2d\u6587",
			check: func(t *testing.T, got int) {
				assert.Equal(t, 4, got, "CJK characters should have width 2 each")
			},
		},
		{
			name:  "mixed ASCII and CJK",
			input: "a\u4e2d",
			check: func(t *testing.T, got int) {
				assert.Equal(t, 3, got, "a (1) + CJK char (2) = 3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			input := tt.input

			// Act
			got := Measure(input)

			// Assert
			tt.check(t, got)
		})
	}
}

// TestPadLeft verifies left-padding behavior for strings shorter, longer,
// or equal to the target width.
func TestPadLeft(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		width     int
		checkFunc func(t *testing.T, got string, width int)
	}{
		{
			name:  "string shorter than width",
			input: "ab",
			width: 5,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 5, Measure(got), "padded result should have target display width")
				assert.True(t, len(got) > 2, "should have leading spaces")
				assert.Equal(t, "ab", got[len(got)-2:], "original string should be at the end")
			},
		},
		{
			name:  "string equal to width",
			input: "abc",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abc", got, "no padding should be added")
			},
		},
		{
			name:  "string longer than width",
			input: "abcdef",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abcdef", got, "string longer than width should be returned unchanged")
			},
		},
		{
			name:  "empty string with positive width",
			input: "",
			width: 4,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 4, len(got), "should pad empty string to width")
				assert.Equal(t, "    ", got, "should be all spaces")
			},
		},
		{
			name:  "zero width",
			input: "ab",
			width: 0,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "ab", got, "zero width should return original string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			got := PadLeft(tt.input, tt.width)

			// Assert
			tt.checkFunc(t, got, tt.width)
		})
	}
}

// TestPadRight verifies right-padding behavior for strings shorter, longer,
// or equal to the target width.
func TestPadRight(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		width     int
		checkFunc func(t *testing.T, got string, width int)
	}{
		{
			name:  "string shorter than width",
			input: "ab",
			width: 5,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 5, Measure(got), "padded result should have target display width")
				assert.True(t, len(got) > 2, "should have trailing spaces")
				assert.Equal(t, "ab", got[:2], "original string should be at the start")
			},
		},
		{
			name:  "string equal to width",
			input: "abc",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abc", got, "no padding should be added")
			},
		},
		{
			name:  "string longer than width",
			input: "abcdef",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abcdef", got, "string longer than width should be returned unchanged")
			},
		},
		{
			name:  "empty string with positive width",
			input: "",
			width: 4,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 4, len(got), "should pad empty string to width")
				assert.Equal(t, "    ", got, "should be all spaces")
			},
		},
		{
			name:  "zero width",
			input: "ab",
			width: 0,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "ab", got, "zero width should return original string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			got := PadRight(tt.input, tt.width)

			// Assert
			tt.checkFunc(t, got, tt.width)
		})
	}
}

// TestPadCenter verifies centering behavior including the case where the
// remaining padding is odd (left gets one fewer space).
func TestPadCenter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		width     int
		checkFunc func(t *testing.T, got string, width int)
	}{
		{
			name:  "string shorter than width - even padding",
			input: "ab",
			width: 6,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 6, Measure(got), "centered result should have target display width")
				// 6 - 2 = 4 padding, left=2, right=2
				assert.Equal(t, "  ab  ", got, "should be centered with equal padding")
			},
		},
		{
			name:  "string shorter than width - odd padding",
			input: "ab",
			width: 5,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 5, Measure(got), "centered result should have target display width")
				// 5 - 2 = 3 padding, left=1, right=2 (extra space goes to the right)
				assert.Equal(t, " ab  ", got, "odd padding should add extra space to the right")
			},
		},
		{
			name:  "string equal to width",
			input: "abc",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abc", got, "no padding should be added")
			},
		},
		{
			name:  "string longer than width",
			input: "abcdef",
			width: 3,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "abcdef", got, "string longer than width should be returned unchanged")
			},
		},
		{
			name:  "empty string with positive width",
			input: "",
			width: 4,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, 4, Measure(got), "should pad empty string to width")
				// 4 / 2 = 2 left, 4 - 2 = 2 right
				assert.Equal(t, "    ", got)
			},
		},
		{
			name:  "single char centered in width 4",
			input: "x",
			width: 4,
			checkFunc: func(t *testing.T, got string, width int) {
				// 4 - 1 = 3 padding, left=1, right=2
				assert.Equal(t, 4, Measure(got))
				assert.Equal(t, " x  ", got, "extra space goes to the right for odd padding")
			},
		},
		{
			name:  "zero width",
			input: "ab",
			width: 0,
			checkFunc: func(t *testing.T, got string, width int) {
				assert.Equal(t, "ab", got, "zero width should return original string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			got := PadCenter(tt.input, tt.width)

			// Assert
			tt.checkFunc(t, got, tt.width)
		})
	}
}

// TestTruncate verifies truncation behavior including boundary cases where
// a wide character would exceed the target width.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "string shorter than width - unchanged",
			input: "hello",
			width: 10,
			want:  "hello",
		},
		{
			name:  "string equal to width - unchanged",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "string longer than width - truncated",
			input: "hello world",
			width: 5,
			want:  "hello",
		},
		{
			name:  "truncate to zero width",
			input: "hello",
			width: 0,
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			width: 5,
			want:  "",
		},
		{
			name:  "truncate stops before wide character that would exceed width",
			input: "ab\u4e2d\u6587",
			width: 3,
			want:  "ab",
		},
		{
			name:  "truncate includes wide character when it fits exactly",
			input: "ab\u4e2d\u6587",
			width: 4,
			want:  "ab\u4e2d",
		},
		{
			name:  "truncate before wide character",
			input: "\u4e2d\u6587",
			width: 1,
			want:  "",
		},
		{
			name:  "truncate to exact wide character width",
			input: "\u4e2d\u6587",
			width: 4,
			want:  "\u4e2d\u6587",
		},
		{
			name:  "truncate with zero width input",
			input: "hello",
			width: 0,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			got := Truncate(tt.input, tt.width)

			// Assert
			assert.Equal(t, tt.want, got)

			// Additional invariant: truncated result width should never exceed target
			resultWidth := Measure(got)
			require.LessOrEqual(t, resultWidth, tt.width,
				"truncated width %d should not exceed target width %d", resultWidth, tt.width)
		})
	}
}
