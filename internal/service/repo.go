package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/github"
	"github.com/perbu/activity/internal/llm"
	"github.com/perbu/activity/internal/progress"
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

// repoPath computes the local filesystem path for a repository
func (s *RepoService) repoPath(repoName string) string {
	return db.RepoLocalPath(s.cfg.DataDir, repoName)
}

// ensureRepoReady ensures the repository is cloned and in bare format
// This handles auto-migration from old full checkout format
func (s *RepoService) ensureRepoReady(repo *db.Repository) error {
	repoPath := s.repoPath(repo.Name)

	// Check if repo exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		slog.Info("Repository missing, re-cloning", "name", repo.Name)
		return s.cloneRepo(repo)
	}

	// Check if it's a bare repo
	if !git.IsBareRepo(repoPath) {
		slog.Info("Migrating to bare repo", "name", repo.Name)
		if err := os.RemoveAll(repoPath); err != nil {
			return fmt.Errorf("failed to remove old repo: %w", err)
		}
		return s.cloneRepo(repo)
	}

	return nil
}

// cloneRepo clones a repository as a bare mirror
func (s *RepoService) cloneRepo(repo *db.Repository) error {
	repoPath := s.repoPath(repo.Name)

	if repo.Private {
		if s.tokenProvider == nil {
			return fmt.Errorf("repository '%s' is private but no GitHub App is configured", repo.Name)
		}
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return fmt.Errorf("failed to get GitHub token: %w", err)
		}
		return git.CloneMirrorWithAuth(repo.URL, repoPath, token)
	}
	return git.CloneMirror(repo.URL, repoPath)
}

// AddOptions contains options for adding a repository
type AddOptions struct {
	Name      string
	URL       string
	Branch    string
	Private   bool
	External  bool
	ForgeType string // "github", "forgejo", or "" (auto-detected from URL for github)
}

// UpdateOptions contains options for updating repository settings
type UpdateOptions struct {
	URL         string
	Branch      string
	Private     bool
	External    bool
	ForgeType   string // "github", "forgejo", or ""
	Description string // manual override; empty string clears it
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

	// Compute local path from data dir and repo name
	localPath := s.repoPath(opts.Name)

	// Check if directory already exists
	if _, err := os.Stat(localPath); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", localPath)
	}

	slog.Info("Cloning repository as bare mirror", "url", opts.URL, "path", localPath, "private", opts.Private)

	// Clone repository as bare mirror (with auth if private)
	if opts.Private {
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.CloneMirrorWithAuth(opts.URL, localPath, token); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		if err := git.CloneMirror(opts.URL, localPath); err != nil {
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

	// Derive forge owner/repo from URL
	var forgeType, forgeOwner, forgeRepo sql.NullString
	if opts.ForgeType != "" {
		_, owner, repoName := ParseForgeURL(opts.URL)
		forgeType = sql.NullString{String: opts.ForgeType, Valid: true}
		forgeOwner = sql.NullString{String: owner, Valid: owner != ""}
		forgeRepo = sql.NullString{String: repoName, Valid: repoName != ""}
	}

	// Create database entry
	repo, err := s.db.CreateRepository(opts.Name, opts.URL, opts.Branch, opts.Private, opts.External, description, forgeType, forgeOwner, forgeRepo)
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
		return fmt.Errorf("repository not found: %s: %w", name, err)
	}

	if err := s.db.DeleteRepository(repo.ID); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	if !keepFiles {
		repoPath := s.repoPath(repo.Name)
		slog.Info("Removing repository files", "path", repoPath)
		if err := os.RemoveAll(repoPath); err != nil {
			slog.Warn("Failed to remove files", "path", repoPath, "error", err)
		}
	}

	slog.Info("Repository removed", "name", name)
	return nil
}

// Activate enables a repository for analysis
func (s *RepoService) Activate(name string) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s: %w", name, err)
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
		return fmt.Errorf("repository not found: %s: %w", name, err)
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
		return fmt.Errorf("repository not found: %s: %w", name, err)
	}

	oldURL := repo.URL
	repoPath := s.repoPath(repo.Name)

	// Update git remote
	if err := git.SetRemoteURL(repoPath, newURL); err != nil {
		return fmt.Errorf("failed to update git remote: %w", err)
	}

	// Update database
	repo.URL = newURL
	if err := s.db.UpdateRepository(repo); err != nil {
		// Try to rollback git remote on DB failure
		_ = git.SetRemoteURL(repoPath, oldURL)
		return fmt.Errorf("failed to update database: %w", err)
	}

	slog.Info("Repository URL updated", "name", name, "old_url", oldURL, "new_url", newURL)
	return nil
}

// UpdateSettings updates repository settings (branch, private, external, and optionally URL)
func (s *RepoService) UpdateSettings(name string, opts UpdateOptions) error {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return fmt.Errorf("repository not found: %s: %w", name, err)
	}

	// If URL changed, update git remote
	if opts.URL != "" && opts.URL != repo.URL {
		if err := s.SetURL(name, opts.URL); err != nil {
			return err
		}
		// Re-fetch repo since SetURL updated it
		repo, err = s.db.GetRepositoryByName(name)
		if err != nil {
			return fmt.Errorf("failed to re-fetch repository: %w", err)
		}
	}

	// Update fields
	if opts.Branch != "" {
		repo.Branch = opts.Branch
	}
	repo.Private = opts.Private
	repo.External = opts.External
	repo.Description = sql.NullString{String: opts.Description, Valid: opts.Description != ""}

	// Update forge fields — derive owner/repo from URL
	if opts.ForgeType != "" {
		repoURL := opts.URL
		if repoURL == "" {
			repoURL = repo.URL
		}
		_, owner, repoName := ParseForgeURL(repoURL)
		repo.ForgeType = sql.NullString{String: opts.ForgeType, Valid: true}
		repo.ForgeOwner = sql.NullString{String: owner, Valid: owner != ""}
		repo.ForgeRepo = sql.NullString{String: repoName, Valid: repoName != ""}
	} else {
		// Clear forge fields if type is empty
		repo.ForgeType = sql.NullString{}
		repo.ForgeOwner = sql.NullString{}
		repo.ForgeRepo = sql.NullString{}
	}

	if err := s.db.UpdateRepository(repo); err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	slog.Info("Repository settings updated", "name", name, "branch", repo.Branch, "private", repo.Private, "external", repo.External, "forge", opts.ForgeType)
	return nil
}

// UpdateResult contains the result of updating a repository
type UpdateResult struct {
	Name            string
	BeforeSHA       string
	AfterSHA        string
	CommitCount     int
	AlreadyUpToDate bool
}

// Update fetches the latest changes for a repository
func (s *RepoService) Update(ctx context.Context, name string) (*UpdateResult, error) {
	repo, err := s.db.GetRepositoryByName(name)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %s: %w", name, err)
	}

	repoPath := s.repoPath(repo.Name)

	// Ensure repo is ready (handles migration from old format)
	if err := s.ensureRepoReady(repo); err != nil {
		return nil, fmt.Errorf("failed to ensure repo ready: %w", err)
	}

	progress.Log(ctx, "Updating repository", "name", name)

	// Get current SHA for the tracked branch before fetch
	beforeSHA, err := git.GetBranchSHA(repoPath, repo.Branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get current SHA: %w", err)
	}

	// Fetch updates (with auth if private)
	if repo.Private {
		if s.tokenProvider == nil {
			return nil, fmt.Errorf("repository '%s' is private but no GitHub App is configured", name)
		}
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.FetchWithAuth(repoPath, repo.URL, token); err != nil {
			return nil, fmt.Errorf("failed to fetch: %w", err)
		}
	} else {
		if err := git.Fetch(repoPath); err != nil {
			return nil, fmt.Errorf("failed to fetch: %w", err)
		}
	}

	// Get SHA after fetch for the tracked branch
	afterSHA, err := git.GetBranchSHA(repoPath, repo.Branch)
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
		progress.Log(ctx, "Repository up to date", "name", name)
	} else {
		commits, err := git.GetCommitRange(repoPath, beforeSHA, afterSHA)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit range: %w", err)
		}
		result.CommitCount = len(commits)
		progress.Log(ctx, "Repository updated", "name", name, "commits", len(commits))
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
// Works with bare repositories by using git show to retrieve file content
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
		content, err := git.GetFileContent(repoPath, name)
		if err == nil {
			return content, nil
		}
	}

	return "", fmt.Errorf("no README file found")
}

// ParseForgeURL extracts owner and repo from any HTTPS git clone URL.
// It also auto-detects GitHub from the host. Returns forgeType ("github" or ""),
// owner, and repo name. For non-GitHub URLs, forgeType is empty but owner/repo
// are still extracted if the path has the right shape.
func ParseForgeURL(repoURL string) (forgeType, owner, repo string) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", ""
	}

	// Extract owner/repo from path
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ""
	}

	owner = parts[0]
	repo = parts[1]

	// Auto-detect GitHub from host
	host := strings.ToLower(parsed.Host)
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		return "github", owner, repo
	}

	return "", owner, repo
}
