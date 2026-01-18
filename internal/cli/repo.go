package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Run executes the repo add command
func (c *RepoAddCmd) Run(ctx *Context) error {
	// Check if repo already exists
	_, err := ctx.DB.GetRepositoryByName(c.Name)
	if err == nil {
		return fmt.Errorf("repository '%s' already exists", c.Name)
	}

	// Validate private flag requires GitHub App configuration
	if c.Private && ctx.TokenProvider == nil {
		return fmt.Errorf("--private requires GitHub App configuration (set github.app_id, github.installation_id, and github.private_key_path in config)")
	}

	// Create local path
	localPath := filepath.Join(ctx.Config.DataDir, c.Name)

	// Check if directory already exists
	if _, err := os.Stat(localPath); err == nil {
		return fmt.Errorf("directory already exists: %s", localPath)
	}

	if !ctx.Quiet {
		fmt.Printf("Cloning %s to %s...\n", c.URL, localPath)
		if c.Private {
			fmt.Println("  (using GitHub App authentication)")
		}
	}

	// Clone repository (with auth if private)
	if c.Private {
		token, err := ctx.TokenProvider.GetToken()
		if err != nil {
			return fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.CloneWithAuth(c.URL, localPath, c.Branch, token); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		if err := git.Clone(c.URL, localPath, c.Branch); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// Get current SHA
	sha, err := git.GetCurrentSHA(localPath)
	if err != nil {
		return fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Generate description from README
	var description sql.NullString
	if !ctx.Quiet {
		fmt.Println("Generating description from README...")
	}
	desc, err := generateDescription(ctx, localPath)
	if err != nil {
		if !ctx.Quiet {
			fmt.Printf("  Note: Could not generate description: %v\n", err)
		}
	} else if desc != "" {
		description = sql.NullString{String: desc, Valid: true}
		if !ctx.Quiet && ctx.Verbose {
			fmt.Printf("  Description: %s\n", desc)
		}
	}

	// Create database entry
	repo, err := ctx.DB.CreateRepository(c.Name, c.URL, c.Branch, localPath, c.Private, description)
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
			fmt.Printf("  Private: %v\n", c.Private)
			fmt.Printf("  Current SHA: %s\n", sha)
			if repo.Description.Valid {
				fmt.Printf("  Description: %s\n", repo.Description.String)
			}
		}
	}

	return nil
}

// generateDescription reads the README and uses LLM to generate a project description
func generateDescription(ctx *Context, repoPath string) (string, error) {
	// Try to find README file
	readmeContent, err := findAndReadREADME(repoPath)
	if err != nil {
		return "", err
	}

	// Truncate if too long (max 4000 chars)
	if len(readmeContent) > 4000 {
		readmeContent = readmeContent[:4000]
	}

	// Create LLM client
	llmClient, err := llm.NewClient(context.Background(), ctx.Config)
	if err != nil {
		return "", fmt.Errorf("failed to initialize LLM: %w", err)
	}
	defer llmClient.Close()

	// Generate description using prompt
	prompt := fmt.Sprintf(config.DefaultDescriptionPrompt, readmeContent)
	description, err := llmClient.GenerateText(context.Background(), prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	return strings.TrimSpace(description), nil
}

// findAndReadREADME looks for README files in the repository and returns the content
func findAndReadREADME(repoPath string) (string, error) {
	// Try common README file names
	readmeNames := []string{
		"README.md",
		"README",
		"readme.md",
		"readme",
		"README.txt",
		"README.rst",
		"Readme.md",
	}

	for _, name := range readmeNames {
		path := filepath.Join(repoPath, name)
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("no README file found")
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
	fmt.Printf("  Private: %v\n", repo.Private)
	if repo.Description.Valid && repo.Description.String != "" {
		fmt.Printf("  Description: %s\n", repo.Description.String)
	}
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

// Run executes the repo set-url command
func (c *RepoSetURLCmd) Run(ctx *Context) error {
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
	}

	oldURL := repo.URL

	// Update git remote
	if err := git.SetRemoteURL(repo.LocalPath, c.URL); err != nil {
		return fmt.Errorf("failed to update git remote: %w", err)
	}

	// Update database
	repo.URL = c.URL
	if err := ctx.DB.UpdateRepository(repo); err != nil {
		// Try to rollback git remote on DB failure
		_ = git.SetRemoteURL(repo.LocalPath, oldURL)
		return fmt.Errorf("failed to update database: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Repository '%s' URL updated\n", c.Name)
		if ctx.Verbose {
			fmt.Printf("  Old URL: %s\n", oldURL)
			fmt.Printf("  New URL: %s\n", c.URL)
		}
	}

	return nil
}

// Run executes the repo describe command
func (c *RepoDescribeCmd) Run(ctx *Context) error {
	repo, err := ctx.DB.GetRepositoryByName(c.Name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Name)
	}

	// If --show flag, just display current description
	if c.Show {
		if repo.Description.Valid && repo.Description.String != "" {
			fmt.Printf("Description for '%s':\n%s\n", repo.Name, repo.Description.String)
		} else {
			fmt.Printf("Repository '%s' has no description\n", repo.Name)
		}
		return nil
	}

	// If --set flag, use the provided description
	if c.Set != "" {
		repo.Description = sql.NullString{String: c.Set, Valid: true}
		if err := ctx.DB.UpdateRepository(repo); err != nil {
			return fmt.Errorf("failed to save description: %w", err)
		}

		if !ctx.Quiet {
			fmt.Printf("Description set for '%s':\n%s\n", repo.Name, c.Set)
		}
		return nil
	}

	// Generate new description from README
	if !ctx.Quiet {
		fmt.Printf("Generating description for '%s'...\n", repo.Name)
	}

	desc, err := generateDescription(ctx, repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to generate description: %w", err)
	}

	if desc == "" {
		if !ctx.Quiet {
			fmt.Println("No description could be generated (no README found or empty)")
		}
		return nil
	}

	// Update repository with new description
	repo.Description = sql.NullString{String: desc, Valid: true}
	if err := ctx.DB.UpdateRepository(repo); err != nil {
		return fmt.Errorf("failed to save description: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Description updated for '%s':\n%s\n", repo.Name, desc)
	}

	return nil
}
