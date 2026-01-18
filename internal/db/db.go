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
		{
			version: 3,
			sql: `
				-- Newsletter feature: subscribers and subscriptions
				CREATE TABLE subscribers (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					email TEXT UNIQUE NOT NULL,
					subscribe_all BOOLEAN NOT NULL DEFAULT 0,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TABLE subscriptions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					subscriber_id INTEGER NOT NULL,
					repo_id INTEGER NOT NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
					FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
					UNIQUE(subscriber_id, repo_id)
				);

				CREATE TABLE newsletter_sends (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					subscriber_id INTEGER NOT NULL,
					activity_run_id INTEGER NOT NULL,
					sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					sendgrid_message_id TEXT,
					FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
					FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE CASCADE,
					UNIQUE(subscriber_id, activity_run_id)
				);

				CREATE INDEX idx_subscriptions_subscriber_id ON subscriptions(subscriber_id);
				CREATE INDEX idx_subscriptions_repo_id ON subscriptions(repo_id);
				CREATE INDEX idx_newsletter_sends_subscriber_id ON newsletter_sends(subscriber_id);
				CREATE INDEX idx_newsletter_sends_activity_run_id ON newsletter_sends(activity_run_id);
			`,
		},
		{
			version: 4,
			sql: `
				-- Weekly reports feature: week-indexed analysis storage
				CREATE TABLE weekly_reports (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					repo_id INTEGER NOT NULL,
					year INTEGER NOT NULL,
					week INTEGER NOT NULL,
					week_start DATE NOT NULL,
					week_end DATE NOT NULL,
					summary TEXT,
					commit_count INTEGER NOT NULL DEFAULT 0,
					metadata TEXT,
					agent_mode BOOLEAN DEFAULT 0,
					tool_usage_stats TEXT,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					source_run_id INTEGER,
					FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
					FOREIGN KEY (source_run_id) REFERENCES activity_runs(id) ON DELETE SET NULL,
					UNIQUE(repo_id, year, week)
				);

				CREATE INDEX idx_weekly_reports_repo_id ON weekly_reports(repo_id);
				CREATE INDEX idx_weekly_reports_year_week ON weekly_reports(year, week);
			`,
		},
		{
			version: 5,
			sql: `
				-- Private repository support: flag for GitHub App authentication
				ALTER TABLE repositories ADD COLUMN private BOOLEAN NOT NULL DEFAULT 0;
			`,
		},
		{
			version: 6,
			sql: `
				-- Repository description: AI-generated summary from README
				ALTER TABLE repositories ADD COLUMN description TEXT;
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
