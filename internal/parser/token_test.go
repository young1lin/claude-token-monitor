package parser

import (
	"testing"
)

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name   string
		msg    *AssistantMessage
		want   TokenStats
	}{
		{
			name: "basic stats",
			msg: &AssistantMessage{
				Message: struct {
					Model      string `json:"model"`
					ID         string `json:"id"`
					Type       string `json:"type"`
					Role       string `json:"role"`
					StopReason string `json:"stop_reason"`
					Usage      struct {
						InputTokens              int `json:"input_tokens"`
						OutputTokens             int `json:"output_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					} `json:"usage"`
				}{
					Usage: struct {
						InputTokens              int `json:"input_tokens"`
						OutputTokens             int `json:"output_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					}{
						InputTokens:          1000,
						OutputTokens:         500,
						CacheReadInputTokens: 200,
					},
				},
			},
			want: TokenStats{
				InputTokens:  1000,
				OutputTokens: 500,
				CacheTokens:  200,
				TotalTokens:  1500,
			},
		},
		{
			name: "zero tokens",
			msg: &AssistantMessage{
				Message: struct {
					Model      string `json:"model"`
					ID         string `json:"id"`
					Type       string `json:"type"`
					Role       string `json:"role"`
					StopReason string `json:"stop_reason"`
					Usage      struct {
						InputTokens              int `json:"input_tokens"`
						OutputTokens             int `json:"output_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					} `json:"usage"`
				}{
					Usage: struct {
						InputTokens              int `json:"input_tokens"`
						OutputTokens             int `json:"output_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					}{
						InputTokens:          0,
						OutputTokens:         0,
						CacheReadInputTokens: 0,
					},
				},
			},
			want: TokenStats{
				InputTokens:  0,
				OutputTokens: 0,
				CacheTokens:  0,
				TotalTokens:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateStats(tt.msg)
			if got.InputTokens != tt.want.InputTokens {
				t.Errorf("InputTokens = %v, want %v", got.InputTokens, tt.want.InputTokens)
			}
			if got.OutputTokens != tt.want.OutputTokens {
				t.Errorf("OutputTokens = %v, want %v", got.OutputTokens, tt.want.OutputTokens)
			}
			if got.CacheTokens != tt.want.CacheTokens {
				t.Errorf("CacheTokens = %v, want %v", got.CacheTokens, tt.want.CacheTokens)
			}
			if got.TotalTokens != tt.want.TotalTokens {
				t.Errorf("TotalTokens = %v, want %v", got.TotalTokens, tt.want.TotalTokens)
			}
		})
	}
}

func TestCalculateContextPercentage(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		totalTokens int
		want        float64
	}{
		{
			name:        "sonnet 50% usage",
			model:       "claude-sonnet-4-5-20250929",
			totalTokens: 100000,
			want:        50.0,
		},
		{
			name:        "unknown model",
			model:       "unknown-model",
			totalTokens: 100000,
			want:        50.0, // defaults to sonnet
		},
		{
			name:        "zero tokens",
			model:       "claude-sonnet-4-5-20250929",
			totalTokens: 0,
			want:        0,
		},
		{
			name:        "empty model",
			model:       "",
			totalTokens: 100000,
			want:        50.0, // defaults to sonnet
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateContextPercentage(tt.model, tt.totalTokens)
			if got != tt.want {
				t.Errorf("CalculateContextPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"small count", 999, "999"},
		{"thousands", 1234, "1.2K"},
		{"exact thousand", 1000, "1.0K"},
		{"millions", 1234567, "1.2M"},
		{"exact million", 1000000, "1.0M"},
		{"zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatTokens(tt.count); got != tt.want {
				t.Errorf("FormatTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		name string
		cost float64
		want string
	}{
		{"small cost", 0.0012, "$0.0012"},
		{"dollar cost", 1.50, "$1.50"},
		{"exact dollar", 1.00, "$1.00"},
		{"zero cost", 0.00, "$0.0000"},
		{"large cost", 100.1234, "$100.12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCost(tt.cost); got != tt.want {
				t.Errorf("FormatCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		inputTokens   int
		outputTokens  int
		cacheTokens   int
		wantCostRange [2]float64 // min and max expected cost
	}{
		{
			name:          "sonnet simple",
			model:         "claude-sonnet-4-5-20250929",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{10.50, 10.50},
		},
		{
			name:          "with cache",
			model:         "claude-sonnet-4-5-20250929",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   1000000,
			wantCostRange: [2]float64{10.80, 10.80},
		},
		{
			name:          "zero tokens",
			model:         "claude-sonnet-4-5-20250929",
			inputTokens:   0,
			outputTokens:  0,
			cacheTokens:   0,
			wantCostRange: [2]float64{0, 0},
		},
		{
			name:          "unknown model",
			model:         "unknown-model",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{10.50, 10.50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.model, tt.inputTokens, tt.outputTokens, tt.cacheTokens)
			if got < tt.wantCostRange[0] || got > tt.wantCostRange[1] {
				t.Errorf("CalculateCost() = %v, want range %v", got, tt.wantCostRange)
			}
		})
	}
}

func TestCalculateContextPercentageZeroContextWindow(t *testing.T) {
	// Mock a context window getter that returns 0
	zeroGetter := func(model string) int {
		return 0
	}

	result := CalculateContextPercentageWithGetter("any-model", 1000, zeroGetter)

	if result != 0 {
		t.Errorf("Expected 0%% for zero context window, got %v", result)
	}
}

func TestCalculateContextPercentageWithCustomGetter(t *testing.T) {
	customGetter := func(model string) int {
		return 100000 // Custom context window
	}

	result := CalculateContextPercentageWithGetter("test-model", 50000, customGetter)

	expected := 50.0
	if result != expected {
		t.Errorf("Expected %v%%, got %v", expected, result)
	}
}

func TestCalculateContextPercentageWithGetterVariousScenarios(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		totalTokens int
		getter      func(string) int
		want        float64
	}{
		{
			name:        "zero context window",
			model:       "any-model",
			totalTokens: 1000,
			getter:      func(string) int { return 0 },
			want:        0,
		},
		{
			name:        "full context",
			model:       "test-model",
			totalTokens: 100000,
			getter:      func(string) int { return 100000 },
			want:        100.0,
		},
		{
			name:        "half context",
			model:       "test-model",
			totalTokens: 50000,
			getter:      func(string) int { return 100000 },
			want:        50.0,
		},
		{
			name:        "over context",
			model:       "test-model",
			totalTokens: 150000,
			getter:      func(string) int { return 100000 },
			want:        150.0,
		},
		{
			name:        "zero tokens",
			model:       "test-model",
			totalTokens: 0,
			getter:      func(string) int { return 100000 },
			want:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateContextPercentageWithGetter(tt.model, tt.totalTokens, tt.getter)
			if got != tt.want {
				t.Errorf("CalculateContextPercentageWithGetter() = %v, want %v", got, tt.want)
			}
		})
	}
}
