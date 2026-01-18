package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/perbu/activity/internal/git"
)

// Repo handles the 'repo' subcommand
func Repo(ctx *Context, args []string) error {
	if len(args) == 0 {
		printRepoUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		return repoAdd(ctx, subArgs)
	case "remove":
		return repoRemove(ctx, subArgs)
	case "activate":
		return repoActivate(ctx, subArgs)
	case "deactivate":
		return repoDeactivate(ctx, subArgs)
	case "info":
		return repoInfo(ctx, subArgs)
	case "list":
		return List(ctx, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown repo subcommand: %s\n\n", subcommand)
		printRepoUsage()
		return fmt.Errorf("unknown repo subcommand: %s", subcommand)
	}
}

func repoAdd(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("repo add", flag.ExitOnError)
	branch := flags.String("branch", "main", "Branch to track")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: activity repo add [--branch=main] <name> <url>\n")
		return fmt.Errorf("requires exactly 2 arguments: name and url")
	}

	name := flags.Arg(0)
	url := flags.Arg(1)

	// Check if repo already exists
	_, err := ctx.DB.GetRepositoryByName(name)
	if err == nil {
		return fmt.Errorf("repository '%s' already exists", name)
	}

	// Create local path
	localPath := filepath.Join(ctx.Config.DataDir, name)

	// Check if directory already exists
	if _, err := os.Stat(localPath); err == nil {
		return fmt.Errorf("directory already exists: %s", localPath)
	}

	if !ctx.Quiet {
		fmt.Printf("Cloning %s to %s...\n", url, localPath)
	}

	// Clone repository
	if err := git.Clone(url, localPath, *branch); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get current SHA
	sha, err := git.GetCurrentSHA(localPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Create database entry
	repo, err := ctx.DB.CreateRepository(name, url, *branch, localPath)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' added successfully\n", name)
		if ctx.Verbose {
			fmt.Printf("  ID: %d\n", repo.ID)
			fmt.Printf("  URL: %s\n", url)
			fmt.Printf("  Branch: %s\n", *branch)
			fmt.Printf("  Path: %s\n", localPath)
			fmt.Printf("  Current SHA: %s\n", sha)
		}
	}

	return nil
}

func repoRemove(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("repo remove", flag.ExitOnError)
	keepFiles := flags.Bool("keep-files", false, "Keep cloned files")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity repo remove [--keep-files] <name>\n")
		return fmt.Errorf("requires exactly 1 argument: name")
	}

	name := flags.Arg(0)

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	// Delete from database
	if err := ctx.DB.DeleteRepository(repo.ID); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Delete files if requested
	if !*keepFiles {
		if !ctx.Quiet {
			fmt.Printf("Removing files from %s...\n", repo.LocalPath)
		}
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove files: %v\n", err)
		}
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' removed successfully\n", name)
	}

	return nil
}

func repoActivate(ctx *Context, args []string) error {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity repo activate <name>\n")
		return fmt.Errorf("requires exactly 1 argument: name")
	}

	name := args[0]

	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if repo.Active {
		if !ctx.Quiet {
			fmt.Printf("Repository '%s' is already active\n", name)
		}
		return nil
	}

	if err := ctx.DB.SetRepositoryActive(repo.ID, true); err != nil {
		return fmt.Errorf("failed to activate repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' activated\n", name)
	}

	return nil
}

func repoDeactivate(ctx *Context, args []string) error {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity repo deactivate <name>\n")
		return fmt.Errorf("requires exactly 1 argument: name")
	}

	name := args[0]

	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if !repo.Active {
		if !ctx.Quiet {
			fmt.Printf("Repository '%s' is already inactive\n", name)
		}
		return nil
	}

	if err := ctx.DB.SetRepositoryActive(repo.ID, false); err != nil {
		return fmt.Errorf("failed to deactivate repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' deactivated\n", name)
	}

	return nil
}

func repoInfo(ctx *Context, args []string) error {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity repo info <name>\n")
		return fmt.Errorf("requires exactly 1 argument: name")
	}

	name := args[0]

	repo, err := ctx.DB.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	// Get current SHA
	currentSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		currentSHA = "(error reading SHA)"
	}

	fmt.Printf("Repository: %s\n", repo.Name)
	fmt.Printf("  ID: %d\n", repo.ID)
	fmt.Printf("  URL: %s\n", repo.URL)
	fmt.Printf("  Branch: %s\n", repo.Branch)
	fmt.Printf("  Path: %s\n", repo.LocalPath)
	fmt.Printf("  Active: %v\n", repo.Active)
	fmt.Printf("  Created: %s\n", repo.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", repo.UpdatedAt.Format("2006-01-02 15:04:05"))
	if repo.LastRunAt.Valid {
		fmt.Printf("  Last Run: %s\n", repo.LastRunAt.Time.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("  Last Run: never\n")
	}
	if repo.LastRunSHA.Valid {
		fmt.Printf("  Last Run SHA: %s\n", repo.LastRunSHA.String)
	}
	fmt.Printf("  Current SHA: %s\n", currentSHA)

	return nil
}

func printRepoUsage() {
	fmt.Println(`Repository management commands

Usage:
  activity repo <subcommand> [flags] [arguments]

Subcommands:
  add [--branch=main] <name> <url>
                      Add a repository
  remove [--keep-files] <name>
                      Remove a repository
  activate <name>     Activate a repository
  deactivate <name>   Deactivate a repository
  info <name>         Show repository details
  list                List all repositories

Examples:
  activity repo add myproject https://github.com/user/repo
  activity repo add --branch=develop myproject https://github.com/user/repo
  activity repo remove myproject
  activity repo remove --keep-files myproject
  activity repo info myproject
  activity repo list`)
}
