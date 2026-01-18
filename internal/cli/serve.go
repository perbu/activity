package cli

import (
	"fmt"

	"github.com/perbu/activity/internal/web"
)

// Run executes the serve command
func (c *ServeCmd) Run(ctx *Context) error {
	server, err := web.NewServer(ctx.DB, c.Host, c.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Starting web server at %s\n", server.Address())
		fmt.Printf("Press Ctrl+C to stop\n")
	}

	return server.Start()
}
