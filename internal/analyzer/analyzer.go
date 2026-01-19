package analyzer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

type Analyzer struct {
	llmClient *llm.Client
	db        *db.DB
	config    *config.Config
}

// New creates a new Analyzer
func New(llmClient *llm.Client, database *db.DB, cfg *config.Config) *Analyzer {
	return &Analyzer{
		llmClient: llmClient,
		db:        database,
		config:    cfg,
	}
}

// AnalyzeCommits analyzes a range of commits and returns a summary
// Routes to either Phase 2 (simple LLM) or Phase 3 (agent) based on config
// previousSummary provides context from the previous week's report for narrative continuity
func (a *Analyzer) AnalyzeCommits(ctx context.Context, repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string) (string, error) {
	if len(commits) == 0 {
		return "No new commits to analyze.", nil
	}

	// Route to agent-based or simple analyzer
	if a.config.LLM.UseAgent {
		summary, _, err := a.analyzeWithAgent(ctx, repo, commits, branchActivity, previousSummary)
		return summary, err
	}

	// Fall back to Phase 2 simple analyzer
	return a.analyzeWithSimpleLLM(ctx, repo, commits, branchActivity, previousSummary)
}

// analyzeWithSimpleLLM performs simple LLM-based analysis (Phase 2)
func (a *Analyzer) analyzeWithSimpleLLM(ctx context.Context, repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string) (string, error) {
	// Build prompt from commits
	prompt := buildAnalysisPrompt(repo, commits, branchActivity, a.config, previousSummary)

	// Call LLM
	summary, err := a.llmClient.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	return summary, nil
}

// AnalyzeAndSave performs analysis and saves to database
// previousSummary provides context from the previous week's report for narrative continuity
func (a *Analyzer) AnalyzeAndSave(ctx context.Context, repo *db.Repository, fromSHA, toSHA string, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string) (*db.ActivityRun, error) {
	// Create activity run record
	run, err := a.db.CreateActivityRun(repo.ID, fromSHA, toSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity run: %w", err)
	}

	// Store metadata as JSON
	metadata := map[string]interface{}{
		"commit_count": len(commits),
		"authors":      extractAuthors(commits),
		"date_range": map[string]string{
			"start": commits[len(commits)-1].Date.Format(time.RFC3339),
			"end":   commits[0].Date.Format(time.RFC3339),
		},
	}

	// Track whether agent mode was used
	run.AgentMode = a.config.LLM.UseAgent

	// Generate summary
	var summary string
	if a.config.LLM.UseAgent {
		// Use agent analyzer and capture cost tracking
		var costTracker *CostTracker
		summary, costTracker, err = a.analyzeWithAgent(ctx, repo, commits, branchActivity, previousSummary)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze commits with agent: %w", err)
		}

		// Store cost tracking metadata
		costMetadata := costTracker.GetMetadata()
		costJSON, _ := json.Marshal(costMetadata)
		run.ToolUsageStats = sql.NullString{String: string(costJSON), Valid: true}

		// Add cost info to metadata
		metadata["agent_diffs_fetched"] = costTracker.GetDiffsFetched()
		metadata["agent_estimated_tokens"] = costTracker.GetEstimatedTokens()
	} else {
		// Use simple LLM analyzer
		summary, err = a.analyzeWithSimpleLLM(ctx, repo, commits, branchActivity, previousSummary)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze commits: %w", err)
		}
	}

	rawData, _ := json.Marshal(metadata)

	// Update run with results
	run.Summary = sql.NullString{String: summary, Valid: true}
	run.RawData = sql.NullString{String: string(rawData), Valid: true}
	run.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	if err := a.db.UpdateActivityRun(run); err != nil {
		return nil, fmt.Errorf("failed to update activity run: %w", err)
	}

	return run, nil
}

// buildAnalysisPrompt creates the prompt for LLM analysis
func buildAnalysisPrompt(repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, cfg *config.Config, previousSummary string) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing git commits for a software project.\n\n")
	sb.WriteString(fmt.Sprintf("Repository: %s\n", repo.Name))
	if repo.Description.Valid && repo.Description.String != "" {
		sb.WriteString(fmt.Sprintf("About: %s\n", repo.Description.String))
	}
	sb.WriteString(fmt.Sprintf("Branch: %s\n", repo.Branch))
	sb.WriteString(fmt.Sprintf("Total commits: %d\n\n", len(commits)))

	sb.WriteString("Commits (newest first):\n\n")

	// Use configurable max commits limit
	maxCommits := cfg.LLM.MaxCommits
	if maxCommits <= 0 {
		maxCommits = 50 // Fallback to default
	}

	limit := len(commits)
	if limit > maxCommits {
		limit = maxCommits
	}

	// Max message length
	maxMsgLen := cfg.LLM.MaxMessageLength
	if maxMsgLen <= 0 {
		maxMsgLen = 1000 // Fallback to default
	}

	for i := 0; i < limit; i++ {
		commit := commits[i]
		sb.WriteString(fmt.Sprintf("Commit %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  SHA: %s\n", commit.SHA[:8]))
		sb.WriteString(fmt.Sprintf("  Author: %s\n", commit.Author))
		sb.WriteString(fmt.Sprintf("  Date: %s\n", commit.Date.Format("2006-01-02 15:04")))

		// Truncate long commit messages
		message := commit.Message
		if len(message) > maxMsgLen {
			message = message[:maxMsgLen] + "... [truncated]"
		}
		sb.WriteString(fmt.Sprintf("  Message: %s\n\n", message))
	}

	if len(commits) > maxCommits {
		sb.WriteString(fmt.Sprintf("... and %d more commits\n\n", len(commits)-maxCommits))
	}

	// Include branch activity if present
	if len(branchActivity) > 0 {
		sb.WriteString("## Other Branch Activity\n")
		sb.WriteString("The following feature branches had commits this week that haven't been merged to the main branch:\n")
		for _, ba := range branchActivity {
			sb.WriteString(fmt.Sprintf("- %s: %d commits (", ba.BranchName, ba.CommitCount))
			first := true
			for author, count := range ba.AuthorCounts {
				if !first {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s: %d", author, count))
				first = false
			}
			sb.WriteString(")\n")
		}
		sb.WriteString("\nInclude a brief mention of this parallel work in your summary.\n\n")
	}

	// Include previous week's summary for context
	if previousSummary != "" {
		sb.WriteString("## Previous Week's Summary (for context)\n")
		sb.WriteString(previousSummary)
		sb.WriteString("\n\nUse this context to maintain narrative continuity and reference ongoing work where relevant.\n\n")
	}

	// Use configured prompt (or default)
	sb.WriteString(cfg.GetPhase2Prompt())
	sb.WriteString("\n")

	return sb.String()
}

// extractAuthors gets unique author list from commits
func extractAuthors(commits []git.Commit) []string {
	authors := make(map[string]bool)
	for _, c := range commits {
		authors[c.Author] = true
	}

	result := make([]string, 0, len(authors))
	for author := range authors {
		result = append(result, author)
	}
	return result
}
