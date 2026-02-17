package forge

import (
	"fmt"
	"net/url"

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
			return nil, fmt.Errorf("forgejo forge requires owner and repo")
		}
		// Derive base URL from the repo clone URL host
		parsed, err := url.Parse(repo.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse repo URL: %w", err)
		}
		baseURL := parsed.Scheme + "://" + parsed.Host

		token := cfg.GetForgejoTokenByHost(parsed.Host)
		return NewForgejo(baseURL, repo.ForgeOwner.String, repo.ForgeRepo.String, token)

	default:
		return nil, fmt.Errorf("unknown forge type: %s", repo.ForgeType.String)
	}
}
