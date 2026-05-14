package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/perbu/activity/internal/api"
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/github"
	"github.com/perbu/activity/internal/progress"
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

	base := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	handler := progress.NewHandler(base)
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

	// Require data directory for git repository storage
	if cfg.DataDir == "" {
		return fmt.Errorf("data directory must be specified via --data-dir flag or config file (used for git repository storage)")
	}

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return err
	}

	// Require database DSN to be specified
	dsn := cfg.GetDatabaseDSN()
	if dsn == "" {
		return fmt.Errorf("database DSN must be specified via config file or DATABASE_URL environment variable")
	}

	// Open database
	database, err := db.Open(db.OpenConfig{
		DSN:                    dsn,
		MaxOpenConns:           cfg.Database.MaxOpenConns,
		MaxIdleConns:           cfg.Database.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.Database.ConnMaxLifetimeSeconds,
	})
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

	services := service.New(database, cfg, tokenProvider)

	// Single signal-driven context shared by both servers. If either server
	// exits (cleanly or with an error) cancellation propagates so the other
	// shuts down too.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var apiErrCh chan error
	if cfg.API.Enabled {
		apiSrv, err := api.NewServer(cfg, services)
		if err != nil {
			return fmt.Errorf("failed to create API server: %w", err)
		}
		apiErrCh = make(chan error, 1)
		go func() {
			err := apiSrv.Run(ctx)
			if err != nil {
				slog.Error("API server error", "error", err)
			}
			apiErrCh <- err
			stop() // bring the web server down too
		}()
	}

	server, err := web.NewServer(database, services, cfg, *host, *port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	slog.Info("Starting web server", "address", server.Address())
	webErr := server.Start(ctx)

	stop()
	if apiErrCh != nil {
		<-apiErrCh
	}
	return webErr
}
