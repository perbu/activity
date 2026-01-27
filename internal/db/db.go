package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/perbu/activity/internal/db/migrations"
	"github.com/pressly/goose/v3"
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

	// Handle migration from old system to goose
	if err := handleLegacyMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to handle legacy migrations: %w", err)
	}

	// Run goose migrations
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{DB: sqlDB}, nil
}

// handleLegacyMigrations checks if the old migrations table exists with version >= 8.
// If so, it creates the goose_db_version table with version 1 and drops the old table.
func handleLegacyMigrations(db *sql.DB) error {
	// Check if old migrations table exists
	var tableName string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='migrations'
	`).Scan(&tableName)
	if err == sql.ErrNoRows {
		// No old migrations table, nothing to do
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check for migrations table: %w", err)
	}

	// Old table exists, check the version
	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if version < 8 {
		return fmt.Errorf("database is at migration version %d, expected 8 or higher for goose migration", version)
	}

	// Check if goose table already exists (migration already done)
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='goose_db_version'
	`).Scan(&tableName)
	if err == nil {
		// Goose table already exists, just drop old table if it's still there
		_, err = db.Exec("DROP TABLE IF EXISTS migrations")
		return err
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check for goose table: %w", err)
	}

	// Create goose version table and insert version 1 (our consolidated migration)
	_, err = db.Exec(`
		CREATE TABLE goose_db_version (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			is_applied INTEGER NOT NULL,
			tstamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create goose_db_version table: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1)
	`)
	if err != nil {
		return fmt.Errorf("failed to insert goose version: %w", err)
	}

	// Drop the old migrations table
	_, err = db.Exec("DROP TABLE migrations")
	if err != nil {
		return fmt.Errorf("failed to drop old migrations table: %w", err)
	}

	return nil
}
