package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
	"github.com/perbu/activity/internal/service"
	"github.com/perbu/activity/internal/web"
)

//go:embed .version
var version string

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

	// Parse command-line flags
	var (
		port       = flag.Int("port", 8080, "Port to listen on")
		host       = flag.String("host", "localhost", "Host to bind to")
		configPath = flag.String("config", "", "Config file path")
		dataDir    = flag.String("data-dir", "", "Data directory")
		debug      = flag.Bool("debug", false, "Enable debug logging")
		showVer    = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *showVer {
		fmt.Println(strings.TrimSpace(version))
		return nil
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override data dir if specified
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}

	// Override debug if specified via CLI flag
	if *debug {
		cfg.Debug = true
	}

	// Set up slog based on debug setting
	setupLogger(cfg.Debug)
	slog.Info("starting activity", "version", strings.TrimSpace(version))

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
		tokenProvider, err = github.NewTokenProvider(cfg.GetGitHubAppID(), cfg.GetGitHubInstallationID(), privateKey)
		if err != nil {
			return fmt.Errorf("failed to create GitHub token provider: %w", err)
		}
	}

	// Create services
	services := service.New(database, cfg, tokenProvider)

	// Create and start web server
	server, err := web.NewServer(database, services, cfg, *host, *port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	slog.Info("Starting web server", "address", server.Address())
	return server.Start()
}
