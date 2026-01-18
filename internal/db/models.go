package db

import (
	"database/sql"
	"time"
)

// Repository represents a Git repository being tracked
type Repository struct {
	ID         int64
	Name       string
	URL        string
	Branch     string
	LocalPath  string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastRunAt  sql.NullTime
	LastRunSHA sql.NullString
}

// ActivityRun represents a single analysis run on a repository
type ActivityRun struct {
	ID          int64
	RepoID      int64
	StartSHA    string
	EndSHA      string
	StartedAt   time.Time
	CompletedAt sql.NullTime
	Summary     sql.NullString
	RawData     sql.NullString // JSON

	// Phase 3: Agent-based analysis fields
	AgentMode      bool           // Whether agent-based analysis was used
	ToolUsageStats sql.NullString // JSON: cost tracker metadata
}
