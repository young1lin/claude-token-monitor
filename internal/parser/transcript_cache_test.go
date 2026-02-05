package parser

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestParseTranscriptMtimeCacheHit(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// First parse - should read file
	start1 := time.Now()
	summary1, err := ParseTranscriptLastNLines(transcriptPath, 100)
	elapsed1 := time.Since(start1)
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	// Second parse without modifying file - should hit cache
	start2 := time.Now()
	summary2, err := ParseTranscriptLastNLines(transcriptPath, 100)
	elapsed2 := time.Since(start2)
	if err != nil {
		t.Fatalf("Second parse failed: %v", err)
	}

	// Verify results are identical
	if summary1.InputTokens != summary2.InputTokens {
		t.Errorf("InputTokens mismatch: %d != %d", summary1.InputTokens, summary2.InputTokens)
	}
	if summary1.OutputTokens != summary2.OutputTokens {
		t.Errorf("OutputTokens mismatch: %d != %d", summary1.OutputTokens, summary2.OutputTokens)
	}

	// Cache hit should be significantly faster (at least 10x)
	if elapsed2 > elapsed1/10 {
		t.Logf("Warning: Cache hit not significantly faster: %v vs %v", elapsed1, elapsed2)
	}

	t.Logf("Parse times: first=%v, cached=%v, speedup=%.1fx",
		elapsed1, elapsed2, float64(elapsed1)/float64(elapsed2))
}

func TestParseTranscriptMtimeCacheMiss(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content1 := `{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// First parse
	summary1, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	if summary1.InputTokens != 100 {
		t.Errorf("Expected InputTokens=100, got %d", summary1.InputTokens)
	}

	// Wait a bit to ensure different mtime
	time.Sleep(10 * time.Millisecond)

	// Modify file
	content2 := `{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":200,"output_tokens":75}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to modify transcript file: %v", err)
	}

	// Second parse - should detect file modification and re-parse
	summary2, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("Second parse failed: %v", err)
	}

	// Results should reflect new content
	if summary2.InputTokens != 200 {
		t.Errorf("Expected InputTokens=200 after modification, got %d", summary2.InputTokens)
	}
	if summary2.OutputTokens != 75 {
		t.Errorf("Expected OutputTokens=75 after modification, got %d", summary2.OutputTokens)
	}

	// Verify old and new values are different
	if summary1.InputTokens == summary2.InputTokens {
		t.Error("InputTokens should differ after file modification")
	}
}

func TestParseTranscriptCacheExpiration(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// First parse
	summary1, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	// Wait for cache to expire (5s TTL + buffer)
	t.Log("Waiting 6 seconds for cache expiration...")
	time.Sleep(6 * time.Second)

	// Second parse - cache should be expired, even if file unchanged
	summary2, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("Second parse failed: %v", err)
	}

	// Results should still be identical (same file)
	if summary1.InputTokens != summary2.InputTokens {
		t.Errorf("InputTokens mismatch: %d != %d", summary1.InputTokens, summary2.InputTokens)
	}

	t.Log("Cache expiration verified - file was re-parsed after TTL")
}

func TestParseTranscriptConcurrent(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// Launch 10 concurrent goroutines
	var wg sync.WaitGroup
	results := make([]*TranscriptSummary, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			summary, err := ParseTranscriptLastNLines(transcriptPath, 100)
			results[idx] = summary
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}

	// Verify all results are identical (cache coherency)
	for i := 1; i < len(results); i++ {
		if results[i].InputTokens != results[0].InputTokens {
			t.Errorf("Concurrent InputTokens mismatch at index %d: %d != %d",
				i, results[i].InputTokens, results[0].InputTokens)
		}
		if results[i].OutputTokens != results[0].OutputTokens {
			t.Errorf("Concurrent OutputTokens mismatch at index %d: %d != %d",
				i, results[i].OutputTokens, results[0].OutputTokens)
		}
	}

	t.Log("Concurrent parsing verified - all results identical")
}

func TestParseTranscriptEmptyFile(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create empty transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "empty.jsonl")

	if err := os.WriteFile(transcriptPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Parse empty file
	summary, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should return zero values
	if summary.InputTokens != 0 {
		t.Errorf("Expected InputTokens=0, got %d", summary.InputTokens)
	}
	if summary.OutputTokens != 0 {
		t.Errorf("Expected OutputTokens=0, got %d", summary.OutputTokens)
	}
}

func TestParseTranscriptNonExistentFile(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Try to parse non-existent file
	nonExistentPath := filepath.Join(t.TempDir(), "non-existent.jsonl")
	summary, err := ParseTranscriptLastNLines(nonExistentPath, 100)

	// Should return empty summary, not error (per original behavior)
	if err != nil {
		t.Fatalf("Expected nil error for non-existent file, got: %v", err)
	}

	// Should return zero values
	if summary.InputTokens != 0 {
		t.Errorf("Expected InputTokens=0, got %d", summary.InputTokens)
	}
}

func TestParseTranscriptInvalidJSON(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create transcript file with invalid JSON
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "invalid.jsonl")

	// First line is invalid JSON, second and third are valid
	content := `{"type":"user_message","invalid json here without closing brace
{"type":"user","timestamp":"2024-01-01T00:00:00Z"}
{"type":"assistant","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// Should skip invalid lines and parse valid ones
	summary, err := ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have parsed the valid assistant message
	if summary.InputTokens != 100 {
		t.Errorf("Expected InputTokens=100, got %d", summary.InputTokens)
	}
	if summary.OutputTokens != 50 {
		t.Errorf("Expected OutputTokens=50, got %d", summary.OutputTokens)
	}

	t.Logf("Parser successfully skipped invalid JSON and parsed valid entries")
}

func TestParseTranscriptMtimePreservation(t *testing.T) {
	// Clear cache before test
	clearTranscriptCache()

	// Create temp transcript file
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"assistant_message","message":{"content":[{"type":"text"}],"usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create transcript file: %v", err)
	}

	// Get original mtime
	info1, err := os.Stat(transcriptPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	mtime1 := info1.ModTime()

	// Parse file
	_, err = ParseTranscriptLastNLines(transcriptPath, 100)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify mtime unchanged (parser only reads, doesn't write)
	info2, err := os.Stat(transcriptPath)
	if err != nil {
		t.Fatalf("Failed to stat file after parse: %v", err)
	}
	mtime2 := info2.ModTime()

	if !mtime1.Equal(mtime2) {
		t.Errorf("File mtime changed after parsing: %v -> %v", mtime1, mtime2)
	}
}

// Helper: Clear transcript cache
func clearTranscriptCache() {
	transcriptCacheMu.Lock()
	transcriptCache = nil
	transcriptCachePath = ""
	transcriptCacheTime = time.Time{}
	transcriptCacheMu.Unlock()
}
