package cli

import (
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
)

// Context holds common dependencies for CLI commands
type Context struct {
	DB            *db.DB
	Config        *config.Config
	TokenProvider *github.TokenProvider // nil if no GitHub App configured
	Verbose       bool
	Quiet         bool
}
