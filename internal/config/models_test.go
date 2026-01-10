package config

import (
	"testing"
)

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		name          string
		modelID       string
		wantName      string
		wantContext   int
	}{
		{
			name:        "sonnet 4.5 full ID",
			modelID:     "claude-sonnet-4-5-20250929",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
		{
			name:        "opus 4.5 full ID",
			modelID:     "claude-opus-4-5-20251101",
			wantName:    "Opus 4.5",
			wantContext: 200000,
		},
		{
			name:        "haiku 4.5 full ID",
			modelID:     "claude-haiku-4-5-20250929",
			wantName:    "Haiku 4.5",
			wantContext: 200000,
		},
		{
			name:        "sonnet short ID",
			modelID:     "claude-sonnet-4-5",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
		{
			name:        "unknown model defaults to sonnet",
			modelID:     "unknown-model-123",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
		{
			name:        "empty string defaults to sonnet",
			modelID:     "",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
		{
			name:        "model ID prefix match - sonnet with version suffix",
			modelID:     "claude-sonnet-4-5-20250929-v2",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
		{
			name:        "partial model ID match",
			modelID:     "claude-sonnet",
			wantName:    "Sonnet 4.5",
			wantContext: 200000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModelInfo(tt.modelID)
			if got.Name != tt.wantName {
				t.Errorf("GetModelInfo().Name = %v, want %v", got.Name, tt.wantName)
			}
			if got.ContextWindow != tt.wantContext {
				t.Errorf("GetModelInfo().ContextWindow = %v, want %v", got.ContextWindow, tt.wantContext)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name          string
		modelID       string
		inputTokens   int
		outputTokens  int
		cacheTokens   int
		wantCostRange [2]float64 // min and max expected cost
	}{
		{
			name:          "sonnet simple",
			modelID:       "claude-sonnet-4-5-20250929",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{10.50, 10.50}, // 3.00 + 7.50 = 10.50
		},
		{
			name:          "opus expensive",
			modelID:       "claude-opus-4-5-20251101",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{52.50, 52.50}, // 15.00 + 37.50 = 52.50
		},
		{
			name:          "haiku cheap",
			modelID:       "claude-haiku-4-5-20250929",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{2.80, 2.80}, // 0.80 + 2.00 = 2.80
		},
		{
			name:          "with cache read",
			modelID:       "claude-sonnet-4-5-20250929",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   1000000,
			wantCostRange: [2]float64{10.80, 10.80}, // 3.00 + 7.50 + 0.30 = 10.80
		},
		{
			name:          "zero tokens",
			modelID:       "claude-sonnet-4-5-20250929",
			inputTokens:   0,
			outputTokens:  0,
			cacheTokens:   0,
			wantCostRange: [2]float64{0, 0},
		},
		{
			name:          "unknown model defaults to sonnet",
			modelID:       "unknown-model",
			inputTokens:   1000000,
			outputTokens:  500000,
			cacheTokens:   0,
			wantCostRange: [2]float64{10.50, 10.50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.modelID, tt.inputTokens, tt.outputTokens, tt.cacheTokens)
			if got < tt.wantCostRange[0] || got > tt.wantCostRange[1] {
				t.Errorf("CalculateCost() = %v, want range %v", got, tt.wantCostRange)
			}
		})
	}
}

func TestGetContextWindow(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    int
	}{
		{"sonnet", "claude-sonnet-4-5-20250929", 200000},
		{"opus", "claude-opus-4-5-20251101", 200000},
		{"haiku", "claude-haiku-4-5-20250929", 200000},
		{"unknown", "unknown-model", 200000}, // defaults to sonnet
		{"empty", "", 200000},                // defaults to sonnet
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetContextWindow(tt.modelID); got != tt.want {
				t.Errorf("GetContextWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetModelName(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    string
	}{
		{"sonnet full", "claude-sonnet-4-5-20250929", "Sonnet 4.5"},
		{"sonnet short", "claude-sonnet-4-5", "Sonnet 4.5"},
		{"opus full", "claude-opus-4-5-20251101", "Opus 4.5"},
		{"haiku full", "claude-haiku-4-5-20250929", "Haiku 4.5"},
		{"unknown", "unknown-model", "Sonnet 4.5"}, // defaults to sonnet
		{"empty", "", "Sonnet 4.5"},                // defaults to sonnet
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetModelName(tt.modelID); got != tt.want {
				t.Errorf("GetModelName() = %v, want %v", got, tt.want)
			}
		})
	}
}
