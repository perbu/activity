package analyzer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/perbu/activity/internal/forge"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// GetPRReviewsTool allows the agent to fetch PR and review data
type GetPRReviewsTool struct {
	forge forge.Forge
	since time.Time
	until time.Time
}

// NewGetPRReviewsTool creates a new tool for fetching PR reviews
func NewGetPRReviewsTool(f forge.Forge, since, until time.Time) *GetPRReviewsTool {
	return &GetPRReviewsTool{
		forge: f,
		since: since,
		until: until,
	}
}

// Name returns the tool name
func (t *GetPRReviewsTool) Name() string {
	return "get_pr_reviews"
}

// Description returns the tool description
func (t *GetPRReviewsTool) Description() string {
	return `Get pull request and code review information for the analysis period.
Returns merged PRs with their reviews, including who reviewed, approval status, and comments.
Use this to understand code review patterns and who is doing reviews.`
}

// IsLongRunning returns false as this fetches cached data
func (t *GetPRReviewsTool) IsLongRunning() bool {
	return false
}

// ProcessRequest adds this tool to the LLM request
func (t *GetPRReviewsTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
}

// Declaration returns the function declaration for the tool
func (t *GetPRReviewsTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type:       "object",
			Properties: map[string]*genai.Schema{},
			Required:   []string{},
		},
	}
}

// FormatPRData formats PR and review data into a human-readable string, truncated to maxBytes.
// Returns empty string if prs is empty.
func FormatPRData(prs []forge.PRWithReviews, maxBytes int) string {
	if len(prs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d merged PRs:\n\n", len(prs)))

	for _, pr := range prs {
		sb.WriteString(fmt.Sprintf("PR #%d: %s\n", pr.PR.Number, pr.PR.Title))
		sb.WriteString(fmt.Sprintf("  Author: %s", pr.PR.Author))
		if !pr.PR.MergedAt.IsZero() {
			sb.WriteString(fmt.Sprintf(", Merged: %s", pr.PR.MergedAt.Format("2006-01-02")))
		}
		if pr.PR.MergedBy != "" {
			sb.WriteString(fmt.Sprintf(" by %s", pr.PR.MergedBy))
		}
		sb.WriteString("\n")

		if len(pr.Reviews) > 0 {
			sb.WriteString("  Reviews:\n")
			for _, r := range pr.Reviews {
				sb.WriteString(fmt.Sprintf("    - %s: %s", r.Author, r.State))
				if r.Body != "" {
					body := r.Body
					if len(body) > 200 {
						body = body[:200] + "..."
					}
					body = strings.ReplaceAll(body, "\n", " ")
					sb.WriteString(fmt.Sprintf(" (%s)", body))
				}
				sb.WriteString("\n")
			}
		}

		if len(pr.Comments) > 0 {
			sb.WriteString(fmt.Sprintf("  Discussion (%d comments):\n", len(pr.Comments)))
			for _, c := range pr.Comments {
				body := c.Body
				if len(body) > 200 {
					body = body[:200] + "..."
				}
				body = strings.ReplaceAll(body, "\n", " ")
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", c.Author, body))
			}
		}

		sb.WriteString("\n")

		if maxBytes > 0 && sb.Len() > maxBytes {
			// Truncate and add notice
			result := sb.String()[:maxBytes]
			return result + "\n... [PR data truncated due to size limit]\n"
		}
	}

	return sb.String()
}

// Run executes the tool
func (t *GetPRReviewsTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	slog.Debug("tool call", "tool", "get_pr_reviews")

	if t.forge == nil {
		return map[string]any{
			"message": "No forge configured for this repository - PR data unavailable.",
		}, nil
	}

	prs, err := t.forge.ListMergedPRs(ctx, t.since, t.until)
	if err != nil {
		slog.Debug("PR fetch error", "error", err)
		return map[string]any{
			"error":   fmt.Sprintf("Unable to fetch PR data: %s", err.Error()),
			"message": "PR data could not be fetched, proceed with commit data only.",
		}, nil
	}

	if len(prs) == 0 {
		return map[string]any{
			"message":  "No merged PRs found in this period.",
			"pr_count": 0,
		}, nil
	}

	details := FormatPRData(prs, 0) // No truncation for tool calls

	// Build reviewer summary
	reviewerCounts := make(map[string]int)
	approvalCounts := make(map[string]int)
	for _, pr := range prs {
		for _, r := range pr.Reviews {
			reviewerCounts[r.Author]++
			if r.State == forge.ReviewApproved {
				approvalCounts[r.Author]++
			}
		}
	}

	var reviewerSummary []map[string]any
	for reviewer, count := range reviewerCounts {
		reviewerSummary = append(reviewerSummary, map[string]any{
			"reviewer":  reviewer,
			"reviews":   count,
			"approvals": approvalCounts[reviewer],
		})
	}

	return map[string]any{
		"pr_count":   len(prs),
		"details":    details,
		"reviewers":  reviewerSummary,
		"forge_type": t.forge.Type(),
	}, nil
}

// parseArgs is a helper to parse tool arguments
func parseArgs(args any) (map[string]any, error) {
	argsMap, ok := args.(map[string]any)
	if ok {
		return argsMap, nil
	}

	// Try JSON unmarshaling if args is a string
	if argsStr, ok := args.(string); ok {
		var result map[string]any
		if err := json.Unmarshal([]byte(argsStr), &result); err != nil {
			return nil, fmt.Errorf("invalid arguments format: %w", err)
		}
		return result, nil
	}

	return nil, fmt.Errorf("invalid arguments type: %T", args)
}
