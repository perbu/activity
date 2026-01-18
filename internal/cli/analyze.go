package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Analyze analyzes activity for repositories
func Analyze(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("analyze", flag.ExitOnError)
	since := flags.String("since", "", "Analyze commits since this date (e.g., 2024-01-01, '1 week ago')")
	until := flags.String("until", "", "Analyze commits until this date")
	n := flags.Int("n", 0, "Analyze last N commits")
	limit := flags.Int("limit", 10, "Maximum number of commits to display in fallback mode")

	if err := flags.Parse(args); err != nil {
		return err
	}

	repoNames := flags.Args()
	if len(repoNames) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: activity analyze <repo...> [--since=<date>] [--until=<date>] [-n=<count>]\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  activity analyze myrepo --since '1 week ago'\n")
		fmt.Fprintf(os.Stderr, "  activity analyze myrepo --since 2024-01-01 --until 2024-01-31\n")
		fmt.Fprintf(os.Stderr, "  activity analyze myrepo -n 5\n")
		return fmt.Errorf("requires at least one repository name")
	}

	// Validate flags: at least one of --since, --until, or -n must be provided
	if *since == "" && *until == "" && *n == 0 {
		return fmt.Errorf("at least one of --since, --until, or -n must be provided")
	}

	// Validate flags: -n is mutually exclusive with date flags
	if *n > 0 && (*since != "" || *until != "") {
		return fmt.Errorf("-n cannot be used with --since or --until")
	}

	for i, name := range repoNames {
		if i > 0 {
			fmt.Println()
		}

		if err := analyzeRepository(ctx, name, *since, *until, *n, *limit); err != nil {
			fmt.Fprintf(os.Stderr, "Error analyzing %s: %v\n", name, err)
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
	fmt.Println()

	// Check if there are commits to analyze
	if len(commits) == 0 {
		fmt.Println("No commits found in the specified range")
		return nil
	}

	// Determine SHA range for analyzer
	var fromSHA, toSHA string
	toSHA = commits[0].SHA                    // Most recent commit
	if len(commits) > 1 {
		fromSHA = commits[len(commits)-1].SHA // Oldest commit (exclusive in git range)
	}

	// Check if we should use AI analysis
	// Initialize LLM client
	llmClient, err := llm.NewClient(context.Background(), ctx.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize AI analysis: %v\n", err)
		fmt.Println("Falling back to git log display.")
		// Fall through to show commits
	} else {
		defer llmClient.Close()

		// Create analyzer
		llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

		// Analyze and save
		run, err := llmAnalyzer.AnalyzeAndSave(context.Background(), repo, fromSHA, toSHA, commits)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Analysis failed: %v\n", err)
			fmt.Println("Falling back to git log display.")
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
