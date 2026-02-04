package forge

import (
	"fmt"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
)

// New creates a Forge client for the given repository.
// Returns nil if the repo has no forge configured.
func New(repo *db.Repository, cfg *config.Config, tokenProvider *github.TokenProvider) (Forge, error) {
	if !repo.ForgeType.Valid || repo.ForgeType.String == "" {
		return nil, nil
	}

	switch repo.ForgeType.String {
	case "github":
		if !repo.ForgeOwner.Valid || !repo.ForgeRepo.Valid {
			return nil, fmt.Errorf("github forge requires owner and repo")
		}
		return NewGitHub(repo.ForgeOwner.String, repo.ForgeRepo.String, tokenProvider)

	case "forgejo":
		if !repo.ForgeOwner.Valid || !repo.ForgeRepo.Valid {
			return nil, fmt.Errorf("forgejo forge requires instance name (owner) and repo")
		}
		// For Forgejo, ForgeOwner stores the instance name (e.g., "codeberg")
		// and ForgeRepo stores the actual owner/repo path (e.g., "owner/repo")
		instanceName := repo.ForgeOwner.String
		forgejoCfg := cfg.GetForgejoConfig(instanceName)
		if forgejoCfg == nil {
			return nil, fmt.Errorf("no forgejo config for instance %q", instanceName)
		}

		// Parse owner/repo from ForgeRepo
		// ForgeRepo should be in format "owner/repo" for Forgejo
		repoPath := repo.ForgeRepo.String
		var owner, repoName string
		for i, c := range repoPath {
			if c == '/' {
				owner = repoPath[:i]
				repoName = repoPath[i+1:]
				break
			}
		}
		if owner == "" || repoName == "" {
			return nil, fmt.Errorf("forgejo forge_repo must be in owner/repo format, got %q", repoPath)
		}

		token := cfg.GetForgejoToken(instanceName)
		return NewForgejo(forgejoCfg.BaseURL, owner, repoName, token)

	default:
		return nil, fmt.Errorf("unknown forge type: %s", repo.ForgeType.String)
	}
}
