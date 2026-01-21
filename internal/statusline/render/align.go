package render

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// Align provides text alignment utilities
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Measure returns the display width of a string
// This correctly handles emoji and wide characters (e.g., CJK)
func Measure(s string) int {
	return runewidth.StringWidth(s)
}

// PadLeft adds padding to the left of a string
func PadLeft(s string, width int) string {
	currentWidth := Measure(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return strings.Repeat(" ", padding) + s
}

// PadRight adds padding to the right of a string
func PadRight(s string, width int) string {
	currentWidth := Measure(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

// PadCenter centers a string within the given width
func PadCenter(s string, width int) string {
	currentWidth := Measure(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	leftPadding := padding / 2
	rightPadding := padding - leftPadding
	return strings.Repeat(" ", leftPadding) + s + strings.Repeat(" ", rightPadding)
}

// Truncate truncates a string to the given display width
func Truncate(s string, width int) string {
	if Measure(s) <= width {
		return s
	}
	runes := []rune(s)
	result := []rune{}
	currentWidth := 0
	for _, r := range runes {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > width {
			break
		}
		result = append(result, r)
		currentWidth += rw
	}
	return string(result)
}
