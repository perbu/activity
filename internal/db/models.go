package db

import (
	"database/sql"
	"path/filepath"
	"time"
)

// Repository represents a Git repository being tracked
type Repository struct {
	ID          int64
	Name        string
	URL         string
	Branch      string
	Active      bool
	Private     bool           // Requires GitHub App authentication
	Description sql.NullString // AI-generated description from README
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastRunAt   sql.NullTime
	LastRunSHA  sql.NullString
}

// RepoLocalPath computes the local filesystem path for a repository.
// The path is derived from the data directory and repository name.
func RepoLocalPath(dataDir, repoName string) string {
	return filepath.Join(dataDir, repoName)
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

// Subscriber represents an email subscriber for newsletters
type Subscriber struct {
	ID           int64
	Email        string
	SubscribeAll bool // If true, subscribed to all repos
	CreatedAt    time.Time
}

// Subscription represents a subscriber's subscription to a specific repository
type Subscription struct {
	ID           int64
	SubscriberID int64
	RepoID       int64
	CreatedAt    time.Time
}

// NewsletterSend tracks which activity runs have been sent to which subscribers
type NewsletterSend struct {
	ID                int64
	SubscriberID      int64
	ActivityRunID     int64
	SentAt            time.Time
	SendGridMessageID sql.NullString
}

// WeeklyReport represents a week-indexed analysis summary for a repository
type WeeklyReport struct {
	ID             int64
	RepoID         int64
	Year           int
	Week           int
	WeekStart      time.Time
	WeekEnd        time.Time
	Summary        sql.NullString
	CommitCount    int
	Metadata       sql.NullString // JSON: authors, commit info, etc.
	AgentMode      bool
	ToolUsageStats sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SourceRunID    sql.NullInt64
}

// Admin represents an admin user for web authentication
type Admin struct {
	ID        int64
	Email     string
	CreatedAt time.Time
	CreatedBy sql.NullString // Email of admin who created this admin
}
