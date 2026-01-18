package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Run executes the analyze command
func (c *AnalyzeCmd) Run(ctx *Context) error {
	// Validate flags: at least one of --since, --until, or -n must be provided
	if c.Since == "" && c.Until == "" && c.N == 0 {
		return fmt.Errorf("at least one of --since, --until, or -n must be provided")
	}

	// Validate flags: -n is mutually exclusive with date flags
	if c.N > 0 && (c.Since != "" || c.Until != "") {
		return fmt.Errorf("-n cannot be used with --since or --until")
	}

	for i, name := range c.Repos {
		if i > 0 {
			fmt.Println()
		}

		if err := analyzeRepository(ctx, name, c.Since, c.Until, c.N, c.Limit); err != nil {
			slog.Error("Failed to analyze repository", "name", name, "error", err)
			continue
		}
	}

	return nil
}

func analyzeRepository(ctx *Context, name, since, until string, n, limit int) error {
	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	// Get commits based on flags
	var commits []git.Commit
	if n > 0 {
		commits, err = git.GetLastNCommits(repo.LocalPath, n)
	} else {
		commits, err = git.GetCommitsSince(repo.LocalPath, since, until)
	}
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Display header
	fmt.Printf("Repository: %s\n", repo.Name)
	fmt.Printf("Branch: %s\n", repo.Branch)
	if n > 0 {
		fmt.Printf("Analyzing: last %d commits\n", n)
	} else {
		rangeDesc := "since " + since
		if until != "" {
			rangeDesc += " until " + until
		}
		fmt.Printf("Analyzing: %s\n", rangeDesc)
	}
	// Show commit range if we have commits
	if len(commits) > 0 {
		oldest := commits[len(commits)-1]
		newest := commits[0]
		fmt.Printf("Range: %s (%s) - %s (%s)\n",
			oldest.Date.Format("2006-01-02"), oldest.SHA[:7],
			newest.Date.Format("2006-01-02"), newest.SHA[:7])
	}
	fmt.Printf("Model: %s\n", ctx.Config.LLM.Model)
	fmt.Println()

	// Check if there are commits to analyze
	if len(commits) == 0 {
		fmt.Println("No commits found in the specified range")
		return nil
	}

	// Determine SHA range for analyzer
	var fromSHA, toSHA string
	toSHA = commits[0].SHA // Most recent commit
	if len(commits) > 1 {
		fromSHA = commits[len(commits)-1].SHA // Oldest commit (exclusive in git range)
	}

	// Check if we should use AI analysis
	// Initialize LLM client
	llmClient, err := llm.NewClient(context.Background(), ctx.Config)
	if err != nil {
		slog.Warn("Failed to initialize AI analysis, falling back to git log", "error", err)
		// Fall through to show commits
	} else {
		defer llmClient.Close()

		// Create analyzer
		llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

		// Analyze and save
		run, err := llmAnalyzer.AnalyzeAndSave(context.Background(), repo, fromSHA, toSHA, commits)
		if err != nil {
			slog.Warn("Analysis failed, falling back to git log", "error", err)
			// Fall through to show commits
		} else {
			// Display AI-generated summary
			fmt.Printf("AI Analysis Summary (%d commits analyzed):\n", len(commits))
			fmt.Println(strings.Repeat("=", 70))
			fmt.Println()
			fmt.Println(run.Summary.String)
			fmt.Println()
			fmt.Println(strings.Repeat("=", 70))

			// NOTE: analyze command does not update LastRunSHA/LastRunAt
			// Use 'update --analyze' for that behavior

			return nil
		}
	}

	// Fallback: show git log
	// Display commits
	fmt.Printf("Recent Activity (%d commit(s)):\n", len(commits))
	fmt.Println()

	displayLimit := limit
	if len(commits) < displayLimit {
		displayLimit = len(commits)
	}

	for i := 0; i < displayLimit; i++ {
		commit := commits[i]
		fmt.Printf("  Commit:  %s\n", commit.SHA[:7])
		fmt.Printf("  Author:  %s\n", commit.Author)
		fmt.Printf("  Date:    %s\n", commit.Date.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Message: %s\n", commit.Message)
		fmt.Println()
	}

	if len(commits) > displayLimit {
		fmt.Printf("  ... and %d more commits (use --limit to see more)\n", len(commits)-displayLimit)
	}

	return nil
}
