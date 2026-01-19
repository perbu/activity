package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/llm"
)

// Run executes the report generate command
func (c *ReportGenerateCmd) Run(ctx *Context) error {
	// Validate flags
	if c.Week == "" && c.Since == "" {
		return fmt.Errorf("either --week or --since must be specified")
	}
	if c.Week != "" && c.Since != "" {
		return fmt.Errorf("--week and --since are mutually exclusive")
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Repo)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Repo)
	}

	// Determine weeks to generate
	var weeksToGenerate [][2]int

	if c.Week != "" {
		// Single week
		year, wk, err := git.ParseISOWeek(c.Week)
		if err != nil {
			return err
		}
		weeksToGenerate = append(weeksToGenerate, [2]int{year, wk})
	} else {
		// Backfill since date
		sinceTime, err := time.Parse("2006-01-02", c.Since)
		if err != nil {
			return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", c.Since)
		}
		weeksToGenerate = git.WeeksInRange(sinceTime, time.Now())
	}

	if !ctx.Quiet {
		slog.Info("Generating reports", "count", len(weeksToGenerate), "repo", repo.Name)
	}

	// Fetch all remote branches to ensure we have the latest branch refs
	if !ctx.Quiet {
		slog.Debug("Fetching all remote branches", "repo", repo.Name)
	}
	if repo.Private {
		if ctx.TokenProvider == nil {
			return fmt.Errorf("repository '%s' is private but no GitHub App is configured", repo.Name)
		}
		token, err := ctx.TokenProvider.GetToken()
		if err != nil {
			return fmt.Errorf("failed to get GitHub token: %w", err)
		}
		if err := git.FetchAllWithAuth(repo.LocalPath, repo.URL, token); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
	} else {
		if err := git.FetchAll(repo.LocalPath); err != nil {
			slog.Warn("Failed to fetch all branches", "error", err)
		}
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

		if exists && !c.Force {
			if ctx.Verbose {
				slog.Debug("Report skipped, already exists", "week", weekStr)
			}
			skipped++
			continue
		}

		// Get commits for this week
		commits, err := git.GetCommitsForWeek(repo.LocalPath, year, wk)
		if err != nil {
			slog.Error("Failed to get commits", "week", weekStr, "error", err)
			continue
		}

		if len(commits) == 0 {
			if ctx.Verbose {
				slog.Debug("No commits in week", "week", weekStr)
			}
			noCommits++
			continue
		}

		// Get feature branch activity for this week
		branchActivity, err := git.GetFeatureBranchActivity(repo.LocalPath, repo.Branch, year, wk)
		if err != nil {
			slog.Warn("Failed to get branch activity", "week", weekStr, "error", err)
			// Continue without branch activity
			branchActivity = nil
		}

		// Generate report
		if !ctx.Quiet {
			slog.Info("Analyzing commits", "week", weekStr, "commits", len(commits), "branches", len(branchActivity))
		}

		report, err := generateWeeklyReport(ctx, llmAnalyzer, repo, year, wk, commits, branchActivity, exists)
		if err != nil {
			slog.Error("Failed to generate report", "week", weekStr, "error", err)
			continue
		}

		if ctx.Verbose {
			slog.Debug("Report generated", "week", weekStr, "id", report.ID, "commits", report.CommitCount)
		}
		generated++
	}

	if !ctx.Quiet {
		slog.Info("Report generation complete", "generated", generated, "skipped", skipped, "no_commits", noCommits)
	}

	return nil
}

func generateWeeklyReport(ctx *Context, llmAnalyzer *analyzer.Analyzer, repo *db.Repository,
	year, week int, commits []git.Commit, branchActivity []git.BranchActivity, exists bool) (*db.WeeklyReport, error) {

	weekStart, weekEnd := git.ISOWeekBounds(year, week)

	// Determine SHA range
	var fromSHA, toSHA string
	toSHA = commits[0].SHA
	if len(commits) > 1 {
		fromSHA = commits[len(commits)-1].SHA
	}

	// Analyze commits
	run, err := llmAnalyzer.AnalyzeAndSave(context.Background(), repo, fromSHA, toSHA, commits, branchActivity)
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

// Run executes the report show command
func (c *ReportShowCmd) Run(ctx *Context) error {
	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Repo)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Repo)
	}

	// Get the report
	var report *db.WeeklyReport

	if c.Week != "" {
		year, wk, err := git.ParseISOWeek(c.Week)
		if err != nil {
			return err
		}
		report, err = ctx.DB.GetWeeklyReportByRepoAndWeek(repo.ID, year, wk)
		if err != nil {
			return fmt.Errorf("failed to get report: %w", err)
		}
		if report == nil {
			return fmt.Errorf("no report found for %s week %s", c.Repo, c.Week)
		}
	} else {
		// Default to latest
		report, err = ctx.DB.GetLatestWeeklyReport(repo.ID)
		if err != nil {
			return fmt.Errorf("failed to get latest report: %w", err)
		}
		if report == nil {
			return fmt.Errorf("no reports found for %s", c.Repo)
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

// Run executes the report list command
func (c *ReportListCmd) Run(ctx *Context) error {
	var yearFilter *int
	if c.Year > 0 {
		yearFilter = &c.Year
	}

	if c.All {
		return listAllReports(ctx, yearFilter)
	}

	if c.Repo == "" {
		return fmt.Errorf("requires a repository name or --all flag")
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Repo)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Repo)
	}

	// List reports
	reports, err := ctx.DB.ListWeeklyReportsByRepo(repo.ID, yearFilter)
	if err != nil {
		return fmt.Errorf("failed to list reports: %w", err)
	}

	if len(reports) == 0 {
		fmt.Printf("No reports found for %s\n", c.Repo)
		return nil
	}

	fmt.Printf("Reports for %s:\n\n", c.Repo)
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
