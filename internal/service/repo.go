package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/github"
	"github.com/perbu/activity/internal/llm"
)

// RepoService handles repository management operations
type RepoService struct {
	db            *db.DB
	cfg           *config.Config
	tokenProvider *github.TokenProvider
}

// NewRepoService creates a new RepoService
func NewRepoService(database *db.DB, cfg *config.Config, tokenProvider *github.TokenProvider) *RepoService {
	return &RepoService{
		db:            database,
		cfg:           cfg,
		tokenProvider: tokenProvider,
	}
}

// AddOptions contains options for adding a repository
type AddOptions struct {
	Name    string
	URL     string
	Branch  string
	Private bool
}

// Add creates a new tracked repository
func (s *RepoService) Add(ctx context.Context, opts AddOptions) (*db.Repository, error) {
	// Check if repo already exists
	_, err := s.db.GetRepositoryByName(opts.Name)
	if err == nil {
		return nil, fmt.Errorf("repository '%s' already exists", opts.Name)
	}

	// Validate private flag requires GitHub App configuration
	if opts.Private && s.tokenProvider == nil {
		return nil, fmt.Errorf("private repositories require GitHub App configuration")
	}

	// Default branch
	if opts.Branch == "" {
		opts.Branch = "main"
	}

	// Create local path
	localPath := filepath.Join(s.cfg.DataDir, opts.Name)

	// Check if directory already exists
	if _, err := os.Stat(localPath); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", localPath)
	}

	slog.Info("Cloning repository", "url", opts.URL, "path", localPath, "private", opts.Private)

	// Clone repository (with auth if private)
	if opts.Private {
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.CloneWithAuth(opts.URL, localPath, opts.Branch, token); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		if err := git.Clone(opts.URL, localPath, opts.Branch); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// Generate description from README
	var description sql.NullString
	slog.Info("Generating description from README")
	desc, err := s.generateDescription(ctx, localPath)
	if err != nil {
		slog.Warn("Could not generate description", "error", err)
	} else if desc != "" {
		description = sql.NullString{String: desc, Valid: true}
	}

	// Create database entry
	repo, err := s.db.CreateRepository(opts.Name, opts.URL, opts.Branch, localPath, opts.Private, description)
	if err != nil {
		// Clean up cloned directory on failure
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	slog.Info("Repository added", "name", opts.Name, "id", repo.ID)
	return repo, nil
}

// Remove deletes a repository
func (s *RepoService) Remove(name string, keepFiles bool) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if err := s.db.DeleteRepository(repo.ID); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	if !keepFiles {
		slog.Info("Removing repository files", "path", repo.LocalPath)
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			slog.Warn("Failed to remove files", "path", repo.LocalPath, "error", err)
		}
	}

	slog.Info("Repository removed", "name", name)
	return nil
}

// Activate enables a repository for analysis
func (s *RepoService) Activate(name string) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if repo.Active {
		return nil // Already active
	}

	if err := s.db.SetRepositoryActive(repo.ID, true); err != nil {
		return fmt.Errorf("failed to activate repository: %w", err)
	}

	slog.Info("Repository activated", "name", name)
	return nil
}

// Deactivate disables a repository for analysis
func (s *RepoService) Deactivate(name string) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	if !repo.Active {
		return nil // Already inactive
	}

	if err := s.db.SetRepositoryActive(repo.ID, false); err != nil {
		return fmt.Errorf("failed to deactivate repository: %w", err)
	}

	slog.Info("Repository deactivated", "name", name)
	return nil
}

// SetURL updates the remote URL for a repository
func (s *RepoService) SetURL(name, newURL string) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s", name)
	}

	oldURL := repo.URL

	// Update git remote
	if err := git.SetRemoteURL(repo.LocalPath, newURL); err != nil {
		return fmt.Errorf("failed to update git remote: %w", err)
	}

	// Update database
	repo.URL = newURL
	if err := s.db.UpdateRepository(repo); err != nil {
		// Try to rollback git remote on DB failure
		_ = git.SetRemoteURL(repo.LocalPath, oldURL)
		return fmt.Errorf("failed to update database: %w", err)
	}

	slog.Info("Repository URL updated", "name", name, "old_url", oldURL, "new_url", newURL)
	return nil
}

// UpdateResult contains the result of updating a repository
type UpdateResult struct {
	Name          string
	BeforeSHA     string
	AfterSHA      string
	CommitCount   int
	AlreadyUpToDate bool
}

// Update pulls the latest changes from a repository
func (s *RepoService) Update(ctx context.Context, name string) (*UpdateResult, error) {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %s", name)
	}

	slog.Info("Updating repository", "name", name)

	// Get current SHA before pull
	beforeSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Fetch all branches and pull updates (with auth if private)
	if repo.Private {
		if s.tokenProvider == nil {
			return nil, fmt.Errorf("repository '%s' is private but no GitHub App is configured", name)
		}
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.FetchAllWithAuth(repo.LocalPath, repo.URL, token); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
		if err := git.PullWithAuth(repo.LocalPath, repo.URL, token); err != nil {
			return nil, fmt.Errorf("failed to pull: %w", err)
		}
	} else {
		if err := git.FetchAll(repo.LocalPath); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
		if err := git.Pull(repo.LocalPath); err != nil {
			return nil, fmt.Errorf("failed to pull: %w", err)
		}
	}

	// Get SHA after pull
	afterSHA, err := git.GetCurrentSHA(repo.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated SHA: %w", err)
	}

	// Update repository timestamp
	repo.UpdatedAt = time.Now()
	if err := s.db.UpdateRepository(repo); err != nil {
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	result := &UpdateResult{
		Name:      name,
		BeforeSHA: beforeSHA,
		AfterSHA:  afterSHA,
	}

	if beforeSHA == afterSHA {
		result.AlreadyUpToDate = true
		slog.Info("Repository already up to date", "name", name)
	} else {
		commits, err := git.GetCommitRange(repo.LocalPath, beforeSHA, afterSHA)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit range: %w", err)
		}
		result.CommitCount = len(commits)
		slog.Info("Repository updated", "name", name, "commits", len(commits))
	}

	return result, nil
}

// UpdateAll updates all active repositories
func (s *RepoService) UpdateAll(ctx context.Context) ([]*UpdateResult, error) {
	activeOnly := true
	repos, err := s.db.ListRepositories(&activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	var results []*UpdateResult
	for _, repo := range repos {
		result, err := s.Update(ctx, repo.Name)
		if err != nil {
			slog.Error("Failed to update repository", "name", repo.Name, "error", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// List returns all repositories
func (s *RepoService) List(activeOnly *bool) ([]*db.Repository, error) {
	return s.db.ListRepositories(activeOnly)
}

// Get returns a repository by name
func (s *RepoService) Get(name string) (*db.Repository, error) {
	return s.db.GetRepositoryByName(name)
}

// GetByID returns a repository by ID
func (s *RepoService) GetByID(id int64) (*db.Repository, error) {
	return s.db.GetRepository(id)
}

// generateDescription reads the README and uses LLM to generate a project description
func (s *RepoService) generateDescription(ctx context.Context, repoPath string) (string, error) {
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
	llmClient, err := llm.NewClient(ctx, s.cfg)
	if err != nil {
		return "", fmt.Errorf("failed to initialize LLM: %w", err)
	}
	defer llmClient.Close()

	// Generate description using prompt
	prompt := fmt.Sprintf(config.DefaultDescriptionPrompt, readmeContent)
	description, err := llmClient.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	return strings.TrimSpace(description), nil
}

// findAndReadREADME looks for README files in the repository and returns the content
func findAndReadREADME(repoPath string) (string, error) {
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
