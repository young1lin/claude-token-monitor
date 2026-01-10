package store

import (
	"database/sql"
	"time"

	_ "github.com/glebarez/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	*sql.DB
}

// SessionRecord represents a stored session record
type SessionRecord struct {
	ID           string
	Timestamp    time.Time
	Model        string
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	TotalTokens  int
	Cost         float64
	Project      string
}

// Open opens the SQLite database and creates tables if needed
func Open(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, err
	}

	db := &DB{DB: sqlDB}

	// Create tables
	if err := db.createTables(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return db, nil
}

// createTables creates the necessary database tables
func (db *DB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		timestamp INTEGER NOT NULL,
		model TEXT NOT NULL,
		input_tokens INTEGER NOT NULL,
		output_tokens INTEGER NOT NULL,
		cache_tokens INTEGER NOT NULL,
		total_tokens INTEGER NOT NULL,
		cost REAL NOT NULL,
		project TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_timestamp ON sessions(timestamp DESC);
	`

	_, err := db.Exec(query)
	return err
}

// SaveRecord saves or updates a session record
func (db *DB) SaveRecord(record SessionRecord) error {
	query := `
	INSERT INTO sessions (id, timestamp, model, input_tokens, output_tokens, cache_tokens, total_tokens, cost, project)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		timestamp = excluded.timestamp,
		model = excluded.model,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		cache_tokens = excluded.cache_tokens,
		total_tokens = excluded.total_tokens,
		cost = excluded.cost,
		project = excluded.project
	`

	_, err := db.Exec(
		query,
		record.ID,
		record.Timestamp.Unix(),
		record.Model,
		record.InputTokens,
		record.OutputTokens,
		record.CacheTokens,
		record.TotalTokens,
		record.Cost,
		record.Project,
	)

	return err
}

// UpdateSessionTokens updates the token counts for a session
func (db *DB) UpdateSessionTokens(sessionID string, inputTokens, outputTokens, cacheTokens int, cost float64) error {
	query := `
	INSERT INTO sessions (id, timestamp, model, input_tokens, output_tokens, cache_tokens, total_tokens, cost, project)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')
	ON CONFLICT(id) DO UPDATE SET
		input_tokens = input_tokens + excluded.input_tokens,
		output_tokens = output_tokens + excluded.output_tokens,
		cache_tokens = cache_tokens + excluded.cache_tokens,
		total_tokens = total_tokens + excluded.total_tokens,
		cost = cost + excluded.cost,
		timestamp = excluded.timestamp
	`

	totalTokens := inputTokens + outputTokens
	_, err := db.Exec(
		query,
		sessionID,
		time.Now().Unix(),
		"",
		inputTokens,
		outputTokens,
		cacheTokens,
		totalTokens,
		cost,
	)

	return err
}

// GetRecentHistory retrieves recent session records
func (db *DB) GetRecentHistory(limit int) ([]SessionRecord, error) {
	query := `
	SELECT id, timestamp, model, input_tokens, output_tokens, cache_tokens, total_tokens, cost, project
	FROM sessions
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SessionRecord
	for rows.Next() {
		var r SessionRecord
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.Model, &r.InputTokens, &r.OutputTokens, &r.CacheTokens, &r.TotalTokens, &r.Cost, &r.Project); err != nil {
			return nil, err
		}
		r.Timestamp = time.Unix(ts, 0)
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// GetSession retrieves a specific session record
func (db *DB) GetSession(sessionID string) (*SessionRecord, error) {
	query := `
	SELECT id, timestamp, model, input_tokens, output_tokens, cache_tokens, total_tokens, cost, project
	FROM sessions
	WHERE id = ?
	`

	var r SessionRecord
	var ts int64
	err := db.QueryRow(query, sessionID).Scan(
		&r.ID, &ts, &r.Model, &r.InputTokens, &r.OutputTokens,
		&r.CacheTokens, &r.TotalTokens, &r.Cost, &r.Project,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r.Timestamp = time.Unix(ts, 0)
	return &r, nil
}

// DeleteSession deletes a session record
func (db *DB) DeleteSession(sessionID string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
