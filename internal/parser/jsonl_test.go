package parser

import (
	"testing"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name    string
		line    []byte
		wantErr bool
		wantNil bool
	}{
		{
			name: "valid assistant message",
			line: []byte(`{"type":"assistant","uuid":"test","timestamp":"2024-01-01T00:00:00Z","sessionId":"session123","message":{"model":"claude-sonnet-4-5-20250929","id":"msg123","type":"message","role":"assistant","stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":0,"cache_read_input_tokens":200}}}`),
			wantErr: false,
			wantNil: false,
		},
		{
			name: "user message (should return nil)",
			line: []byte(`{"type":"user","uuid":"test","timestamp":"2024-01-01T00:00:00Z","sessionId":"session123"}`),
			wantErr: false,
			wantNil: true,
		},
		{
			name:    "invalid JSON",
			line:    []byte(`{invalid json}`),
			wantErr: true,
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    []byte(``),
			wantErr: true,
			wantNil: true,
		},
		{
			name: "system message (should return nil)",
			line: []byte(`{"type":"system","uuid":"test"}`),
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantNil {
				t.Errorf("ParseLine() got = %v, wantNil %v", got, tt.wantNil)
			}
			if !tt.wantNil && got != nil {
				if got.Type != "assistant" {
					t.Errorf("ParseLine() Type = %v, want assistant", got.Type)
				}
			}
		})
	}
}

func TestIsAssistantMessage(t *testing.T) {
	tests := []struct {
		name string
		line []byte
		want bool
	}{
		{
			name: "assistant message",
			line: []byte(`{"type":"assistant","uuid":"test"}`),
			want: true,
		},
		{
			name: "user message",
			line: []byte(`{"type":"user","uuid":"test"}`),
			want: false,
		},
		{
			name: "invalid JSON",
			line: []byte(`{invalid}`),
			want: false,
		},
		{
			name: "empty line",
			line: []byte(``),
			want: false,
		},
		{
			name: "line with assistant text but not JSON",
			line: []byte(`this is assistant text`),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAssistantMessage(tt.line); got != tt.want {
				t.Errorf("IsAssistantMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		subslice []byte
		want     bool
	}{
		{"contains", []byte("hello world"), []byte("world"), true},
		{"not contains", []byte("hello world"), []byte("goodbye"), false},
		{"empty subslice", []byte("hello"), []byte(""), true},
		{"empty data", []byte(""), []byte("hello"), false},
		{"both empty", []byte(""), []byte(""), true},
		{"case sensitive", []byte("Hello"), []byte("hello"), false},
		{"single char match", []byte("abc"), []byte("b"), true},
		{"single char no match", []byte("abc"), []byte("d"), false},
		{"subslice longer than data", []byte("ab"), []byte("abc"), false},
		{"subslice same as data", []byte("abc"), []byte("abc"), true},
		{"partial match at start", []byte("abcdef"), []byte("abc"), true},
		{"partial match at end", []byte("abcdef"), []byte("def"), true},
		{"partial match in middle", []byte("abcdef"), []byte("cde"), true},
		{"special characters", []byte("hello\nworld"), []byte("\n"), true},
		{"json object", []byte(`{"type":"assistant"}`), []byte(`type`), true},
		{"unicode characters", []byte("hello世界"), []byte("世界"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.data, tt.subslice); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLineWithInvalidAssistantMessage(t *testing.T) {
	// Test case where type is "assistant" but the message is malformed
	line := []byte(`{"type":"assistant","uuid":"test","timestamp":"2024-01-01T00:00:00Z","sessionId":"session123","message":{"model":"claude-sonnet-4-5-20250929","id":"msg123","type":"message","role":"assistant","stop_reason":"end_turn","usage":{"input_tokens":invalid}}}`)

	got, err := ParseLine(line)
	if err == nil {
		t.Error("Expected error for malformed assistant message, got nil")
	}
	if got != nil {
		t.Error("Expected nil result for malformed assistant message")
	}
}
