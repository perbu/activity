package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Report handles the report subcommand
func Report(ctx *Context, args []string) error {
	if len(args) == 0 {
		printReportUsage()
		return fmt.Errorf("requires a subcommand")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "generate":
		return reportGenerate(ctx, subArgs)
	case "show":
		return reportShow(ctx, subArgs)
	case "list":
		return reportList(ctx, subArgs)
	default:
		printReportUsage()
		return fmt.Errorf("unknown report subcommand: %s", subcommand)
	}
}

func printReportUsage() {
	fmt.Fprintln(os.Stderr, `Usage: activity report <subcommand> [options]

Subcommands:
  generate <repo> [options]   Generate weekly report(s)
  show <repo> [options]       Show stored report
  list <repo|--all> [options] List reports

Generate options:
  --week=2026-W02            Generate report for specific week
  --since=2025-01-01         Backfill all weeks since date
  --force                    Regenerate existing reports

Show options:
  --week=2026-W02            Show report for specific week
  --latest                   Show most recent report (default)

List options:
  --all                      List reports for all repositories
  --year=2026                Filter by year

Examples:
  activity report generate myrepo --week=2026-W02
  activity report generate myrepo --since=2025-12-01
  activity report generate myrepo --since=2025-12-01 --force
  activity report show myrepo --latest
  activity report show myrepo --week=2026-W02
  activity report list myrepo
  activity report list --all --year=2026`)
}

func reportGenerate(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("report generate", flag.ExitOnError)
	week := flags.String("week", "", "Generate report for specific ISO week (e.g., 2026-W02)")
	since := flags.String("since", "", "Backfill all weeks since date (e.g., 2025-01-01)")
	force := flags.Bool("force", false, "Regenerate existing reports")

	if err := flags.Parse(args); err != nil {
		return err
	}

	repoNames := flags.Args()
	if len(repoNames) == 0 {
		return fmt.Errorf("requires a repository name")
	}
	if len(repoNames) > 1 {
		return fmt.Errorf("only one repository can be specified")
	}

	repoName := repoNames[0]

	// Validate flags
	if *week == "" && *since == "" {
		return fmt.Errorf("either --week or --since must be specified")
	}
	if *week != "" && *since != "" {
		return fmt.Errorf("--week and --since are mutually exclusive")
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// Determine weeks to generate
	var weeksToGenerate [][2]int

	if *week != "" {
		// Single week
		year, wk, err := git.ParseISOWeek(*week)
		if err != nil {
			return err
		}
		weeksToGenerate = append(weeksToGenerate, [2]int{year, wk})
	} else {
		// Backfill since date
		sinceTime, err := time.Parse("2006-01-02", *since)
		if err != nil {
			return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", *since)
		}
		weeksToGenerate = git.WeeksInRange(sinceTime, time.Now())
	}

	if !ctx.Quiet {
		fmt.Printf("Generating %d report(s) for %s\n", len(weeksToGenerate), repo.Name)
	}

	// Initialize LLM client
	llmClient, err := llm.NewClient(context.Background(), ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	defer llmClient.Close()

	// Create analyzer
	llmAnalyzer := analyzer.New(llmClient, ctx.DB, ctx.Config)

	// Generate reports
	generated := 0
	skipped := 0
	noCommits := 0

	for _, yw := range weeksToGenerate {
		year, wk := yw[0], yw[1]
		weekStr := git.FormatISOWeek(year, wk)

		// Check if report exists
		exists, err := ctx.DB.WeeklyReportExists(repo.ID, year, wk)
		if err != nil {
			return fmt.Errorf("failed to check existing report: %w", err)
		}

		if exists && !*force {
			if ctx.Verbose {
				fmt.Printf("  %s: skipped (already exists)\n", weekStr)
			}
			skipped++
			continue
		}

		// Get commits for this week
		commits, err := git.GetCommitsForWeek(repo.LocalPath, year, wk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error getting commits: %v\n", weekStr, err)
			continue
		}

		if len(commits) == 0 {
			if ctx.Verbose {
				fmt.Printf("  %s: no commits\n", weekStr)
			}
			noCommits++
			continue
		}

		// Generate report
		if !ctx.Quiet {
			fmt.Printf("  %s: analyzing %d commits...\n", weekStr, len(commits))
		}

		report, err := generateWeeklyReport(ctx, llmAnalyzer, repo, year, wk, commits, exists)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error generating report: %v\n", weekStr, err)
			continue
		}

		if ctx.Verbose {
			fmt.Printf("  %s: generated (id=%d, commits=%d)\n", weekStr, report.ID, report.CommitCount)
		}
		generated++
	}

	if !ctx.Quiet {
		fmt.Printf("\nSummary: %d generated, %d skipped, %d weeks with no commits\n",
			generated, skipped, noCommits)
	}

	return nil
}

func generateWeeklyReport(ctx *Context, llmAnalyzer *analyzer.Analyzer, repo *db.Repository,
	year, week int, commits []git.Commit, exists bool) (*db.WeeklyReport, error) {

	weekStart, weekEnd := git.ISOWeekBounds(year, week)

	// Determine SHA range
	var fromSHA, toSHA string
	toSHA = commits[0].SHA
	if len(commits) > 1 {
		fromSHA = commits[len(commits)-1].SHA
	}

	// Analyze commits
	run, err := llmAnalyzer.AnalyzeAndSave(context.Background(), repo, fromSHA, toSHA, commits)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Build metadata
	metadata := buildReportMetadata(commits)
	metadataJSON, _ := json.Marshal(metadata)

	// Create or update report
	if exists {
		// Update existing
		existingReport, err := ctx.DB.GetWeeklyReportByRepoAndWeek(repo.ID, year, week)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing report: %w", err)
		}

		existingReport.Summary = run.Summary
		existingReport.CommitCount = len(commits)
		existingReport.Metadata = sql.NullString{String: string(metadataJSON), Valid: true}
		existingReport.AgentMode = run.AgentMode
		existingReport.ToolUsageStats = run.ToolUsageStats
		existingReport.SourceRunID = sql.NullInt64{Int64: run.ID, Valid: true}

		if err := ctx.DB.UpdateWeeklyReport(existingReport); err != nil {
			return nil, fmt.Errorf("failed to update report: %w", err)
		}

		return existingReport, nil
	}

	// Create new report
	report := &db.WeeklyReport{
		RepoID:         repo.ID,
		Year:           year,
		Week:           week,
		WeekStart:      weekStart,
		WeekEnd:        weekEnd,
		Summary:        run.Summary,
		CommitCount:    len(commits),
		Metadata:       sql.NullString{String: string(metadataJSON), Valid: true},
		AgentMode:      run.AgentMode,
		ToolUsageStats: run.ToolUsageStats,
		SourceRunID:    sql.NullInt64{Int64: run.ID, Valid: true},
	}

	return ctx.DB.CreateWeeklyReport(report)
}

// ReportMetadata contains metadata about a weekly report
type ReportMetadata struct {
	Authors      []string       `json:"authors"`
	CommitSHAs   []string       `json:"commit_shas"`
	AuthorCounts map[string]int `json:"author_counts"`
}

func buildReportMetadata(commits []git.Commit) ReportMetadata {
	authorSet := make(map[string]bool)
	authorCounts := make(map[string]int)
	var shas []string

	for _, c := range commits {
		authorSet[c.Author] = true
		authorCounts[c.Author]++
		shas = append(shas, c.SHA)
	}

	var authors []string
	for a := range authorSet {
		authors = append(authors, a)
	}

	return ReportMetadata{
		Authors:      authors,
		CommitSHAs:   shas,
		AuthorCounts: authorCounts,
	}
}

func reportShow(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("report show", flag.ExitOnError)
	week := flags.String("week", "", "Show report for specific ISO week (e.g., 2026-W02)")
	_ = flags.Bool("latest", false, "Show most recent report (default)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	repoNames := flags.Args()
	if len(repoNames) == 0 {
		return fmt.Errorf("requires a repository name")
	}
	if len(repoNames) > 1 {
		return fmt.Errorf("only one repository can be specified")
	}

	repoName := repoNames[0]

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// Get the report
	var report *db.WeeklyReport

	if *week != "" {
		year, wk, err := git.ParseISOWeek(*week)
		if err != nil {
			return err
		}
		report, err = ctx.DB.GetWeeklyReportByRepoAndWeek(repo.ID, year, wk)
		if err != nil {
			return fmt.Errorf("failed to get report: %w", err)
		}
		if report == nil {
			return fmt.Errorf("no report found for %s week %s", repoName, *week)
		}
	} else {
		// Default to latest
		report, err = ctx.DB.GetLatestWeeklyReport(repo.ID)
		if err != nil {
			return fmt.Errorf("failed to get latest report: %w", err)
		}
		if report == nil {
			return fmt.Errorf("no reports found for %s", repoName)
		}
	}

	// Display the report
	displayReport(ctx, repo, report)
	return nil
}

func displayReport(_ *Context, repo *db.Repository, report *db.WeeklyReport) {
	weekStr := git.FormatISOWeek(report.Year, report.Week)

	fmt.Printf("Repository: %s\n", repo.Name)
	fmt.Printf("Week: %s (%s to %s)\n", weekStr,
		report.WeekStart.Format("2006-01-02"),
		report.WeekEnd.Format("2006-01-02"))
	fmt.Printf("Commits: %d\n", report.CommitCount)

	// Parse and show metadata
	if report.Metadata.Valid && report.Metadata.String != "" {
		var metadata ReportMetadata
		if err := json.Unmarshal([]byte(report.Metadata.String), &metadata); err == nil {
			if len(metadata.Authors) > 0 {
				fmt.Printf("Authors: %s\n", strings.Join(metadata.Authors, ", "))
			}
		}
	}

	if report.AgentMode {
		fmt.Printf("Analysis: Agent-based\n")
	} else {
		fmt.Printf("Analysis: Simple\n")
	}
	fmt.Printf("Generated: %s\n", report.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Display summary
	if report.Summary.Valid && report.Summary.String != "" {
		fmt.Println(strings.Repeat("=", 70))
		fmt.Println()
		fmt.Println(report.Summary.String)
		fmt.Println()
		fmt.Println(strings.Repeat("=", 70))
	} else {
		fmt.Println("(No summary available)")
	}
}

func reportList(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("report list", flag.ExitOnError)
	all := flags.Bool("all", false, "List reports for all repositories")
	year := flags.Int("year", 0, "Filter by year")

	if err := flags.Parse(args); err != nil {
		return err
	}

	repoNames := flags.Args()

	var yearFilter *int
	if *year > 0 {
		yearFilter = year
	}

	if *all {
		return listAllReports(ctx, yearFilter)
	}

	if len(repoNames) == 0 {
		return fmt.Errorf("requires a repository name or --all flag")
	}
	if len(repoNames) > 1 {
		return fmt.Errorf("only one repository can be specified")
	}

	repoName := repoNames[0]

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// List reports
	reports, err := ctx.DB.ListWeeklyReportsByRepo(repo.ID, yearFilter)
	if err != nil {
		return fmt.Errorf("failed to list reports: %w", err)
	}

	if len(reports) == 0 {
		fmt.Printf("No reports found for %s\n", repoName)
		return nil
	}

	fmt.Printf("Reports for %s:\n\n", repoName)
	printReportTable(reports)
	return nil
}

func listAllReports(ctx *Context, yearFilter *int) error {
	reports, err := ctx.DB.ListAllWeeklyReports(yearFilter)
	if err != nil {
		return fmt.Errorf("failed to list reports: %w", err)
	}

	if len(reports) == 0 {
		fmt.Println("No reports found")
		return nil
	}

	// Group by repo
	repoReports := make(map[int64][]*db.WeeklyReport)
	for _, r := range reports {
		repoReports[r.RepoID] = append(repoReports[r.RepoID], r)
	}

	// Get repo names
	repos, err := ctx.DB.ListRepositories(nil)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}
	repoNames := make(map[int64]string)
	for _, r := range repos {
		repoNames[r.ID] = r.Name
	}

	fmt.Println("All Weekly Reports:")
	fmt.Println()

	for repoID, reps := range repoReports {
		repoName := repoNames[repoID]
		if repoName == "" {
			repoName = fmt.Sprintf("(repo %d)", repoID)
		}
		fmt.Printf("Repository: %s\n", repoName)
		printReportTable(reps)
		fmt.Println()
	}

	return nil
}

func printReportTable(reports []*db.WeeklyReport) {
	fmt.Printf("  %-10s  %-7s  %-10s  %s\n", "Week", "Commits", "Generated", "Summary Preview")
	fmt.Printf("  %-10s  %-7s  %-10s  %s\n", "----", "-------", "---------", "--------------")

	for _, r := range reports {
		weekStr := git.FormatISOWeek(r.Year, r.Week)
		generated := r.CreatedAt.Format("2006-01-02")

		preview := ""
		if r.Summary.Valid && r.Summary.String != "" {
			preview = r.Summary.String
			// Get first line and truncate
			if idx := strings.Index(preview, "\n"); idx > 0 {
				preview = preview[:idx]
			}
			if len(preview) > 40 {
				preview = preview[:37] + "..."
			}
		}

		fmt.Printf("  %-10s  %7d  %-10s  %s\n", weekStr, r.CommitCount, generated, preview)
	}
}
