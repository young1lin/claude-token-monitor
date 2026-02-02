package content

import (
	"testing"
)

func TestFormatMemoryFilesDisplay(t *testing.T) {
	tests := []struct {
		name string
		info MemoryFilesInfo
		want string
	}{
		{
			name: "all fields populated",
			info: MemoryFilesInfo{
				CLAUDEMdCount: 2,
				RulesCount:    3,
				MCPCount:      1,
			},
			want: "📦 2 CLAUDE.md + 3 rules + 1 MCPs",
		},
		{
			name: "single CLAUDE.md only",
			info: MemoryFilesInfo{
				CLAUDEMdCount: 1,
			},
			want: "📦 CLAUDE.md",
		},
		{
			name: "no fields returns empty",
			info: MemoryFilesInfo{},
			want: "",
		},
		{
			name: "rules and MCPs without CLAUDE.md",
			info: MemoryFilesInfo{
				RulesCount: 2,
				MCPCount:   3,
			},
			want: "📦 2 rules + 3 MCPs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatMemoryFilesDisplay(tt.info)

			// Assert
			if got != tt.want {
				t.Errorf("formatMemoryFilesDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}
