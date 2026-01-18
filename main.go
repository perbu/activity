package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
	"github.com/perbu/activity/internal/cli"
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load .env file if present (ignore errors if file doesn't exist)
	_ = godotenv.Load()

	var cliArgs cli.CLI
	kongCtx := kong.Parse(&cliArgs,
		kong.Name("activity"),
		kong.Description("Git repository activity analyzer"),
		kong.UsageOnError(),
		kong.Vars{
			"version": version,
		},
	)

	// Load configuration
	cfg, err := config.Load(cliArgs.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override data dir if specified
	if cliArgs.DataDir != "" {
		cfg.DataDir = cliArgs.DataDir
	}

	// Require data directory to be specified
	if cfg.DataDir == "" {
		return fmt.Errorf("data directory must be specified via --data-dir flag or config file")
	}

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return err
	}

	// Open database
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Create context
	appCtx := &cli.Context{
		DB:      database,
		Config:  cfg,
		Verbose: cliArgs.Verbose,
		Quiet:   cliArgs.Quiet,
	}

	// Run the selected command
	return kongCtx.Run(appCtx)
}
