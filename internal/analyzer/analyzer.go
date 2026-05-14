package analyzer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/forge"
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

	// Route to agent-based or simple analyzer (no logger for standalone calls)
	if a.config.LLM.UseAgent {
		summary, _, err := a.analyzeWithAgent(ctx, repo, commits, branchActivity, previousSummary, nil)
		return summary, err
	}

	// Fall back to Phase 2 simple analyzer
	return a.analyzeWithSimpleLLM(ctx, repo, commits, branchActivity, previousSummary, nil)
}

// analyzeWithSimpleLLM performs simple LLM-based analysis (Phase 2)
func (a *Analyzer) analyzeWithSimpleLLM(ctx context.Context, repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string, logger *AnalysisLogger) (string, error) {
	// Pre-fetch PR data if forge is available
	var prData string
	f, err := forge.New(repo, a.config)
	if err != nil {
		slog.Debug("No forge client for simple LLM analysis", "repo", repo.Name, "error", err)
	}
	if f != nil && len(commits) > 0 {
		since := commits[len(commits)-1].Date
		until := commits[0].Date
		prs, err := f.ListMergedPRs(ctx, since, until)
		if err != nil {
			slog.Warn("Failed to pre-fetch PR data for simple LLM", "repo", repo.Name, "error", err)
		} else {
			maxBytes := a.config.LLM.MaxPRDataSizeKB * 1024
			prData = FormatPRData(prs, maxBytes)
			if prData != "" {
				slog.Info("pre-fetched PR data for prompt", "repo", repo.Name, "prs", len(prs), "bytes", len(prData))
				if logger != nil {
					logger.LogPRData(prData)
				}
			}
		}
	}

	// Build prompt from commits
	prompt := buildAnalysisPrompt(repo, commits, branchActivity, a.config, previousSummary, prData)

	// Log prompt if logger available
	if logger != nil {
		logger.LogPrompt(prompt)
	}

	// Call LLM
	summary, err := a.llmClient.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	// Log response if logger available
	if logger != nil {
		logger.LogResponse(summary)
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

	// Create logger for this run
	logger := NewAnalysisLogger(a.db, run.ID)

	// Store metadata as JSON
	metadata := map[string]any{
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
		summary, costTracker, err = a.analyzeWithAgent(ctx, repo, commits, branchActivity, previousSummary, logger)
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
		summary, err = a.analyzeWithSimpleLLM(ctx, repo, commits, branchActivity, previousSummary, logger)
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
