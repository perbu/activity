package git

import "time"

// Commit represents a Git commit.
type Commit struct {
	SHA     string
	Author  string
	Date    time.Time
	Message string
}

// DiffResult contains diff content and metadata about filtering.
type DiffResult struct {
	Diff            string
	SuppressedLines int
}

// AuthorStats contains statistics about an author's contributions.
type AuthorStats struct {
	Name         string
	TotalCommits int
	FirstCommit  time.Time
	LastCommit   time.Time
}

// BranchActivity represents activity on a single branch.
type BranchActivity struct {
	BranchName   string
	CommitCount  int
	Authors      []string
	AuthorCounts map[string]int
}
