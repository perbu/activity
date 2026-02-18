package forge

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	gh "github.com/google/go-github/v75/github"
)

// GitHub implements the Forge interface for GitHub repositories
type GitHub struct {
	owner  string
	repo   string
	client *gh.Client
}

// NewGitHub creates a new GitHub forge client.
// Pass an *http.Client with ghinstallation transport for authenticated access,
// or nil for unauthenticated (public repos).
func NewGitHub(owner, repo string, httpClient *http.Client) (*GitHub, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	var client *gh.Client
	if httpClient != nil {
		client = gh.NewClient(httpClient)
	} else {
		client = gh.NewClient(nil)
	}

	return &GitHub{
		owner:  owner,
		repo:   repo,
		client: client,
	}, nil
}

// Type returns "github"
func (g *GitHub) Type() string { return "github" }

// ListMergedPRs returns PRs merged in the given time range with their reviews
func (g *GitHub) ListMergedPRs(ctx context.Context, since, until time.Time) ([]PRWithReviews, error) {
	prs, err := g.searchMergedPRs(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("failed to search PRs: %w", err)
	}

	var result []PRWithReviews
	for _, pr := range prs {
		reviews, err := g.getReviews(ctx, pr.Number)
		if err != nil {
			slog.Warn("failed to fetch reviews", "pr", pr.Number, "error", err)
			reviews = nil
		}
		comments, err := g.getComments(ctx, pr.Number)
		if err != nil {
			slog.Warn("failed to fetch comments", "pr", pr.Number, "error", err)
			comments = nil
		}
		result = append(result, PRWithReviews{
			PR:       pr,
			Reviews:  reviews,
			Comments: comments,
		})
	}

	return result, nil
}

// searchMergedPRs uses the GitHub search API to find merged PRs
func (g *GitHub) searchMergedPRs(ctx context.Context, since, until time.Time) ([]PullRequest, error) {
	query := fmt.Sprintf("repo:%s/%s is:pr is:merged merged:%s..%s",
		g.owner, g.repo,
		since.Format("2006-01-02"),
		until.Format("2006-01-02"))

	opts := &gh.SearchOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	searchResult, _, err := g.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("GitHub search failed: %w", err)
	}

	var prs []PullRequest
	for _, item := range searchResult.Issues {
		pr := PullRequest{
			Number: item.GetNumber(),
			Title:  item.GetTitle(),
			Author: item.GetUser().GetLogin(),
			URL:    item.GetHTMLURL(),
		}

		details, err := g.getPRDetails(ctx, item.GetNumber())
		if err == nil {
			pr.MergedAt = details.MergedAt
			pr.MergedBy = details.MergedBy
			pr.Commits = details.Commits
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// prDetails holds detailed PR information
type prDetails struct {
	MergedAt time.Time
	MergedBy string
	Commits  []string
}

// getPRDetails fetches additional details for a PR
func (g *GitHub) getPRDetails(ctx context.Context, number int) (*prDetails, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, g.owner, g.repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details: %w", err)
	}

	details := &prDetails{}
	if pr.MergedAt != nil {
		details.MergedAt = pr.MergedAt.Time
	}
	if pr.MergedBy != nil {
		details.MergedBy = pr.MergedBy.GetLogin()
	}
	if pr.MergeCommitSHA != nil {
		details.Commits = []string{*pr.MergeCommitSHA}
	}

	return details, nil
}

// getReviews fetches reviews for a PR
func (g *GitHub) getReviews(ctx context.Context, number int) ([]Review, error) {
	opts := &gh.ListOptions{PerPage: 100}
	ghReviews, _, err := g.client.PullRequests.ListReviews(ctx, g.owner, g.repo, number, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}

	var reviews []Review
	for _, r := range ghReviews {
		review := Review{
			PRNumber: number,
			Author:   r.GetUser().GetLogin(),
			State:    mapGitHubReviewState(r.GetState()),
			Body:     r.GetBody(),
		}
		if r.SubmittedAt != nil {
			review.CreatedAt = r.SubmittedAt.Time
		}
		reviews = append(reviews, review)
	}

	return reviews, nil
}

// getComments fetches issue comments (discussion) for a PR
func (g *GitHub) getComments(ctx context.Context, number int) ([]Comment, error) {
	opts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	ghComments, _, err := g.client.Issues.ListComments(ctx, g.owner, g.repo, number, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	var comments []Comment
	for _, c := range ghComments {
		comment := Comment{
			Author: c.GetUser().GetLogin(),
			Body:   c.GetBody(),
		}
		if c.CreatedAt != nil {
			comment.CreatedAt = c.CreatedAt.Time
		}
		comments = append(comments, comment)
	}

	return comments, nil
}

// mapGitHubReviewState maps GitHub review states to our ReviewState type
func mapGitHubReviewState(state string) ReviewState {
	switch state {
	case "APPROVED":
		return ReviewApproved
	case "CHANGES_REQUESTED":
		return ReviewChangesRequested
	case "COMMENTED":
		return ReviewCommented
	case "DISMISSED":
		return ReviewDismissed
	case "PENDING":
		return ReviewPending
	default:
		return ReviewCommented
	}
}
