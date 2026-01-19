package cli

import "github.com/alecthomas/kong"

// CLI is the root command structure for kong
type CLI struct {
	Config  string           `short:"c" help:"Config file path" type:"path"`
	DataDir string           `short:"d" name:"data-dir" help:"Data directory" type:"path"`
	Verbose bool             `short:"v" help:"Verbose output"`
	Quiet   bool             `short:"q" help:"Minimal output"`
	Debug   bool             `help:"Enable debug logging"`
	Version kong.VersionFlag `short:"V" help:"Show version"`

	List        ListCmd        `cmd:"" help:"List tracked repositories"`
	Analyze     AnalyzeCmd     `cmd:"" help:"Analyze repository commits"`
	Update      UpdateCmd      `cmd:"" help:"Update repositories"`
	Repo        RepoCmd        `cmd:"" help:"Manage repositories"`
	ShowPrompts ShowPromptsCmd `cmd:"" name:"show-prompts" help:"Show LLM prompts"`
	Report      ReportCmd      `cmd:"" help:"Generate and view weekly reports"`
	Newsletter  NewsletterCmd  `cmd:"" help:"Manage newsletter subscribers"`
	Serve       ServeCmd       `cmd:"" help:"Start the web server"`
}

// ServeCmd starts the web server for browsing reports
type ServeCmd struct {
	Port int    `short:"p" help:"Port to listen on" default:"8080"`
	Host string `help:"Host to bind to" default:"localhost"`
}

// ListCmd lists all repositories
type ListCmd struct {
	Format   string `help:"Output format" enum:"table,json" default:"table"`
	Active   bool   `help:"Show only active repositories"`
	Inactive bool   `help:"Show only inactive repositories"`
}

// AnalyzeCmd analyzes repository commits
type AnalyzeCmd struct {
	Repos []string `arg:"" name:"repo" help:"Repository name(s)" required:""`
	Since string   `help:"Start date (YYYY-MM-DD or relative like '1 week ago')"`
	Until string   `help:"End date (YYYY-MM-DD)"`
	N     int      `short:"n" help:"Number of recent commits"`
	Limit int      `help:"Maximum commits to display in fallback mode" default:"10"`
}

// UpdateCmd updates repositories
type UpdateCmd struct {
	Repos   []string `arg:"" optional:"" name:"repo" help:"Repository name(s)"`
	All     bool     `help:"Update all active repositories"`
	Analyze bool     `help:"Analyze new commits with AI after updating"`
}

// ShowPromptsCmd displays the current prompts
type ShowPromptsCmd struct {
	Defaults bool `help:"Show default prompts even if custom ones are configured"`
}

// RepoCmd is the parent command for repository management
type RepoCmd struct {
	Add        RepoAddCmd        `cmd:"" help:"Add a repository"`
	Remove     RepoRemoveCmd     `cmd:"" help:"Remove a repository"`
	Activate   RepoActivateCmd   `cmd:"" help:"Activate a repository"`
	Deactivate RepoDeactivateCmd `cmd:"" help:"Deactivate a repository"`
	Info       RepoInfoCmd       `cmd:"" help:"Show repository info"`
	List       RepoListCmd       `cmd:"" help:"List repositories"`
	SetURL     RepoSetURLCmd     `cmd:"" name:"set-url" help:"Update repository URL (when remote moves)"`
	Describe   RepoDescribeCmd   `cmd:"" help:"Generate or show repository description"`
}

// RepoAddCmd adds a new repository
type RepoAddCmd struct {
	Name    string `arg:"" help:"Repository name"`
	URL     string `arg:"" help:"Repository URL"`
	Branch  string `help:"Branch to track" default:"main"`
	Private bool   `help:"Repository requires GitHub App authentication"`
}

// RepoRemoveCmd removes a repository
type RepoRemoveCmd struct {
	Name      string `arg:"" help:"Repository name"`
	KeepFiles bool   `name:"keep-files" help:"Keep cloned files"`
}

// RepoActivateCmd activates a repository
type RepoActivateCmd struct {
	Name string `arg:"" help:"Repository name"`
}

// RepoDeactivateCmd deactivates a repository
type RepoDeactivateCmd struct {
	Name string `arg:"" help:"Repository name"`
}

// RepoInfoCmd shows repository details
type RepoInfoCmd struct {
	Name string `arg:"" help:"Repository name"`
}

// RepoSetURLCmd updates the URL for a repository
type RepoSetURLCmd struct {
	Name string `arg:"" help:"Repository name"`
	URL  string `arg:"" help:"New repository URL"`
}

// RepoDescribeCmd generates or shows a repository description
type RepoDescribeCmd struct {
	Name string `arg:"" help:"Repository name"`
	Show bool   `help:"Only show current description, don't regenerate"`
	Set  string `help:"Manually set the description to this value"`
}

// RepoListCmd lists repositories (alias for list command)
type RepoListCmd struct {
	Format   string `help:"Output format" enum:"table,json" default:"table"`
	Active   bool   `help:"Show only active repositories"`
	Inactive bool   `help:"Show only inactive repositories"`
}

// ReportCmd is the parent command for weekly reports
type ReportCmd struct {
	Generate ReportGenerateCmd `cmd:"" help:"Generate weekly report(s)"`
	Show     ReportShowCmd     `cmd:"" help:"Show stored report"`
	List     ReportListCmd     `cmd:"" help:"List reports"`
}

// ReportGenerateCmd generates weekly reports
type ReportGenerateCmd struct {
	Repo  string `arg:"" optional:"" help:"Repository name"`
	All   bool   `help:"Generate reports for all repositories"`
	Week  string `help:"Generate report for specific ISO week (e.g., 2026-W02)"`
	Since string `help:"Backfill all weeks since date (e.g., 2025-01-01)"`
	Force bool   `help:"Regenerate existing reports"`
}

// ReportShowCmd shows a stored report
type ReportShowCmd struct {
	Repo   string `arg:"" help:"Repository name"`
	Week   string `help:"Show report for specific ISO week (e.g., 2026-W02)"`
	Latest bool   `help:"Show most recent report (default)"`
}

// ReportListCmd lists reports
type ReportListCmd struct {
	Repo string `arg:"" optional:"" help:"Repository name"`
	All  bool   `help:"List reports for all repositories"`
	Year int    `help:"Filter by year"`
}

// NewsletterCmd is the parent command for newsletter management
type NewsletterCmd struct {
	Subscriber  SubscriberCmd            `cmd:"" help:"Manage subscribers"`
	Subscribe   NewsletterSubscribeCmd   `cmd:"" help:"Subscribe to a repository"`
	Unsubscribe NewsletterUnsubscribeCmd `cmd:"" help:"Unsubscribe from a repository"`
	Send        NewsletterSendCmd        `cmd:"" help:"Send newsletters"`
}

// SubscriberCmd is the parent command for subscriber management
type SubscriberCmd struct {
	Add    SubscriberAddCmd    `cmd:"" help:"Add a subscriber"`
	Remove SubscriberRemoveCmd `cmd:"" help:"Remove a subscriber"`
	List   SubscriberListCmd   `cmd:"" help:"List all subscribers"`
}

// SubscriberAddCmd adds a subscriber
type SubscriberAddCmd struct {
	Email string `arg:"" help:"Subscriber email address"`
	All   bool   `help:"Subscribe to all repositories"`
}

// SubscriberRemoveCmd removes a subscriber
type SubscriberRemoveCmd struct {
	Email string `arg:"" help:"Subscriber email address"`
}

// SubscriberListCmd lists subscribers
type SubscriberListCmd struct{}

// NewsletterSubscribeCmd subscribes an email to a repo
type NewsletterSubscribeCmd struct {
	Email string `arg:"" help:"Subscriber email address"`
	Repo  string `arg:"" help:"Repository name"`
}

// NewsletterUnsubscribeCmd unsubscribes an email from a repo
type NewsletterUnsubscribeCmd struct {
	Email string `arg:"" help:"Subscriber email address"`
	Repo  string `arg:"" help:"Repository name"`
}

// NewsletterSendCmd sends newsletters
type NewsletterSendCmd struct {
	DryRun bool   `name:"dry-run" help:"Preview what would be sent without actually sending"`
	Since  string `help:"Send activity since (e.g., 7d, 24h, 1w)" default:"7d"`
}
