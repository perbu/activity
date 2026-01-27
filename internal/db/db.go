package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/perbu/activity/internal/db/migrations"
	"github.com/pressly/goose/v3"
	_ "github.com/lib/pq"
)

// DB wraps a database connection
type DB struct {
	*sql.DB
}

// OpenConfig contains database connection configuration
type OpenConfig struct {
	DSN                    string
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
}

// Open opens a PostgreSQL database connection and runs migrations
func Open(cfg OpenConfig) (*DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	sqlDB, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run goose migrations
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{DB: sqlDB}, nil
}
