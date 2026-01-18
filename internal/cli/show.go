package cli

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Show displays activity for repositories
func Show(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("show", flag.ExitOnError)
	limit := flags.Int("limit", 10, "Maximum number of commits to show")

	if err := flags.Parse(args); err != nil {
		return err
	}

	repoNames := flags.Args()
	if len(repoNames) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: activity show <repo...> [--limit=10]\n")
		return fmt.Errorf("requires at least one repository name")
	}

	for i, name := range repoNames {
		if i > 0 {
			fmt.Println()
		}

		if err := showRepository(ctx, name, *limit); err != nil {
			fmt.Fprintf(os.Stderr, "Error showing %s: %v\n", name, err)
			continue
		}
	}

	return nil
}

func showRepository(ctx *Context, name string, limit int) error {
	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	// Get current SHA
	currentSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Determine commit range
	var fromSHA string
	if repo.LastRunSHA.Valid {
		fromSHA = repo.LastRunSHA.String
	}

	// Get commits
	commits, err := git.GetCommitRange(repo.LocalPath, fromSHA, currentSHA)
	if err != nil {
		return fmt.Errorf("failed to get commit range: %w", err)
	}

	// Display header
	fmt.Printf("Repository: %s\n", repo.Name)
	fmt.Printf("Branch: %s\n", repo.Branch)
	fmt.Printf("Current SHA: %s\n", currentSHA)
	if repo.LastRunSHA.Valid {
		fmt.Printf("Last Run SHA: %s\n", repo.LastRunSHA.String)
	}
	fmt.Println()

	// Check if there are new commits
	if len(commits) == 0 {
		fmt.Println("No new commits since last run")
		return nil
	}

	// Check if we should use AI analysis
	if len(commits) > 0 {
		// Initialize LLM client
		llmClient, err := llm.NewClient(context.Background(), ctx.Config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize AI analysis: %v\n", err)
			fmt.Println("Falling back to git log display.\n")
			// Fall through to show commits
		} else {
			defer llmClient.Close()

			// Create analyzer
			llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

			// Analyze and save
			run, err := llmAnalyzer.AnalyzeAndSave(context.Background(), repo, fromSHA, currentSHA, commits)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Analysis failed: %v\n", err)
				fmt.Println("Falling back to git log display.\n")
				// Fall through to show commits
			} else {
				// Display AI-generated summary
				fmt.Printf("AI Analysis Summary (%d commits analyzed):\n", len(commits))
				fmt.Println(strings.Repeat("=", 70))
				fmt.Println()
				fmt.Println(run.Summary.String)
				fmt.Println()
				fmt.Println(strings.Repeat("=", 70))

				// Update repository last run info
				repo.LastRunAt = sql.NullTime{Time: time.Now(), Valid: true}
				repo.LastRunSHA = sql.NullString{String: currentSHA, Valid: true}
				if err := ctx.DB.UpdateRepository(repo); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to update repository: %v\n", err)
				}

				return nil
			}
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
