package forge

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation/v2"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
)

// New creates a Forge client for the given repository.
// Returns nil if the repo has no forge configured.
func New(repo *db.Repository, cfg *config.Config) (Forge, error) {
	if !repo.ForgeType.Valid || repo.ForgeType.String == "" {
		return nil, nil
	}

	switch repo.ForgeType.String {
	case "github":
		if !repo.ForgeOwner.Valid || !repo.ForgeRepo.Valid {
			return nil, fmt.Errorf("github forge requires owner and repo")
		}
		var httpClient *http.Client
		if cfg.HasGitHubApp() {
			key, err := cfg.GetGitHubPrivateKey()
			if err != nil {
				slog.Warn("GitHub App configured but private key unavailable, using unauthenticated access", "error", err)
			} else {
				itr, err := ghinstallation.New(http.DefaultTransport, cfg.GetGitHubAppID(), cfg.GetGitHubInstallationID(), key)
				if err != nil {
					slog.Warn("Failed to create GitHub App transport, using unauthenticated access", "error", err)
				} else {
					httpClient = &http.Client{Transport: itr}
				}
			}
		}
		return NewGitHub(repo.ForgeOwner.String, repo.ForgeRepo.String, httpClient)

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
