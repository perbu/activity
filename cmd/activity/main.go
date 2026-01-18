package main

import (
	"flag"
	"fmt"
	"os"

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

	// Parse global flags
	globalFlags := flag.NewFlagSet("activity", flag.ExitOnError)
	configPath := globalFlags.String("config", "", "Config file path (default: ~/.config/activity/config.yaml)")
	dataDir := globalFlags.String("data-dir", "", "Data directory (overrides config file)")
	verbose := globalFlags.Bool("verbose", false, "Verbose output")
	quiet := globalFlags.Bool("quiet", false, "Minimal output")
	globalFlags.BoolVar(verbose, "v", false, "Verbose output (shorthand)")
	globalFlags.BoolVar(quiet, "q", false, "Minimal output (shorthand)")

	// Custom usage
	globalFlags.Usage = func() {
		printUsage()
	}

	// Parse global flags
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	// Check for help or version flags
	if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		printUsage()
		return nil
	}
	if os.Args[1] == "--version" || os.Args[1] == "version" {
		fmt.Printf("activity version %s\n", version)
		return nil
	}

	// Find where the subcommand starts
	subcommandIdx := 1
	for i := 1; i < len(os.Args); i++ {
		if !isFlag(os.Args[i]) {
			subcommandIdx = i
			break
		}
	}

	// Parse global flags before the subcommand
	if err := globalFlags.Parse(os.Args[1:subcommandIdx]); err != nil {
		return err
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
	ctx := &cli.Context{
		DB:      database,
		Config:  cfg,
		Verbose: *verbose,
		Quiet:   *quiet,
	}

	// Get subcommand
	if subcommandIdx >= len(os.Args) {
		printUsage()
		return nil
	}

	subcommand := os.Args[subcommandIdx]
	args := os.Args[subcommandIdx+1:]

	// Route to subcommand
	switch subcommand {
	case "list":
		return cli.List(ctx, args)
	case "show":
		return cli.Show(ctx, args)
	case "update":
		return cli.Update(ctx, args)
	case "repo":
		return cli.Repo(ctx, args)
	case "show-prompts":
		return cli.ShowPrompts(ctx, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		return fmt.Errorf("unknown command: %s", subcommand)
	}
}

func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

func printUsage() {
	fmt.Println(`Activity - Git repository change analyzer

Usage:
  activity [global flags] <command> [command flags] [arguments]

Global Flags:
  --config=<path>     Config file (default: ~/.config/activity/config.yaml)
  --data-dir=<path>   Data directory (required)
  --verbose, -v       Verbose output
  --quiet, -q         Minimal output
  --version           Show version
  --help, -h          Show this help

Commands:
  list                List all repositories
  show [repo...]      Show activity for repositories
  update [repo...]    Update repositories (git pull)
  repo <subcommand>   Manage repositories
  show-prompts        Display current analysis prompts

Repository Management:
  repo add <name> <url> [--branch=main]
                      Add a repository
  repo remove <name> [--keep-files]
                      Remove a repository
  repo activate <name>
                      Activate a repository
  repo deactivate <name>
                      Deactivate a repository
  repo info <name>    Show repository details
  repo list           List all repositories (alias for 'list')

Examples:
  activity repo add myproject https://github.com/user/repo
  activity list
  activity update myproject
  activity show myproject

For more information, visit: https://github.com/perbu/activity
`)
}
