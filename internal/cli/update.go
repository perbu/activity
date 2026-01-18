package cli

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Update pulls updates for repositories
func Update(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("update", flag.ExitOnError)
	all := flags.Bool("all", false, "Update all active repositories")
	analyze := flags.Bool("analyze", false, "Analyze new commits with AI after updating")

	if err := flags.Parse(args); err != nil {
		return err
	}

	var repoNames []string
	if *all {
		// Get all active repositories
		activeFlag := true
		repos, err := ctx.DB.ListRepositories(&activeFlag)
		if err != nil {
			return fmt.Errorf("failed to list repositories: %w", err)
		}

		if len(repos) == 0 {
			if !ctx.Quiet {
				fmt.Println("No active repositories found")
			}
			return nil
		}

		for _, repo := range repos {
			repoNames = append(repoNames, repo.Name)
		}
	} else {
		// Use specified repositories
		if flags.NArg() == 0 {
			fmt.Fprintf(os.Stderr, "Usage: activity update [repo...] [--all]\n")
			return fmt.Errorf("requires repository names or --all flag")
		}
		repoNames = flags.Args()
	}

	// Update each repository
	for _, name := range repoNames {
		if err := updateRepository(ctx, name, *analyze); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", name, err)
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
		fmt.Printf("Updating %s...\n", name)
	}

	// Get current SHA before pull
	beforeSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Pull updates
	if err := git.Pull(repo.LocalPath); err != nil {
		return fmt.Errorf("failed to pull: %w", err)
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
			fmt.Printf("  %s is already up to date\n", name)
		}
	} else {
		// Get commit range
		commits, err := git.GetCommitRange(repo.LocalPath, beforeSHA, afterSHA)
		if err != nil {
			return fmt.Errorf("failed to get commit range: %w", err)
		}

		if !ctx.Quiet {
			fmt.Printf("  Updated %s: %d new commit(s)\n", name, len(commits))
			if ctx.Verbose {
				for _, commit := range commits {
					fmt.Printf("    %s %s\n", commit.SHA[:7], commit.Message)
				}
			}
		}

		// After successful update, check for AI analysis
		if analyze && len(commits) > 0 {
			if !ctx.Quiet {
				fmt.Printf("  Analyzing %d new commits...\n", len(commits))
			}

			// Initialize LLM and analyzer
			llmClient, err := llm.NewClient(context.Background(), ctx.Config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: Failed to initialize AI: %v\n", err)
				return nil
			}
			defer llmClient.Close()

			llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

			// Analyze and save
			_, err = llmAnalyzer.AnalyzeAndSave(context.Background(), repo, beforeSHA, afterSHA, commits)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: Analysis failed: %v\n", err)
				return nil
			}

			// Update repository
			repo.LastRunAt = sql.NullTime{Time: time.Now(), Valid: true}
			repo.LastRunSHA = sql.NullString{String: afterSHA, Valid: true}
			if err := ctx.DB.UpdateRepository(repo); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: Failed to update repository: %v\n", err)
			}

			if !ctx.Quiet {
				fmt.Printf("  Analysis complete\n")
			}
		}
	}

	return nil
}
