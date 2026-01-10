package parser

import (
	"encoding/json"
	"fmt"
)

// AssistantMessage represents an assistant message entry in the JSONL file
type AssistantMessage struct {
	Type      string `json:"type"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Message   struct {
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
	} `json:"message"`
}

// ParseLine parses a single JSONL line and returns an AssistantMessage if applicable
func ParseLine(line []byte) (*AssistantMessage, error) {
	var msg struct {
		Type string `json:"type"`
	}

	// First check the type without full unmarshaling
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message type: %w", err)
	}

	// Only care about assistant messages (which contain token usage)
	if msg.Type != "assistant" {
		return nil, nil
	}

	var assistant AssistantMessage
	if err := json.Unmarshal(line, &assistant); err != nil {
		return nil, fmt.Errorf("failed to parse assistant message: %w", err)
	}

	return &assistant, nil
}

// IsAssistantMessage quickly checks if a line is an assistant message
func IsAssistantMessage(line []byte) bool {
	// Quick check for "type":"assistant"
	return jsonContains(line, "type") && jsonContains(line, "assistant")
}

// jsonContains is a simple helper to check if a JSON line contains a key/value pair
// This is faster than full unmarshaling for filtering
func jsonContains(data []byte, value string) bool {
	return json.Valid(data) && contains(data, []byte(value))
}

// contains is a simple bytes.Contains implementation
func contains(data, subslice []byte) bool {
	for i := 0; i <= len(data)-len(subslice); i++ {
		match := true
		for j := 0; j < len(subslice); j++ {
			if data[i+j] != subslice[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
