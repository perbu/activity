package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Run executes the update command
func (c *UpdateCmd) Run(ctx *Context) error {
	var repoNames []string

	if c.All {
		// Get all active repositories
		activeFlag := true
		repos, err := ctx.DB.ListRepositories(&activeFlag)
		if err != nil {
			return fmt.Errorf("failed to list repositories: %w", err)
		}

		if len(repos) == 0 {
			if !ctx.Quiet {
				slog.Info("No active repositories found")
			}
			return nil
		}

		for _, repo := range repos {
			repoNames = append(repoNames, repo.Name)
		}
	} else {
		// Use specified repositories
		if len(c.Repos) == 0 {
			return fmt.Errorf("requires repository names or --all flag")
		}
		repoNames = c.Repos
	}

	// Update each repository
	for _, name := range repoNames {
		if err := updateRepository(ctx, name, c.Analyze); err != nil {
			slog.Error("Failed to update repository", "name", name, "error", err)
			continue
		}
	}

	return nil
}

func updateRepository(ctx *Context, name string, analyze bool) error {
	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if !ctx.Quiet {
		slog.Info("Updating repository", "name", name)
	}

	// Get current SHA before pull
	beforeSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Fetch all branches and pull updates (with auth if private)
	if repo.Private {
		if ctx.TokenProvider == nil {
			return fmt.Errorf("repository '%s' is private but no GitHub App is configured", name)
		}
		token, err := ctx.TokenProvider.GetToken()
		if err != nil {
			return fmt.Errorf("failed to get GitHub token: %w", err)
		}
		// Fetch all remote branches first
		if err := git.FetchAllWithAuth(repo.LocalPath, repo.URL, token); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
		if err := git.PullWithAuth(repo.LocalPath, repo.URL, token); err != nil {
			return fmt.Errorf("failed to pull: %w", err)
		}
	} else {
		// Fetch all remote branches first
		if err := git.FetchAll(repo.LocalPath); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
		if err := git.Pull(repo.LocalPath); err != nil {
			return fmt.Errorf("failed to pull: %w", err)
		}
	}

	// Get SHA after pull
	afterSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to get updated SHA: %w", err)
	}

	// Update repository timestamp
	repo.UpdatedAt = time.Now()
	if err := ctx.DB.UpdateRepository(repo); err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	// Check if there were changes
	if beforeSHA == afterSHA {
		if !ctx.Quiet {
			slog.Info("Repository already up to date", "name", name)
		}
	} else {
		// Get commit range
		commits, err := git.GetCommitRange(repo.LocalPath, beforeSHA, afterSHA)
		if err != nil {
			return fmt.Errorf("failed to get commit range: %w", err)
		}

		if !ctx.Quiet {
			slog.Info("Repository updated", "name", name, "commits", len(commits))
			if ctx.Verbose {
				for _, commit := range commits {
					slog.Debug("New commit", "sha", commit.SHA[:7], "message", commit.Message)
				}
			}
		}

	}

	// Generate weekly report for the last complete week if --analyze is set
	if analyze {
		if err := generateLastWeekReport(ctx, repo); err != nil {
			slog.Warn("Failed to generate weekly report", "error", err)
		}
	}

	return nil
}

// generateLastWeekReport generates the weekly report for the previous complete week if it doesn't exist
func generateLastWeekReport(ctx *Context, repo *db.Repository) error {
	// Calculate the previous complete week
	now := time.Now()
	year, week := now.ISOWeek()

	// Go back to previous week
	week--
	if week < 1 {
		year--
		// Get the last week of the previous year
		lastDayOfPrevYear := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
		_, week = lastDayOfPrevYear.ISOWeek()
	}

	weekStr := git.FormatISOWeek(year, week)

	// Check if report already exists
	exists, err := ctx.DB.WeeklyReportExists(repo.ID, year, week)
	if err != nil {
		return fmt.Errorf("failed to check existing report: %w", err)
	}

	if exists {
		if ctx.Verbose {
			slog.Debug("Weekly report already exists, skipping", "week", weekStr)
		}
		return nil
	}

	// Get commits for this week
	commits, err := git.GetCommitsForWeek(repo.LocalPath, year, week)
	if err != nil {
		return fmt.Errorf("failed to get commits for %s: %w", weekStr, err)
	}

	if len(commits) == 0 {
		if ctx.Verbose {
			slog.Debug("No commits in week, skipping report", "week", weekStr)
		}
		return nil
	}

	// Get feature branch activity for this week
	branchActivity, err := git.GetFeatureBranchActivity(repo.LocalPath, repo.Branch, year, week)
	if err != nil {
		slog.Warn("Failed to get branch activity", "week", weekStr, "error", err)
		branchActivity = nil
	}

	if !ctx.Quiet {
		slog.Info("Generating weekly report", "week", weekStr, "commits", len(commits), "branches", len(branchActivity))
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(context.Background(), ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	defer llmClient.Close()

	// Create analyzer and generate report
	llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

	_, err = generateWeeklyReport(ctx, llmAnalyzer, repo, year, week, commits, branchActivity, false)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Weekly report generated", "week", weekStr)
	}

	return nil
}
