package layout

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRender_WindowsTerminalProbeRegression pins the column alignment of the
// exact three-row statusline that was measured misaligned on Windows Terminal
// on 2026-07-19, using the width model the deployed binary configures for WT
// (Ambiguous narrow + narrow Block Elements). The expected cell widths equal
// the widths a user-run WT probe measured for the real rendering (↑ ↻ · = 1
// cell, base+VS16 = 2 cells, █░ = 1 cell), so computed == rendered and every
// row pads to the same column. The previously shipped AmbigWide=true default
// counted ↑ and · as 2, drifting their rows one cell left each — this test
// fails if that regression ever returns.
func TestRender_WindowsTerminalProbeRegression(t *testing.T) {
	// Arrange: replicate main.go's runtime configuration for Windows Terminal
	// (AmbigWide=false), overriding whatever the host locale preset at init —
	// on CJK Windows go-runewidth boots with EastAsianWidth=true from CP936.
	origEAW := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = false
	runewidth.DefaultCondition.CreateLUT()
	t.Cleanup(func() {
		runewidth.DefaultCondition.EastAsianWidth = origEAW
		runewidth.DefaultCondition.CreateLUT()
	})

	grid := &Grid{Rows: []GridRow{
		{Cells: []string{"🗂️ claude-token-monitor", "[Fable 5 [░░░░░░░░░░] 0/1000K (0.0%)] 💭 xhigh", "v2.1.214"}},
		{Cells: []string{"🌿 main +2 🔄 ↑1", "📦 3 CLAUDE.md + 2 rules"}},
		{Cells: []string{"🕐 2026-07-19 03:52", "📊 [Pro] 0% 5h ↻ 4h57m · 37% 7d ↻ 2d6h", "💾 262.7 MB"}},
	}}
	r := NewRenderer(grid, true) // narrow=true: Windows terminals render █░ at 1 cell

	// Act
	lines := r.Render()

	// Assert: cell widths match the WT-rendered widths from the probe...
	assert.Equal(t, 23, r.displayWidth("🗂️ claude-token-monitor"), "🗂️+VS16 renders 2 cells")
	assert.Equal(t, 16, r.displayWidth("🌿 main +2 🔄 ↑1"), "↑ renders 1 cell on WT")
	assert.Equal(t, 19, r.displayWidth("🕐 2026-07-19 03:52"))
	assert.Equal(t, 46, r.displayWidth("[Fable 5 [░░░░░░░░░░] 0/1000K (0.0%)] 💭 xhigh"), "░ renders 1 cell on WT")
	assert.Equal(t, 38, r.displayWidth("📊 [Pro] 0% 5h ↻ 4h57m · 37% 7d ↻ 2d6h"), "↻ and · render 1 cell on WT")

	// ...so every row pads its first column to 23 and its second to 46.
	require.Len(t, lines, 3)
	assert.Equal(t, "🗂️ claude-token-monitor | [Fable 5 [░░░░░░░░░░] 0/1000K (0.0%)] 💭 xhigh | v2.1.214", lines[0])
	assert.Equal(t, "🌿 main +2 🔄 ↑1        | 📦 3 CLAUDE.md + 2 rules", lines[1])
	assert.Equal(t, "🕐 2026-07-19 03:52     | 📊 [Pro] 0% 5h ↻ 4h57m · 37% 7d ↻ 2d6h         | 💾 262.7 MB", lines[2])
}
