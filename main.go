package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
	"github.com/perbu/activity/internal/cli"
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
)

var version = "dev"

// setupLogger configures the global slog logger based on debug setting
func setupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

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

	// Override debug if specified via CLI flag
	if cliArgs.Debug {
		cfg.Debug = true
	}

	// Set up slog based on debug setting
	setupLogger(cfg.Debug)

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

	// Initialize GitHub App token provider if configured
	var tokenProvider *github.TokenProvider
	if cfg.HasGitHubApp() {
		privateKey, err := cfg.GetGitHubPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to get GitHub App private key: %w", err)
		}
		tokenProvider, err = github.NewTokenProvider(cfg.GitHub.AppID, cfg.GitHub.InstallationID, privateKey)
		if err != nil {
			return fmt.Errorf("failed to create GitHub token provider: %w", err)
		}
	}

	// Create context
	appCtx := &cli.Context{
		DB:            database,
		Config:        cfg,
		TokenProvider: tokenProvider,
		Verbose:       cliArgs.Verbose,
		Quiet:         cliArgs.Quiet,
	}

	// Run the selected command
	return kongCtx.Run(appCtx)
}
