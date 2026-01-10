package store

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	// Create a temporary database file
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Open the database
	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify tables were created
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableName)
	if err != nil {
		t.Errorf("Sessions table was not created: %v", err)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	// Try to open with an invalid path
	_, err := Open("/invalid/path/that/cannot/be/created/test.db")
	if err == nil {
		t.Error("Expected error when opening invalid path, got nil")
	}
}

func TestSaveRecord(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	record := SessionRecord{
		ID:           "test-session-1",
		Timestamp:    time.Now(),
		Model:        "claude-sonnet-4-5",
		InputTokens:  1000,
		OutputTokens: 500,
		CacheTokens:  200,
		TotalTokens:  1500,
		Cost:         0.05,
		Project:      "test-project",
	}

	err = db.SaveRecord(record)
	if err != nil {
		t.Fatalf("Failed to save record: %v", err)
	}

	// Verify the record was saved
	saved, err := db.GetSession("test-session-1")
	if err != nil {
		t.Fatalf("Failed to get saved record: %v", err)
	}
	if saved == nil {
		t.Fatal("Saved record not found")
	}
	if saved.ID != record.ID {
		t.Errorf("Expected ID %s, got %s", record.ID, saved.ID)
	}
	if saved.Model != record.Model {
		t.Errorf("Expected Model %s, got %s", record.Model, saved.Model)
	}
	if saved.TotalTokens != record.TotalTokens {
		t.Errorf("Expected TotalTokens %d, got %d", record.TotalTokens, saved.TotalTokens)
	}
}

func TestUpdateSessionTokens(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	sessionID := "test-session-update"

	// First update
	err = db.UpdateSessionTokens(sessionID, 1000, 500, 200, 0.05)
	if err != nil {
		t.Fatalf("Failed to update session tokens: %v", err)
	}

	// Verify
	saved, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get saved record: %v", err)
	}
	if saved == nil {
		t.Fatal("Saved record not found")
	}
	if saved.InputTokens != 1000 {
		t.Errorf("Expected InputTokens 1000, got %d", saved.InputTokens)
	}

	// Second update (should add to existing)
	err = db.UpdateSessionTokens(sessionID, 500, 250, 100, 0.025)
	if err != nil {
		t.Fatalf("Failed to update session tokens again: %v", err)
	}

	// Verify accumulation
	saved, err = db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get saved record: %v", err)
	}
	if saved.InputTokens != 1500 {
		t.Errorf("Expected InputTokens 1500, got %d", saved.InputTokens)
	}
	if saved.OutputTokens != 750 {
		t.Errorf("Expected OutputTokens 750, got %d", saved.OutputTokens)
	}
}

func TestGetRecentHistory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Insert test records
	now := time.Now()
	records := []SessionRecord{
		{ID: "session-1", Timestamp: now.Add(-2 * time.Hour), Model: "sonnet", TotalTokens: 1000, Cost: 0.01},
		{ID: "session-2", Timestamp: now.Add(-1 * time.Hour), Model: "opus", TotalTokens: 2000, Cost: 0.05},
		{ID: "session-3", Timestamp: now, Model: "haiku", TotalTokens: 500, Cost: 0.005},
	}

	for _, r := range records {
		if err := db.SaveRecord(r); err != nil {
			t.Fatalf("Failed to save record: %v", err)
		}
	}

	// Get history
	history, err := db.GetRecentHistory(10)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("Expected 3 records, got %d", len(history))
	}

	// Verify order (most recent first)
	if history[0].ID != "session-3" {
		t.Errorf("Expected first record to be session-3, got %s", history[0].ID)
	}
	if history[2].ID != "session-1" {
		t.Errorf("Expected last record to be session-1, got %s", history[2].ID)
	}
}

func TestGetRecentHistoryLimit(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Insert 5 records
	now := time.Now()
	for i := 1; i <= 5; i++ {
		r := SessionRecord{
			ID:        fmt.Sprintf("session-%d", i),
			Timestamp: now.Add(time.Duration(-i) * time.Hour),
			Model:     "sonnet",
			Cost:      0.01,
		}
		if err := db.SaveRecord(r); err != nil {
			t.Fatalf("Failed to save record: %v", err)
		}
	}

	// Get limited history
	history, err := db.GetRecentHistory(3)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("Expected 3 records, got %d", len(history))
	}
}

func TestGetSessionNotFound(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Try to get non-existent session
	saved, err := db.GetSession("non-existent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if saved != nil {
		t.Error("Expected nil for non-existent session, got record")
	}
}

func TestDeleteSession(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Insert a record
	record := SessionRecord{
		ID:        "to-delete",
		Timestamp: time.Now(),
		Model:     "sonnet",
		Cost:      0.01,
	}
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to save record: %v", err)
	}

	// Verify it exists
	saved, _ := db.GetSession("to-delete")
	if saved == nil {
		t.Fatal("Record should exist before deletion")
	}

	// Delete it
	err = db.DeleteSession("to-delete")
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify it's gone
	saved, _ = db.GetSession("to-delete")
	if saved != nil {
		t.Error("Record should be nil after deletion")
	}
}

func TestSaveRecordUpdate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Save initial record
	record := SessionRecord{
		ID:           "update-test",
		Timestamp:    time.Now(),
		Model:        "sonnet",
		InputTokens:  1000,
		OutputTokens: 500,
		CacheTokens:  0,
		TotalTokens:  1500,
		Cost:         0.01,
		Project:      "old-project",
	}
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to save record: %v", err)
	}

	// Update with new values
	record.Model = "opus"
	record.InputTokens = 2000
	record.OutputTokens = 1000
	record.TotalTokens = 3000
	record.Cost = 0.05
	record.Project = "new-project"
	if err := db.SaveRecord(record); err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	// Verify update
	saved, err := db.GetSession("update-test")
	if err != nil {
		t.Fatalf("Failed to get updated record: %v", err)
	}
	if saved.Model != "opus" {
		t.Errorf("Expected Model 'opus', got '%s'", saved.Model)
	}
	if saved.TotalTokens != 3000 {
		t.Errorf("Expected TotalTokens 3000, got %d", saved.TotalTokens)
	}
	if saved.Project != "new-project" {
		t.Errorf("Expected Project 'new-project', got '%s'", saved.Project)
	}
}

func TestGetRecentHistoryEmpty(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get history from empty database
	history, err := db.GetRecentHistory(10)
	if err != nil {
		t.Fatalf("Failed to get history from empty database: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d records", len(history))
	}
}

func TestUpdateSessionTokensNewSession(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Update a new session (doesn't exist yet)
	sessionID := "new-session"
	err = db.UpdateSessionTokens(sessionID, 1000, 500, 200, 0.05)
	if err != nil {
		t.Fatalf("Failed to update new session: %v", err)
	}

	// Verify it was created
	saved, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get new session: %v", err)
	}
	if saved == nil {
		t.Fatal("New session should exist")
	}
	if saved.InputTokens != 1000 {
		t.Errorf("Expected InputTokens 1000, got %d", saved.InputTokens)
	}
}

func TestClose(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Close the database
	err = db.Close()
	if err != nil {
		t.Errorf("Failed to close database: %v", err)
	}

	// Try to use after close - should fail
	err = db.SaveRecord(SessionRecord{ID: "test", Timestamp: time.Now()})
	if err == nil {
		t.Error("Expected error when using closed database")
	}
}
