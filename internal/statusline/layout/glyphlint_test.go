package layout

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// reviewedGlyphs is the registry of every non-ASCII rune that production code
// may emit whose rendered width is NOT self-evident. A rune is self-evident
// when every terminal and go-runewidth agree on its width: East Asian
// Wide/Fullwidth runes (emoji, CJK — always 2) and Block Elements (pinned by
// the renderer's narrow flag). Everything else — East Asian Ambiguous,
// Neutral symbols, variation selectors — renders at a width the terminal
// decides, so each such rune must be consciously reviewed and registered here
// with the evidence for its width before it ships.
//
// Widths below were verified against Windows Terminal by a user-run probe on
// 2026-07-19 (glyph ×10 + "|" ruler): Ambiguous and Neutral symbols render at
// 1 cell; base+VS16 emoji sequences render at 2 (displayWidth promotes them).
// macOS Terminal.app and iTerm2 default to the same narrow Ambiguous
// rendering. STATUSLINE_AMBIGUOUS_WIDE=1 remains the escape hatch for
// terminal+font combos that render Ambiguous wide.
var reviewedGlyphs = map[rune]string{
	// East Asian Ambiguous — computed 1 by default; WT probe: rendered 1.
	'·': "U+00B7 middle dot, quota window separator",
	'±': "U+00B1 plus-minus, token delta",
	'—': "U+2014 em dash, text separator",
	'↑': "U+2191 up arrow, git commits ahead",
	'↓': "U+2193 down arrow, git commits behind",
	'→': "U+2192 right arrow, transition marker",
	// Neutral symbols — computed 1 on every locale; WT probe (↻): rendered 1.
	'↻': "U+21BB open-circle arrow, quota reset countdown",
	'✓': "U+2713 check mark, todo/test status",
	'✖': "U+2716 heavy cross, failure status",
	// Base characters that ship with VS16 (U+FE0F): the pair renders as a
	// 2-cell color emoji; displayWidth promotes base+VS16 to width 2.
	'⏱': "U+23F1 stopwatch base of ⏱️",
	'🗂': "U+1F5C2 card-index base of 🗂️, folder icon",
	// Zero-width machinery.
	'️': "VS16 emoji-presentation selector, width 0 on its own",
}

// widthSelfEvident reports whether every terminal and go-runewidth agree on
// r's width without a registry entry: East Asian Wide/Fullwidth runes are 2
// cells everywhere (checked under a neutral non-CJK condition so the host
// locale cannot skew the answer), and Block Elements are pinned by the
// renderer's narrow flag.
func widthSelfEvident(r rune) bool {
	if isBlockElement(r) {
		return true
	}
	neutral := &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}
	return neutral.RuneWidth(r) == 2
}

// TestProductionGlyphsAreRegistered walks every production (non-test) .go
// file under internal/ and cmd/, extracts string and rune literals, and
// requires each non-ASCII rune to be either width-self-evident or explicitly
// registered in reviewedGlyphs. A new decorative glyph whose width depends on
// the terminal therefore fails CI until its rendered width has been measured
// and the rune added to the registry — turning a class of "user spots a
// misaligned column at runtime" bugs into build-time failures.
//
// Reading the repository's own source is deterministic (the files are fixed
// at build time), so the test stays repeatable despite touching the
// filesystem.
func TestProductionGlyphsAreRegistered(t *testing.T) {
	// Arrange: locate the repo root from this file's location.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate repository root")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	// Act: collect every non-ASCII rune from production string/rune literals.
	type offender struct {
		pos  string
		lit  string
		rune rune
	}
	var offenders []offender
	fset := token.NewFileSet()
	for _, dir := range []string{"internal", "cmd"} {
		root := filepath.Join(repoRoot, dir)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, isLit := n.(*ast.BasicLit)
				if !isLit || (lit.Kind != token.STRING && lit.Kind != token.CHAR) {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true // malformed literal; the compiler rejects it anyway
				}
				for _, r := range value {
					if r < 0x80 || widthSelfEvident(r) {
						continue
					}
					if _, registered := reviewedGlyphs[r]; registered {
						continue
					}
					offenders = append(offenders, offender{
						pos:  fset.Position(lit.Pos()).String(),
						lit:  lit.Value,
						rune: r,
					})
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}

	// Assert: every terminal-dependent rune has been reviewed.
	for _, o := range offenders {
		eawOff := &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}
		eawOn := &runewidth.Condition{EastAsianWidth: true, StrictEmojiNeutral: true}
		t.Errorf(
			"unregistered glyph %q (%U) in literal %s at %s — its rendered width is terminal-dependent "+
				"(go-runewidth: %d narrow / %d CJK). Measure it in the target terminals "+
				"(glyph ×10 + \"|\" ruler probe), then register it in reviewedGlyphs.",
			string(o.rune), o.rune, o.lit, o.pos, eawOff.RuneWidth(o.rune), eawOn.RuneWidth(o.rune),
		)
	}
}
