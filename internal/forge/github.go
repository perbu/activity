package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"

	"github.com/perbu/activity/internal/github"
)

// GitHub implements the Forge interface for GitHub repositories
type GitHub struct {
	owner         string
	repo          string
	tokenProvider *github.TokenProvider
	limiter       *rate.Limiter
	client        *http.Client
}

// NewGitHub creates a new GitHub forge client
func NewGitHub(owner, repo string, tokenProvider *github.TokenProvider) (*GitHub, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	return &GitHub{
		owner:         owner,
		repo:          repo,
		tokenProvider: tokenProvider,
		// 10 requests per second = 36000/hr, well under 5000/hr limit
		limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 1),
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Type returns "github"
func (g *GitHub) Type() string { return "github" }

// ListMergedPRs returns PRs merged in the given time range with their reviews
func (g *GitHub) ListMergedPRs(ctx context.Context, since, until time.Time) ([]PRWithReviews, error) {
	// Search for merged PRs in the date range
	prs, err := g.searchMergedPRs(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("failed to search PRs: %w", err)
	}

	// Fetch reviews for each PR
	var result []PRWithReviews
	for _, pr := range prs {
		reviews, err := g.getReviews(ctx, pr.Number)
		if err != nil {
			// Log but continue - don't fail the whole operation for one PR
			reviews = nil
		}
		result = append(result, PRWithReviews{
			PR:      pr,
			Reviews: reviews,
		})
	}

	return result, nil
}

// searchMergedPRs uses the GitHub search API to find merged PRs
func (g *GitHub) searchMergedPRs(ctx context.Context, since, until time.Time) ([]PullRequest, error) {
	if err := g.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Build search query: merged PRs in date range
	query := fmt.Sprintf("repo:%s/%s is:pr is:merged merged:%s..%s",
		g.owner, g.repo,
		since.Format("2006-01-02"),
		until.Format("2006-01-02"))

	reqURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=100",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Add auth header if token provider is available
	if g.tokenProvider != nil {
		token, err := g.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	var searchResult struct {
		Items []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			User   struct {
				Login string `json:"login"`
			} `json:"user"`
			HTMLURL     string `json:"html_url"`
			PullRequest struct {
				MergedAt string `json:"merged_at"`
			} `json:"pull_request"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var prs []PullRequest
	for _, item := range searchResult.Items {
		pr := PullRequest{
			Number: item.Number,
			Title:  item.Title,
			Author: item.User.Login,
			URL:    item.HTMLURL,
		}

		// Fetch additional PR details (merged_by, commits)
		details, err := g.getPRDetails(ctx, item.Number)
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
	if err := g.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d",
		g.owner, g.repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if g.tokenProvider != nil {
		token, err := g.tokenProvider.GetToken()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get PR details: %s", resp.Status)
	}

	var pr struct {
		MergedAt string `json:"merged_at"`
		MergedBy *struct {
			Login string `json:"login"`
		} `json:"merged_by"`
		MergeCommitSHA string `json:"merge_commit_sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	details := &prDetails{}
	if pr.MergedAt != "" {
		details.MergedAt, _ = time.Parse(time.RFC3339, pr.MergedAt)
	}
	if pr.MergedBy != nil {
		details.MergedBy = pr.MergedBy.Login
	}
	if pr.MergeCommitSHA != "" {
		details.Commits = []string{pr.MergeCommitSHA}
	}

	return details, nil
}

// getReviews fetches reviews for a PR
func (g *GitHub) getReviews(ctx context.Context, number int) ([]Review, error) {
	if err := g.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews",
		g.owner, g.repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if g.tokenProvider != nil {
		token, err := g.tokenProvider.GetToken()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get reviews: %s", resp.Status)
	}

	var ghReviews []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State       string `json:"state"`
		Body        string `json:"body"`
		SubmittedAt string `json:"submitted_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghReviews); err != nil {
		return nil, err
	}

	var reviews []Review
	for _, r := range ghReviews {
		review := Review{
			PRNumber: number,
			Author:   r.User.Login,
			State:    mapGitHubReviewState(r.State),
			Body:     r.Body,
		}
		if r.SubmittedAt != "" {
			review.CreatedAt, _ = time.Parse(time.RFC3339, r.SubmittedAt)
		}
		reviews = append(reviews, review)
	}

	return reviews, nil
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
