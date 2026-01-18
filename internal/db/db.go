package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps a database connection
type DB struct {
	*sql.DB
}

// Open opens a database connection and runs migrations
func Open(dataDir string) (*DB, error) {
	dbPath := filepath.Join(dataDir, "activity.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{DB: sqlDB}

	// Run migrations
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// migrate runs database migrations
func (db *DB) migrate() error {
	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	// Define migrations
	migrations := []struct {
		version int
		sql     string
	}{
		{
			version: 1,
			sql: `
				CREATE TABLE repositories (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT UNIQUE NOT NULL,
					url TEXT NOT NULL,
					branch TEXT NOT NULL DEFAULT 'main',
					local_path TEXT NOT NULL,
					active BOOLEAN NOT NULL DEFAULT 1,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					last_run_at DATETIME,
					last_run_sha TEXT
				);

				CREATE TABLE activity_runs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					repo_id INTEGER NOT NULL,
					start_sha TEXT NOT NULL,
					end_sha TEXT NOT NULL,
					started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					completed_at DATETIME,
					summary TEXT,
					raw_data TEXT,
					FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE
				);

				CREATE INDEX idx_activity_runs_repo_id ON activity_runs(repo_id);
				CREATE INDEX idx_activity_runs_started_at ON activity_runs(started_at);
			`,
		},
		{
			version: 2,
			sql: `
				-- Phase 3: Add agent-based analysis columns
				ALTER TABLE activity_runs ADD COLUMN agent_mode BOOLEAN DEFAULT 0;
				ALTER TABLE activity_runs ADD COLUMN tool_usage_stats TEXT;

				CREATE INDEX idx_activity_runs_agent_mode ON activity_runs(agent_mode);
			`,
		},
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.version > currentVersion {
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
			}

			// Apply migration
			if _, err := tx.Exec(m.sql); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
			}

			// Record migration
			if _, err := tx.Exec("INSERT INTO migrations (version) VALUES (?)", m.version); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to record migration %d: %w", m.version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
			}
		}
	}

	return nil
}
