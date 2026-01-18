package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/perbu/activity/internal/git"
)

// Run executes the repo add command
func (c *RepoAddCmd) Run(ctx *Context) error {
	// Check if repo already exists
	_, err := ctx.DB.GetRepositoryByName(c.Name)
	if err == nil {
		return fmt.Errorf("repository '%s' already exists", c.Name)
	}

	// Create local path
	localPath := filepath.Join(ctx.Config.DataDir, c.Name)

	// Check if directory already exists
	if _, err := os.Stat(localPath); err == nil {
		return fmt.Errorf("directory already exists: %s", localPath)
	}

	if !ctx.Quiet {
		fmt.Printf("Cloning %s to %s...\n", c.URL, localPath)
	}

	// Clone repository
	if err := git.Clone(c.URL, localPath, c.Branch); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get current SHA
	sha, err := git.GetCurrentSHA(localPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Create database entry
	repo, err := ctx.DB.CreateRepository(c.Name, c.URL, c.Branch, localPath)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' added successfully\n", c.Name)
		if ctx.Verbose {
			fmt.Printf("  ID: %d\n", repo.ID)
			fmt.Printf("  URL: %s\n", c.URL)
			fmt.Printf("  Branch: %s\n", c.Branch)
			fmt.Printf("  Path: %s\n", localPath)
			fmt.Printf("  Current SHA: %s\n", sha)
		}
	}

	return nil
}

// Run executes the repo remove command
func (c *RepoRemoveCmd) Run(ctx *Context) error {
	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
	}

	// Delete from database
	if err := ctx.DB.DeleteRepository(repo.ID); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	// Delete files if requested
	if !c.KeepFiles {
		if !ctx.Quiet {
			fmt.Printf("Removing files from %s...\n", repo.LocalPath)
		}
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove files: %v\n", err)
		}
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' removed successfully\n", c.Name)
	}

	return nil
}

// Run executes the repo activate command
func (c *RepoActivateCmd) Run(ctx *Context) error {
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
	}

	if repo.Active {
		if !ctx.Quiet {
			fmt.Printf("Repository '%s' is already active\n", c.Name)
		}
		return nil
	}

	if err := ctx.DB.SetRepositoryActive(repo.ID, true); err != nil {
		return fmt.Errorf("failed to activate repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' activated\n", c.Name)
	}

	return nil
}

// Run executes the repo deactivate command
func (c *RepoDeactivateCmd) Run(ctx *Context) error {
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
	}

	if !repo.Active {
		if !ctx.Quiet {
			fmt.Printf("Repository '%s' is already inactive\n", c.Name)
		}
		return nil
	}

	if err := ctx.DB.SetRepositoryActive(repo.ID, false); err != nil {
		return fmt.Errorf("failed to deactivate repository: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' deactivated\n", c.Name)
	}

	return nil
}

// Run executes the repo info command
func (c *RepoInfoCmd) Run(ctx *Context) error {
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
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
