package analyzer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/forge"
	"github.com/perbu/activity/internal/git"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// createAnalyzerAgent creates an ADK agent with tools for commit analysis
func (a *Analyzer) createAnalyzerAgent(ctx context.Context, repo *db.Repository, repoPath string, costTracker *CostTracker, f forge.Forge, since, until time.Time, logger *AnalysisLogger) (agent.Agent, error) {
	// Get the Gemini model from the LLM client
	geminiModel, err := a.llmClient.GetModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gemini model: %w", err)
	}

	// Create tools - base tools always included
	diffTool := NewGetCommitDiffTool(repoPath, costTracker, logger)
	diffFullTool := NewGetCommitDiffFullTool(repoPath, costTracker, logger)
	msgTool := NewGetFullCommitMessageTool(repoPath, logger)
	tools := []tool.Tool{diffTool, diffFullTool, msgTool}

	// Only include author stats tool for external repositories
	if repo.External {
		authorTool := NewGetAuthorStatsTool(repoPath, logger)
		tools = append(tools, authorTool)
	}

	// Add forge tool if configured
	if f != nil {
		forgeTool := NewGetPRReviewsTool(f, since, until)
		tools = append(tools, forgeTool)
	}

	// Get system prompt based on repo type and forge availability
	systemPrompt := a.config.GetAgentSystemPromptForRepo(repo.External, f != nil)

	// Create agent configuration
	agentConfig := llmagent.Config{
		Name:        "git_analyzer",
		Description: "Analyzes git commits and provides summaries",
		Model:       geminiModel,
		Instruction: fmt.Sprintf(systemPrompt, a.config.LLM.MaxDiffFetches),
		Tools:       tools,
	}

	// Create the agent
	return llmagent.New(agentConfig)
}

// analyzeWithAgent performs commit analysis using an ADK agent
func (a *Analyzer) analyzeWithAgent(ctx context.Context, repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string, logger *AnalysisLogger) (string, *CostTracker, error) {
	// Create cost tracker
	costTracker := NewCostTracker(
		a.config.LLM.MaxDiffFetches,
		a.config.LLM.MaxDiffSizeKB*1024,
		a.config.LLM.MaxTotalTokens,
	)

	// Compute repo path from config
	repoPath := db.RepoLocalPath(a.config.DataDir, repo.Name)

	// Determine date range from commits
	var since, until time.Time
	if len(commits) > 0 {
		since = commits[len(commits)-1].Date
		until = commits[0].Date
	} else {
		since = time.Now().AddDate(0, 0, -7)
		until = time.Now()
	}

	// Create forge client for this repo (nil if not configured)
	f, err := forge.New(repo, a.config)
	if err != nil {
		slog.Warn("Failed to create forge client, continuing without PR data", "repo", repo.Name, "error", err)
	}

	// Pre-fetch PR data so it's included directly in the prompt
	var prData string
	if f != nil {
		prs, err := f.ListMergedPRs(ctx, since, until)
		if err != nil {
			slog.Warn("Failed to pre-fetch PR data, agent can still use tool", "repo", repo.Name, "error", err)
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

	agt, err := a.createAnalyzerAgent(ctx, repo, repoPath, costTracker, f, since, until, logger)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Build user prompt
	userPrompt := buildAgentPrompt(repo, commits, branchActivity, a.config.LLM.MaxMessageLength, previousSummary, prData)

	// Log prompt if logger available
	if logger != nil {
		logger.LogPrompt(userPrompt)
	}

	slog.Debug("agent starting analysis", "repo", repo.Name, "commits", len(commits))

	// Create a runner with in-memory session
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "activity-analyzer",
		Agent:          agt,
		SessionService: sessionService,
	})
	if err != nil {
		return "", costTracker, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create the session before running
	_, err = sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "activity-analyzer",
		UserID:    "user1",
		SessionID: "session1",
	})
	if err != nil {
		return "", costTracker, fmt.Errorf("failed to create session: %w", err)
	}

	// Create user message content
	userMessage := genai.NewContentFromText(userPrompt, genai.RoleUser)

	// Execute agent with the user message
	var summary strings.Builder
	for event, err := range r.Run(ctx, "user1", "session1", userMessage, agent.RunConfig{}) {
		if err != nil {
			return "", costTracker, fmt.Errorf("agent execution failed: %w", err)
		}
		if event != nil && event.Content != nil {
			// Extract text from all parts in the content
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					summary.WriteString(part.Text)
				}
			}
		}
	}

	slog.Debug("agent analysis complete", "diffs_fetched", costTracker.GetDiffsFetched(), "tokens", costTracker.GetEstimatedTokens())
	slog.Info("analysis complete", "repo", repo.Name, "commits", len(commits), "diffs", costTracker.GetDiffsFetched())

	// Log response if logger available
	if logger != nil {
		logger.LogResponse(summary.String())
	}

	return summary.String(), costTracker, nil
}
