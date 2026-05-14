package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// Forgejo implements the Forge interface for Forgejo/Gitea instances
type Forgejo struct {
	baseURL string
	owner   string
	repo    string
	token   string // Optional, empty for public repos
	limiter *rate.Limiter
	client  *http.Client
}

// NewForgejo creates a new Forgejo forge client
func NewForgejo(baseURL, owner, repo, token string) (*Forgejo, error) {
	if baseURL == "" || owner == "" || repo == "" {
		return nil, fmt.Errorf("baseURL, owner, and repo are required")
	}

	// Normalize base URL
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Forgejo{
		baseURL: baseURL,
		owner:   owner,
		repo:    repo,
		token:   token,
		// 5 requests per second, conservative default for Forgejo instances
		limiter: rate.NewLimiter(rate.Every(200*time.Millisecond), 1),
		client:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Type returns "forgejo"
func (f *Forgejo) Type() string { return "forgejo" }

// ListMergedPRs returns PRs merged in the given time range with their reviews
func (f *Forgejo) ListMergedPRs(ctx context.Context, since, until time.Time) ([]PRWithReviews, error) {
	// Fetch closed PRs (Forgejo API doesn't have a merged filter)
	prs, err := f.listClosedPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}

	// Filter to merged PRs within date range and fetch reviews
	var result []PRWithReviews
	for _, pr := range prs {
		// Skip if not merged or outside date range
		if pr.MergedAt.IsZero() {
			continue
		}
		if pr.MergedAt.Before(since) || pr.MergedAt.After(until) {
			continue
		}

		// Fetch reviews and comments
		reviews, err := f.getReviews(ctx, pr.Number)
		if err != nil {
			slog.Warn("failed to fetch reviews", "pr", pr.Number, "error", err)
			reviews = nil
		}
		comments, err := f.getComments(ctx, pr.Number)
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

// listClosedPRs fetches all closed PRs (we'll filter for merged ones)
func (f *Forgejo) listClosedPRs(ctx context.Context) ([]PullRequest, error) {
	var allPRs []PullRequest
	page := 1
	perPage := 50

	for {
		if err := f.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		reqURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/pulls?state=closed&page=%d&limit=%d",
			f.baseURL, f.owner, f.repo, page, perPage)

		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, err
		}

		if f.token != "" {
			req.Header.Set("Authorization", "token "+f.token)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := f.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("Forgejo API error: %s - %s", resp.Status, string(body))
		}

		var prs []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			User   struct {
				Login string `json:"login"`
			} `json:"user"`
			MergedAt *string `json:"merged_at"`
			MergedBy *struct {
				Login string `json:"login"`
			} `json:"merged_by"`
			MergeCommitSHA string `json:"merge_commit_sha"`
			HTMLURL        string `json:"html_url"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(prs) == 0 {
			break
		}

		for _, pr := range prs {
			pull := PullRequest{
				Number: pr.Number,
				Title:  pr.Title,
				Author: pr.User.Login,
				URL:    pr.HTMLURL,
			}

			if pr.MergedAt != nil && *pr.MergedAt != "" {
				if t, err := time.Parse(time.RFC3339, *pr.MergedAt); err != nil {
					slog.Warn("Forgejo PR merged_at parse error", "pr", pr.Number, "value", *pr.MergedAt, "error", err)
				} else {
					pull.MergedAt = t
				}
			}
			if pr.MergedBy != nil {
				pull.MergedBy = pr.MergedBy.Login
			}
			if pr.MergeCommitSHA != "" {
				pull.Commits = []string{pr.MergeCommitSHA}
			}

			allPRs = append(allPRs, pull)
		}

		// Check if we got a full page (more might exist)
		if len(prs) < perPage {
			break
		}
		page++

		// Safety limit to avoid infinite loops
		if page > 100 {
			break
		}
	}

	return allPRs, nil
}

// getReviews fetches reviews for a PR
func (f *Forgejo) getReviews(ctx context.Context, number int) ([]Review, error) {
	if err := f.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/pulls/%d/reviews",
		f.baseURL, f.owner, f.repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if f.token != "" {
		req.Header.Set("Authorization", "token "+f.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get reviews: %s", resp.Status)
	}

	var forgejoReviews []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State       string `json:"state"`
		Body        string `json:"body"`
		SubmittedAt string `json:"submitted_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&forgejoReviews); err != nil {
		return nil, err
	}

	var reviews []Review
	for _, r := range forgejoReviews {
		review := Review{
			PRNumber: number,
			Author:   r.User.Login,
			State:    mapForgejoReviewState(r.State),
			Body:     r.Body,
		}
		if r.SubmittedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.SubmittedAt); err != nil {
				slog.Warn("Forgejo review submitted_at parse error", "author", r.User.Login, "value", r.SubmittedAt, "error", err)
			} else {
				review.CreatedAt = t
			}
		}
		reviews = append(reviews, review)
	}

	return reviews, nil
}

// getComments fetches issue comments (discussion) for a PR
func (f *Forgejo) getComments(ctx context.Context, number int) ([]Comment, error) {
	if err := f.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d/comments",
		f.baseURL, f.owner, f.repo, number)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if f.token != "" {
		req.Header.Set("Authorization", "token "+f.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get comments: %s", resp.Status)
	}

	var forgejoComments []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&forgejoComments); err != nil {
		return nil, err
	}

	var comments []Comment
	for _, c := range forgejoComments {
		comment := Comment{
			Author: c.User.Login,
			Body:   c.Body,
		}
		if c.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, c.CreatedAt); err != nil {
				slog.Warn("Forgejo comment created_at parse error", "author", c.User.Login, "value", c.CreatedAt, "error", err)
			} else {
				comment.CreatedAt = t
			}
		}
		comments = append(comments, comment)
	}

	return comments, nil
}

// mapForgejoReviewState maps Forgejo/Gitea review states to our ReviewState type
func mapForgejoReviewState(state string) ReviewState {
	// Forgejo/Gitea uses similar states to GitHub
	switch state {
	case "APPROVED":
		return ReviewApproved
	case "REQUEST_CHANGES":
		return ReviewChangesRequested
	case "COMMENT":
		return ReviewCommented
	case "PENDING":
		return ReviewPending
	default:
		return ReviewCommented
	}
}
