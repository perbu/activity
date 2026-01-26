package analyzer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// buildAgentPrompt creates the user prompt for the agent
func buildAgentPrompt(repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, maxMessageLength int, previousSummary string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Repository: %s\n", repo.Name))
	if repo.Description.Valid && repo.Description.String != "" {
		sb.WriteString(fmt.Sprintf("About: %s\n", repo.Description.String))
	}
	sb.WriteString(fmt.Sprintf("Branch: %s\n", repo.Branch))
	sb.WriteString(fmt.Sprintf("Analyzing %d commits\n\n", len(commits)))
	sb.WriteString("Commits (newest first):\n\n")

	for i, commit := range commits {
		sb.WriteString(fmt.Sprintf("Commit %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  SHA: %s\n", commit.SHA[:8]))
		sb.WriteString(fmt.Sprintf("  Author: %s\n", commit.Author))
		sb.WriteString(fmt.Sprintf("  Date: %s\n", commit.Date.Format("2006-01-02")))

		message := commit.Message
		truncated := false
		if len(message) > maxMessageLength {
			message = message[:maxMessageLength]
			truncated = true
		}
		sb.WriteString(fmt.Sprintf("  Message: %s", message))
		if truncated {
			sb.WriteString(" [truncated - use get_full_commit_message for complete text]")
		}
		sb.WriteString("\n\n")
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

	sb.WriteString("Please analyze these commits and provide a summary.\n")
	return sb.String()
}

// createAnalyzerAgent creates an ADK agent with tools for commit analysis
func (a *Analyzer) createAnalyzerAgent(ctx context.Context, repoPath string, costTracker *CostTracker) (agent.Agent, error) {
	// Get the Gemini model from the LLM client
	geminiModel, err := a.llmClient.GetGeminiModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gemini model: %w", err)
	}

	// Create tools
	diffTool := NewGetCommitDiffTool(repoPath, costTracker)
	diffFullTool := NewGetCommitDiffFullTool(repoPath, costTracker)
	msgTool := NewGetFullCommitMessageTool(repoPath)
	authorTool := NewGetAuthorStatsTool(repoPath)

	// Get system prompt from config (with default fallback)
	systemPrompt := a.config.GetAgentSystemPrompt()

	// Create agent configuration
	agentConfig := llmagent.Config{
		Name:        "git_analyzer",
		Description: "Analyzes git commits and provides summaries",
		Model:       geminiModel,
		Instruction: fmt.Sprintf(systemPrompt, a.config.LLM.MaxDiffFetches),
		Tools:       []tool.Tool{diffTool, diffFullTool, msgTool, authorTool},
	}

	// Create the agent
	return llmagent.New(agentConfig)
}

// analyzeWithAgent performs commit analysis using an ADK agent
func (a *Analyzer) analyzeWithAgent(ctx context.Context, repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, previousSummary string) (string, *CostTracker, error) {
	// Create cost tracker
	costTracker := NewCostTracker(
		a.config.LLM.MaxDiffFetches,
		a.config.LLM.MaxDiffSizeKB*1024,
		a.config.LLM.MaxTotalTokens,
	)

	// Compute repo path from config
	repoPath := db.RepoLocalPath(a.config.DataDir, repo.Name)

	// Create agent
	agt, err := a.createAnalyzerAgent(ctx, repoPath, costTracker)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Build user prompt
	userPrompt := buildAgentPrompt(repo, commits, branchActivity, a.config.LLM.MaxMessageLength, previousSummary)

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

	return summary.String(), costTracker, nil
}
