package analyzer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/perbu/activity/internal/git"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// shortSHA returns a shortened SHA for logging
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

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
	return "Retrieves the code diff for a specific commit. Vendor directories (vendor/, node_modules/) and lock files are filtered out by default. The response will indicate how many lines were suppressed. Use get_commit_diff_full if you need the complete unfiltered diff. Use ONLY when the commit message is unclear, vague, or lacks sufficient detail to understand what was changed. This is an expensive operation, so use it wisely."
}

// IsLongRunning returns false as this is a quick operation
func (t *GetCommitDiffTool) IsLongRunning() bool {
	return false
}

// ProcessRequest adds this tool to the LLM request
func (t *GetCommitDiffTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
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

	slog.Debug("tool call", "tool", "get_commit_diff", "sha", shortSHA(commitSHA), "reason", reason)

	// Pre-flight check: can we fetch more?
	canFetch, msg := t.costTracker.CanFetchMore()
	if !canFetch {
		slog.Debug("diff fetch denied", "sha", shortSHA(commitSHA), "reason", msg)
		return map[string]any{
			"error":   msg,
			"message": "Cannot fetch more diffs. Consider summarizing based on commit messages alone.",
		}, nil
	}

	// Fetch the diff
	result, err := git.GetCommitDiff(t.repoPath, commitSHA)
	if err != nil {
		slog.Debug("diff fetch error", "sha", shortSHA(commitSHA), "error", err)
		return map[string]any{
			"error":      fmt.Sprintf("Error fetching diff: %v", err),
			"commit_sha": commitSHA,
		}, nil
	}

	// Check size limit
	if len(result.Diff) > t.costTracker.GetMaxDiffSizeBytes() {
		slog.Debug("diff too large", "sha", shortSHA(commitSHA), "size", len(result.Diff), "max", t.costTracker.GetMaxDiffSizeBytes())
		return map[string]any{
			"error":      "Diff too large",
			"commit_sha": commitSHA,
			"size_bytes": len(result.Diff),
			"max_bytes":  t.costTracker.GetMaxDiffSizeBytes(),
			"message":    "The commit likely involves extensive changes. Consider this when summarizing.",
		}, nil
	}

	// Record the fetch
	t.costTracker.RecordDiffFetch(commitSHA, len(result.Diff), reason)

	lines := strings.Count(result.Diff, "\n")
	slog.Debug("diff fetched", "sha", shortSHA(commitSHA), "bytes", len(result.Diff), "lines", lines, "suppressed", result.SuppressedLines)

	return map[string]any{
		"commit_sha": commitSHA,
		"diff":       result.Diff,
		"size_bytes": len(result.Diff),
		"reason":     reason,
	}, nil
}

// GetCommitDiffFullTool provides access to unfiltered commit diffs for the agent
type GetCommitDiffFullTool struct {
	repoPath    string
	costTracker *CostTracker
}

// NewGetCommitDiffFullTool creates a new GetCommitDiffFullTool
func NewGetCommitDiffFullTool(repoPath string, costTracker *CostTracker) *GetCommitDiffFullTool {
	return &GetCommitDiffFullTool{
		repoPath:    repoPath,
		costTracker: costTracker,
	}
}

// Name returns the tool name
func (t *GetCommitDiffFullTool) Name() string {
	return "get_commit_diff_full"
}

// Description returns the tool description
func (t *GetCommitDiffFullTool) Description() string {
	return "Get the COMPLETE diff for a commit including vendor directories and lock files. Only use this if the filtered diff (from get_commit_diff) indicated suppressed lines and you need to see them. This is an expensive operation."
}

// IsLongRunning returns false as this is a quick operation
func (t *GetCommitDiffFullTool) IsLongRunning() bool {
	return false
}

// ProcessRequest adds this tool to the LLM request
func (t *GetCommitDiffFullTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
}

// Declaration returns the function declaration for the tool
func (t *GetCommitDiffFullTool) Declaration() *genai.FunctionDeclaration {
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
					Description: "Explanation for why the full unfiltered diff is needed",
				},
			},
			Required: []string{"commit_sha", "reason"},
		},
	}
}

// Run executes the tool
func (t *GetCommitDiffFullTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Parse arguments
	argsMap, ok := args.(map[string]any)
	if !ok {
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

	slog.Debug("tool call", "tool", "get_commit_diff_full", "sha", shortSHA(commitSHA), "reason", reason)

	// Pre-flight check: can we fetch more?
	canFetch, msg := t.costTracker.CanFetchMore()
	if !canFetch {
		slog.Debug("full diff fetch denied", "sha", shortSHA(commitSHA), "reason", msg)
		return map[string]any{
			"error":   msg,
			"message": "Cannot fetch more diffs. Consider summarizing based on commit messages alone.",
		}, nil
	}

	// Fetch the full unfiltered diff
	diff, err := git.GetCommitDiffFull(t.repoPath, commitSHA)
	if err != nil {
		slog.Debug("full diff fetch error", "sha", shortSHA(commitSHA), "error", err)
		return map[string]any{
			"error":      fmt.Sprintf("Error fetching full diff: %v", err),
			"commit_sha": commitSHA,
		}, nil
	}

	// Check size limit
	if len(diff) > t.costTracker.GetMaxDiffSizeBytes() {
		slog.Debug("full diff too large", "sha", shortSHA(commitSHA), "size", len(diff), "max", t.costTracker.GetMaxDiffSizeBytes())
		return map[string]any{
			"error":      "Diff too large",
			"commit_sha": commitSHA,
			"size_bytes": len(diff),
			"max_bytes":  t.costTracker.GetMaxDiffSizeBytes(),
			"message":    "The commit involves extensive changes. Consider this when summarizing.",
		}, nil
	}

	// Record the fetch
	t.costTracker.RecordDiffFetch(commitSHA, len(diff), "full: "+reason)

	lines := strings.Count(diff, "\n")
	slog.Debug("full diff fetched", "sha", shortSHA(commitSHA), "bytes", len(diff), "lines", lines)

	return map[string]any{
		"commit_sha": commitSHA,
		"diff":       diff,
		"size_bytes": len(diff),
		"reason":     reason,
		"note":       "This is the complete unfiltered diff including vendor/node_modules/lock files",
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

// ProcessRequest adds this tool to the LLM request
func (t *GetFullCommitMessageTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
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

	slog.Debug("tool call", "tool", "get_full_commit_message", "sha", shortSHA(commitSHA))

	commit, err := git.GetCommitInfo(t.repoPath, commitSHA)
	if err != nil {
		slog.Debug("commit info error", "sha", shortSHA(commitSHA), "error", err)
		return map[string]any{
			"error":      fmt.Sprintf("Error fetching commit info: %v", err),
			"commit_sha": commitSHA,
		}, nil
	}

	slog.Debug("commit message fetched", "sha", shortSHA(commitSHA), "length", len(commit.Message))

	return map[string]any{
		"commit_sha":     commitSHA,
		"author":         commit.Author,
		"date":           commit.Date.Format("2006-01-02 15:04"),
		"full_message":   commit.Message,
		"message_length": len(commit.Message),
	}, nil
}

// GetAuthorStatsTool provides author statistics for the agent
type GetAuthorStatsTool struct {
	repoPath string
}

// NewGetAuthorStatsTool creates a new GetAuthorStatsTool
func NewGetAuthorStatsTool(repoPath string) *GetAuthorStatsTool {
	return &GetAuthorStatsTool{
		repoPath: repoPath,
	}
}

// Name returns the tool name
func (t *GetAuthorStatsTool) Name() string {
	return "get_author_stats"
}

// Description returns the tool description
func (t *GetAuthorStatsTool) Description() string {
	return "Retrieves statistics about an author's contributions to the repository, including total commits and when they started contributing. Use this to provide context about contributors in your summary."
}

// IsLongRunning returns false as this is a quick operation
func (t *GetAuthorStatsTool) IsLongRunning() bool {
	return false
}

// ProcessRequest adds this tool to the LLM request
func (t *GetAuthorStatsTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
}

// Declaration returns the function declaration for the tool
func (t *GetAuthorStatsTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"author_name": {
					Type:        "string",
					Description: "The author name exactly as it appears in the commits (e.g., 'John Doe')",
				},
			},
			Required: []string{"author_name"},
		},
	}
}

// Run executes the tool
func (t *GetAuthorStatsTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Parse arguments
	argsMap, ok := args.(map[string]any)
	if !ok {
		if argsStr, ok := args.(string); ok {
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
				return map[string]any{"error": "invalid arguments format"}, nil
			}
		} else {
			return map[string]any{"error": "invalid arguments type"}, nil
		}
	}

	authorName, ok := argsMap["author_name"].(string)
	if !ok {
		return map[string]any{"error": "author_name must be a string"}, nil
	}

	slog.Debug("tool call", "tool", "get_author_stats", "author", authorName)

	stats, err := git.GetAuthorStats(t.repoPath, authorName)
	if err != nil {
		slog.Debug("author stats error", "author", authorName, "error", err)
		return map[string]any{
			"error":       fmt.Sprintf("Error fetching author stats: %v", err),
			"author_name": authorName,
		}, nil
	}

	if stats.TotalCommits == 0 {
		slog.Debug("author not found", "author", authorName)
		return map[string]any{
			"author_name":   authorName,
			"total_commits": 0,
			"message":       "No commits found for this author",
		}, nil
	}

	slog.Debug("author stats fetched", "author", stats.Name, "commits", stats.TotalCommits)

	return map[string]any{
		"author_name":   stats.Name,
		"total_commits": stats.TotalCommits,
		"first_commit":  stats.FirstCommit.Format("2006-01-02"),
		"last_commit":   stats.LastCommit.Format("2006-01-02"),
	}, nil
}

// functionTool is an interface for tools that provide function declarations
type functionTool interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
}

// addFunctionTool adds a function tool to the LLM request
func addFunctionTool(req *model.LLMRequest, t functionTool) error {
	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}

	decl := t.Declaration()
	if decl == nil {
		return fmt.Errorf("tool %q has no declaration", t.Name())
	}

	// Add to tools map for execution lookup
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}
	req.Tools[t.Name()] = t

	// Add function declaration to config
	req.Config.Tools = append(req.Config.Tools, &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{decl},
	})

	return nil
}
