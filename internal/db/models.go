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
