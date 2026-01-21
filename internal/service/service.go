// Package service provides business logic extracted from CLI commands
// for use by web handlers and other consumers.
package service

import (
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
)

// Services is a container for all service instances
type Services struct {
	Repo       *RepoService
	Report     *ReportService
	Newsletter *NewsletterService
	Admin      *AdminService
}

// New creates a new Services container with all dependencies
func New(database *db.DB, cfg *config.Config, tokenProvider *github.TokenProvider) *Services {
	return &Services{
		Repo:       NewRepoService(database, cfg, tokenProvider),
		Report:     NewReportService(database, cfg, tokenProvider),
		Newsletter: NewNewsletterService(database, cfg),
		Admin:      NewAdminService(database, cfg),
	}
}
