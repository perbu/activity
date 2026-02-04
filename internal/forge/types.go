package forge

import "time"

// ReviewState represents the state of a code review
type ReviewState string

const (
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
	ReviewDismissed        ReviewState = "dismissed"
	ReviewPending          ReviewState = "pending"
)

// PullRequest represents a pull/merge request from a forge
type PullRequest struct {
	Number   int
	Title    string
	Author   string
	MergedAt time.Time
	MergedBy string
	Commits  []string // SHAs for correlation with git commits
	URL      string
}

// Review represents a code review on a pull request
type Review struct {
	PRNumber  int
	Author    string
	State     ReviewState
	Body      string
	CreatedAt time.Time
}

// PRWithReviews combines a pull request with its reviews
type PRWithReviews struct {
	PR      PullRequest
	Reviews []Review
}
