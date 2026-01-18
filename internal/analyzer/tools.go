package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/perbu/activity/internal/git"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// GetCommitDiffTool provides access to commit diffs for the agent
type GetCommitDiffTool struct {
	repoPath    string
	costTracker *CostTracker
}

// NewGetCommitDiffTool creates a new GetCommitDiffTool
func NewGetCommitDiffTool(repoPath string, costTracker *CostTracker) *GetCommitDiffTool {
	return &GetCommitDiffTool{
		repoPath:    repoPath,
		costTracker: costTracker,
	}
}

// Name returns the tool name
func (t *GetCommitDiffTool) Name() string {
	return "get_commit_diff"
}

// Description returns the tool description
func (t *GetCommitDiffTool) Description() string {
	return "Retrieves the code diff for a specific commit. Use ONLY when the commit message is unclear, vague, or lacks sufficient detail to understand what was changed. This is an expensive operation, so use it wisely."
}

// IsLongRunning returns false as this is a quick operation
func (t *GetCommitDiffTool) IsLongRunning() bool {
	return false
}

// Declaration returns the function declaration for the tool
func (t *GetCommitDiffTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"commit_sha": {
					Type:        "string",
					Description: "The commit SHA (can be full 40-char or shortened 8-char form)",
				},
				"reason": {
					Type:        "string",
					Description: "Explanation for why the diff is needed (e.g., 'commit message is vague', 'need to verify scope of change')",
				},
			},
			Required: []string{"commit_sha", "reason"},
		},
	}
}

// Run executes the tool
func (t *GetCommitDiffTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Parse arguments
	argsMap, ok := args.(map[string]any)
	if !ok {
		// Try JSON unmarshaling if args is a string or bytes
		if argsStr, ok := args.(string); ok {
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
				return map[string]any{"error": "invalid arguments format"}, nil
			}
		} else {
			return map[string]any{"error": "invalid arguments type"}, nil
		}
	}

	commitSHA, ok := argsMap["commit_sha"].(string)
	if !ok {
		return map[string]any{"error": "commit_sha must be a string"}, nil
	}

	reason, ok := argsMap["reason"].(string)
	if !ok {
		return map[string]any{"error": "reason must be a string"}, nil
	}

	// Pre-flight check: can we fetch more?
	canFetch, msg := t.costTracker.CanFetchMore()
	if !canFetch {
		return map[string]any{
			"error":   msg,
			"message": "Cannot fetch more diffs. Consider summarizing based on commit messages alone.",
		}, nil
	}

	// Fetch the diff
	diff, err := git.GetCommitDiff(t.repoPath, commitSHA)
	if err != nil {
		return map[string]any{
			"error":      fmt.Sprintf("Error fetching diff: %v", err),
			"commit_sha": commitSHA,
		}, nil
	}

	// Check size limit
	if len(diff) > t.costTracker.GetMaxDiffSizeBytes() {
		return map[string]any{
			"error":      "Diff too large",
			"commit_sha": commitSHA,
			"size_bytes": len(diff),
			"max_bytes":  t.costTracker.GetMaxDiffSizeBytes(),
			"message":    "The commit likely involves extensive changes. Consider this when summarizing.",
		}, nil
	}

	// Record the fetch
	t.costTracker.RecordDiffFetch(commitSHA, len(diff), reason)

	return map[string]any{
		"commit_sha": commitSHA,
		"diff":       diff,
		"size_bytes": len(diff),
		"reason":     reason,
	}, nil
}

// GetFullCommitMessageTool provides access to full commit messages
type GetFullCommitMessageTool struct {
	repoPath string
}

// NewGetFullCommitMessageTool creates a new GetFullCommitMessageTool
func NewGetFullCommitMessageTool(repoPath string) *GetFullCommitMessageTool {
	return &GetFullCommitMessageTool{
		repoPath: repoPath,
	}
}

// Name returns the tool name
func (t *GetFullCommitMessageTool) Name() string {
	return "get_full_commit_message"
}

// Description returns the tool description
func (t *GetFullCommitMessageTool) Description() string {
	return "Retrieves the full commit message including the body. Use this when the commit message was truncated and you need more context before deciding whether to fetch the diff."
}

// IsLongRunning returns false as this is a quick operation
func (t *GetFullCommitMessageTool) IsLongRunning() bool {
	return false
}

// Declaration returns the function declaration for the tool
func (t *GetFullCommitMessageTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"commit_sha": {
					Type:        "string",
					Description: "The commit SHA (can be full 40-char or shortened 8-char form)",
				},
			},
			Required: []string{"commit_sha"},
		},
	}
}

// Run executes the tool
func (t *GetFullCommitMessageTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Parse arguments
	argsMap, ok := args.(map[string]any)
	if !ok {
		// Try JSON unmarshaling if args is a string or bytes
		if argsStr, ok := args.(string); ok {
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
				return map[string]any{"error": "invalid arguments format"}, nil
			}
		} else {
			return map[string]any{"error": "invalid arguments type"}, nil
		}
	}

	commitSHA, ok := argsMap["commit_sha"].(string)
	if !ok {
		return map[string]any{"error": "commit_sha must be a string"}, nil
	}

	commit, err := git.GetCommitInfo(t.repoPath, commitSHA)
	if err != nil {
		return map[string]any{
			"error":      fmt.Sprintf("Error fetching commit info: %v", err),
			"commit_sha": commitSHA,
		}, nil
	}

	return map[string]any{
		"commit_sha":     commitSHA,
		"author":         commit.Author,
		"date":           commit.Date.Format("2006-01-02 15:04"),
		"full_message":   commit.Message,
		"message_length": len(commit.Message),
	}, nil
}
