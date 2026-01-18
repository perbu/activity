package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
)

// List lists all repositories
func List(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	format := flags.String("format", "table", "Output format: table or json")
	active := flags.Bool("active", false, "Show only active repositories")
	inactive := flags.Bool("inactive", false, "Show only inactive repositories")

	if err := flags.Parse(args); err != nil {
		return err
	}

	// Determine filter
	var activeFilter *bool
	if *active && !*inactive {
		t := true
		activeFilter = &t
	} else if *inactive && !*active {
		f := false
		activeFilter = &f
	}
	// If both or neither are specified, show all

	// Get repositories
	repos, err := ctx.DB.ListRepositories(activeFilter)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repos) == 0 {
		if !ctx.Quiet {
			fmt.Println("No repositories found")
		}
		return nil
	}

	// Output based on format
	switch *format {
	case "json":
		return outputJSON(repos)
	case "table":
		return outputTable(ctx, repos)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

func outputTable(ctx *Context, repos []*db.Repository) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, "NAME\tSTATUS\tBRANCH\tLAST RUN\tURL")

	// Print rows
	for _, repo := range repos {
		status := "active"
		if !repo.Active {
			status = "inactive"
		}

		lastRun := "never"
		if repo.LastRunAt.Valid {
			// Get current SHA to see if there are new commits
			currentSHA, err := git.GetCurrentSHA(repo.LocalPath)
			if err == nil && repo.LastRunSHA.Valid && currentSHA != repo.LastRunSHA.String {
				lastRun = repo.LastRunAt.Time.Format("2006-01-02 15:04") + " (updates available)"
			} else if err == nil {
				lastRun = repo.LastRunAt.Time.Format("2006-01-02 15:04")
			} else {
				lastRun = repo.LastRunAt.Time.Format("2006-01-02 15:04")
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			repo.Name,
			status,
			repo.Branch,
			lastRun,
			repo.URL,
		)
	}

	return nil
}

// outputJSON outputs repositories as JSON
func outputJSON(repos interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(repos)
}
