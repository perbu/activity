package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/perbu/activity/internal/analyzer"
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/github"
	"github.com/perbu/activity/internal/llm"
)

// ReportService handles weekly report generation
type ReportService struct {
	db            *db.DB
	cfg           *config.Config
	tokenProvider *github.TokenProvider
}

// NewReportService creates a new ReportService
func NewReportService(database *db.DB, cfg *config.Config, tokenProvider *github.TokenProvider) *ReportService {
	return &ReportService{
		db:            database,
		cfg:           cfg,
		tokenProvider: tokenProvider,
	}
}

// GenerateOptions contains options for report generation
type GenerateOptions struct {
	RepoName string // Repository name (or empty for all active repos)
	Week     string // ISO week string like "2026-W02" (or empty with Since for backfill)
	Since    string // Backfill start date YYYY-MM-DD (or empty with Week for single week)
	Force    bool   // Regenerate existing reports
}

// GenerateResult contains the result of report generation
type GenerateResult struct {
	Generated  int
	Skipped    int
	NoCommits  int
	RepoName   string
	WeekLabel  string
	ReportID   int64
}

// GenerateForWeek generates a report for a specific ISO week
func (s *ReportService) GenerateForWeek(ctx context.Context, repoName string, weekStr string, force bool) (*GenerateResult, error) {
	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %s", repoName)
	}

	year, week, err := git.ParseISOWeek(weekStr)
	if err != nil {
		return nil, err
	}

	// Check if report exists
	exists, err := s.db.WeeklyReportExists(repo.ID, year, week)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing report: %w", err)
	}

	if exists && !force {
		return &GenerateResult{Skipped: 1, RepoName: repoName, WeekLabel: weekStr}, nil
	}

	// Fetch all remote branches
	if err := s.fetchBranches(repo); err != nil {
		slog.Warn("Failed to fetch branches", "error", err)
	}

	// Get commits for this week
	commits, err := git.GetCommitsForWeek(repo.LocalPath, year, week)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits for %s: %w", weekStr, err)
	}

	if len(commits) == 0 {
		return &GenerateResult{NoCommits: 1, RepoName: repoName, WeekLabel: weekStr}, nil
	}

	// Get feature branch activity
	branchActivity, err := git.GetFeatureBranchActivity(repo.LocalPath, repo.Branch, year, week)
	if err != nil {
		slog.Warn("Failed to get branch activity", "week", weekStr, "error", err)
		branchActivity = nil
	}

	slog.Info("Analyzing commits", "week", weekStr, "commits", len(commits), "branches", len(branchActivity))

	// Generate report
	report, err := s.generateWeeklyReport(ctx, repo, year, week, commits, branchActivity, exists)
	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	return &GenerateResult{
		Generated: 1,
		RepoName:  repoName,
		WeekLabel: weekStr,
		ReportID:  report.ID,
	}, nil
}

// GenerateSince generates reports for all weeks since a date
func (s *ReportService) GenerateSince(ctx context.Context, repoName string, sinceDate string, force bool) (*GenerateResult, error) {
	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %s", repoName)
	}

	sinceTime, err := time.Parse("2006-01-02", sinceDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", sinceDate)
	}

	weeksToGenerate := git.WeeksInRange(sinceTime, time.Now())
	slog.Info("Generating reports", "count", len(weeksToGenerate), "repo", repoName)

	// Fetch all remote branches
	if err := s.fetchBranches(repo); err != nil {
		slog.Warn("Failed to fetch branches", "error", err)
	}

	// Initialize LLM client once for all reports
	llmClient, err := llm.NewClient(ctx, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	defer llmClient.Close()

	llmAnalyzer := analyzer.New(llmClient, s.db, s.cfg)

	result := &GenerateResult{RepoName: repoName}

	for _, yw := range weeksToGenerate {
		year, wk := yw[0], yw[1]
		weekStr := git.FormatISOWeek(year, wk)

		// Check if report exists
		exists, err := s.db.WeeklyReportExists(repo.ID, year, wk)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing report: %w", err)
		}

		if exists && !force {
			result.Skipped++
			continue
		}

		// Get commits for this week
		commits, err := git.GetCommitsForWeek(repo.LocalPath, year, wk)
		if err != nil {
			slog.Error("Failed to get commits", "week", weekStr, "error", err)
			continue
		}

		if len(commits) == 0 {
			result.NoCommits++
			continue
		}

		// Get feature branch activity
		branchActivity, err := git.GetFeatureBranchActivity(repo.LocalPath, repo.Branch, year, wk)
		if err != nil {
			slog.Warn("Failed to get branch activity", "week", weekStr, "error", err)
			branchActivity = nil
		}

		slog.Info("Analyzing commits", "week", weekStr, "commits", len(commits), "branches", len(branchActivity))

		// Generate report using shared analyzer
		report, err := s.generateWeeklyReportWithAnalyzer(ctx, llmAnalyzer, repo, year, wk, commits, branchActivity, exists)
		if err != nil {
			slog.Error("Failed to generate report", "week", weekStr, "error", err)
			continue
		}

		result.Generated++
		result.ReportID = report.ID
		result.WeekLabel = weekStr
	}

	return result, nil
}

// GenerateAllReposSince generates reports for all active repos since a date
func (s *ReportService) GenerateAllReposSince(ctx context.Context, sinceDate string, force bool) ([]*GenerateResult, error) {
	activeOnly := true
	repos, err := s.db.ListRepositories(&activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	var results []*GenerateResult
	for _, repo := range repos {
		result, err := s.GenerateSince(ctx, repo.Name, sinceDate, force)
		if err != nil {
			slog.Error("Failed to generate reports", "repo", repo.Name, "error", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GenerateLastWeek generates reports for the previous complete week for all active repos
func (s *ReportService) GenerateLastWeek(ctx context.Context, force bool) ([]*GenerateResult, error) {
	// Calculate the previous complete week
	now := time.Now()
	year, week := now.ISOWeek()

	// Go back to previous week
	week--
	if week < 1 {
		year--
		lastDayOfPrevYear := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
		_, week = lastDayOfPrevYear.ISOWeek()
	}

	weekStr := git.FormatISOWeek(year, week)

	activeOnly := true
	repos, err := s.db.ListRepositories(&activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	var results []*GenerateResult
	for _, repo := range repos {
		result, err := s.GenerateForWeek(ctx, repo.Name, weekStr, force)
		if err != nil {
			slog.Error("Failed to generate report", "repo", repo.Name, "error", err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetReport retrieves a report by ID
func (s *ReportService) GetReport(id int64) (*db.WeeklyReport, error) {
	return s.db.GetWeeklyReport(id)
}

// GetLatestReport retrieves the most recent report for a repository
func (s *ReportService) GetLatestReport(repoName string) (*db.WeeklyReport, error) {
	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %s", repoName)
	}
	return s.db.GetLatestWeeklyReport(repo.ID)
}

// ListReports retrieves reports for a repository
func (s *ReportService) ListReports(repoID int64, year *int) ([]*db.WeeklyReport, error) {
	return s.db.ListWeeklyReportsByRepo(repoID, year)
}

// ListAllReports retrieves all reports
func (s *ReportService) ListAllReports(year *int) ([]*db.WeeklyReport, error) {
	return s.db.ListAllWeeklyReports(year)
}

// fetchBranches fetches all remote branches for a repository
func (s *ReportService) fetchBranches(repo *db.Repository) error {
	if repo.Private {
		if s.tokenProvider == nil {
			return fmt.Errorf("repository '%s' is private but no GitHub App is configured", repo.Name)
		}
		token, err := s.tokenProvider.GetToken()
		if err != nil {
			return fmt.Errorf("failed to get GitHub token: %w", err)
		}
		return git.FetchAllWithAuth(repo.LocalPath, repo.URL, token)
	}
	return git.FetchAll(repo.LocalPath)
}

// generateWeeklyReport generates a report using a new LLM client
func (s *ReportService) generateWeeklyReport(ctx context.Context, repo *db.Repository,
	year, week int, commits []git.Commit, branchActivity []git.BranchActivity, exists bool) (*db.WeeklyReport, error) {

	llmClient, err := llm.NewClient(ctx, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	defer llmClient.Close()

	llmAnalyzer := analyzer.New(llmClient, s.db, s.cfg)
	return s.generateWeeklyReportWithAnalyzer(ctx, llmAnalyzer, repo, year, week, commits, branchActivity, exists)
}

// generateWeeklyReportWithAnalyzer generates a report using an existing analyzer
func (s *ReportService) generateWeeklyReportWithAnalyzer(ctx context.Context, llmAnalyzer *analyzer.Analyzer,
	repo *db.Repository, year, week int, commits []git.Commit, branchActivity []git.BranchActivity, exists bool) (*db.WeeklyReport, error) {

	weekStart, weekEnd := git.ISOWeekBounds(year, week)

	// Determine SHA range
	var fromSHA, toSHA string
	toSHA = commits[0].SHA
	if len(commits) > 1 {
		fromSHA = commits[len(commits)-1].SHA
	}

	// Fetch previous week's report for context
	prevYear, prevWeek := previousWeek(year, week)
	var previousSummary string
	prevReport, err := s.db.GetWeeklyReportByRepoAndWeek(repo.ID, prevYear, prevWeek)
	if err == nil && prevReport != nil && prevReport.Summary.Valid {
		previousSummary = prevReport.Summary.String
	}

	// Analyze commits
	run, err := llmAnalyzer.AnalyzeAndSave(ctx, repo, fromSHA, toSHA, commits, branchActivity, previousSummary)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Build metadata
	metadata := buildReportMetadata(commits)
	metadataJSON, _ := json.Marshal(metadata)

	// Create or update report
	if exists {
		existingReport, err := s.db.GetWeeklyReportByRepoAndWeek(repo.ID, year, week)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing report: %w", err)
		}

		existingReport.Summary = run.Summary
		existingReport.CommitCount = len(commits)
		existingReport.Metadata = sql.NullString{String: string(metadataJSON), Valid: true}
		existingReport.AgentMode = run.AgentMode
		existingReport.ToolUsageStats = run.ToolUsageStats
		existingReport.SourceRunID = sql.NullInt64{Int64: run.ID, Valid: true}

		if err := s.db.UpdateWeeklyReport(existingReport); err != nil {
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

	return s.db.CreateWeeklyReport(report)
}

// previousWeek returns the previous ISO week, handling year boundaries
func previousWeek(year, week int) (int, int) {
	if week == 1 {
		prevYearEnd := time.Date(year-1, 12, 28, 0, 0, 0, 0, time.UTC)
		prevYear, prevWeek := prevYearEnd.ISOWeek()
		return prevYear, prevWeek
	}
	return year, week - 1
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
