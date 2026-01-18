package cli

import (
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
)

// Context holds common dependencies for CLI commands
type Context struct {
	DB      *db.DB
	Config  *config.Config
	Verbose bool
	Quiet   bool
}
