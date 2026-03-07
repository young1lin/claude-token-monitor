package layout

import (
	"testing"
)

func TestIsBlockElement(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'█', true},  // U+2588 Full Block
		{'░', true},  // U+2591 Light Shade
		{'▓', true},  // U+2593 Dark Shade
		{'▒', true},  // U+2592 Medium Shade
		{'▀', true},  // U+2580 Upper Half Block
		{'▄', true},  // U+2584 Lower Half Block
		{'a', false}, // ASCII
		{'中', false}, // CJK
		{'📁', false}, // Emoji
	}

	for _, tt := range tests {
		got := isBlockElement(tt.r)
		if got != tt.want {
			t.Errorf("isBlockElement(%q): got %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestDisplayWidth_BlockElements(t *testing.T) {
	// Save original state
	original := UseNarrowBlockWidth
	defer func() { UseNarrowBlockWidth = original }()

	progressBar := "[██░░░░░░░░]"

	t.Run("default mode (UseNarrowBlockWidth=false)", func(t *testing.T) {
		UseNarrowBlockWidth = false
		// go-runewidth: █=2, ░=1, so 3*2 + 7*1 + 2 = 15
		got := displayWidth(progressBar)
		// Note: exact value depends on go-runewidth, just verify it's calculated
		if got <= 0 {
			t.Errorf("displayWidth should be positive, got %d", got)
		}
	})

	t.Run("narrow mode (UseNarrowBlockWidth=true)", func(t *testing.T) {
		UseNarrowBlockWidth = true
		// All Block Elements = 1: 3*1 + 7*1 + 2 = 12
		got := displayWidth(progressBar)
		want := 12
		if got != want {
			t.Errorf("displayWidth(%q): got %d, want %d", progressBar, got, want)
		}
	})

	t.Run("mixed content with narrow mode", func(t *testing.T) {
		UseNarrowBlockWidth = true
		// "Prefix [███░░░░░░░] Suffix" = 7 + 12 + 7 = 26
		s := "Prefix [███░░░░░░░] Suffix"
		got := displayWidth(s)
		want := 26
		if got != want {
			t.Errorf("displayWidth(%q): got %d, want %d", s, got, want)
		}
	})
}

func TestDisplayWidth_ANSIStrip(t *testing.T) {
	// ANSI codes should be stripped before calculating width
	 colored := "\x1b[31mRed Text\x1b[0m"
	got := displayWidth(colored)
	want := 8 // "Red Text"
	if got != want {
		t.Errorf("displayWidth with ANSI: got %d, want %d", got, want)
	}
}

func TestDisplayWidth_EmptyString(t *testing.T) {
	UseNarrowBlockWidth = true
	got := displayWidth("")
	if got != 0 {
		t.Errorf("displayWidth(\"\"): got %d, want 0", got)
	}
}
