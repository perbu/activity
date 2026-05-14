package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateWeeklyReport inserts a new weekly report into the database.
func (db *DB) CreateWeeklyReport(report *WeeklyReport) (*WeeklyReport, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO weekly_reports (repo_id, year, week, week_start, week_end, summary, commit_count, metadata, agent_mode, tool_usage_stats, source_run_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, report.RepoID, report.Year, report.Week, report.WeekStart, report.WeekEnd,
		report.Summary, report.CommitCount, report.Metadata, report.AgentMode,
		report.ToolUsageStats, report.SourceRunID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create weekly report: %w", err)
	}

	return db.GetWeeklyReport(id)
}

// GetWeeklyReport retrieves a weekly report by ID.
func (db *DB) GetWeeklyReport(id int64) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE id = $1
	`, id).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("weekly report not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get weekly report: %w", err)
	}
	return report, nil
}

// GetWeeklyReportByRepoAndWeek retrieves a weekly report by repository, year, and week.
func (db *DB) GetWeeklyReportByRepoAndWeek(repoID int64, year, week int) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE repo_id = $1 AND year = $2 AND week = $3
	`, repoID, year, week).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get weekly report: %w", err)
	}
	return report, nil
}

// GetLatestWeeklyReport retrieves the most recent weekly report for a repository.
func (db *DB) GetLatestWeeklyReport(repoID int64) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE repo_id = $1
		ORDER BY year DESC, week DESC
		LIMIT 1
	`, repoID).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No reports yet
		}
		return nil, fmt.Errorf("failed to get latest weekly report: %w", err)
	}
	return report, nil
}

// ListWeeklyReportsByRepo retrieves all weekly reports for a repository, optionally filtered by year.
func (db *DB) ListWeeklyReportsByRepo(repoID int64, year *int) ([]*WeeklyReport, error) {
	var query string
	var args []interface{}

	if year != nil {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE repo_id = $1 AND year = $2
			ORDER BY year DESC, week DESC
		`
		args = []interface{}{repoID, *year}
	} else {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE repo_id = $1
			ORDER BY year DESC, week DESC
		`
		args = []interface{}{repoID}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list weekly reports: %w", err)
	}
	defer rows.Close()

	var reports []*WeeklyReport
	for rows.Next() {
		report := &WeeklyReport{}
		if err := rows.Scan(
			&report.ID, &report.RepoID, &report.Year, &report.Week,
			&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
			&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
			&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan weekly report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// ListAllWeeklyReports retrieves all weekly reports, optionally filtered by year.
func (db *DB) ListAllWeeklyReports(year *int) ([]*WeeklyReport, error) {
	var query string
	var args []interface{}

	if year != nil {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE year = $1
			ORDER BY year DESC, week DESC, repo_id
		`
		args = []interface{}{*year}
	} else {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, false), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			ORDER BY year DESC, week DESC, repo_id
		`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list weekly reports: %w", err)
	}
	defer rows.Close()

	var reports []*WeeklyReport
	for rows.Next() {
		report := &WeeklyReport{}
		if err := rows.Scan(
			&report.ID, &report.RepoID, &report.Year, &report.Week,
			&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
			&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
			&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan weekly report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// UpdateWeeklyReport updates an existing weekly report.
func (db *DB) UpdateWeeklyReport(report *WeeklyReport) error {
	report.UpdatedAt = time.Now()
	_, err := db.Exec(`
		UPDATE weekly_reports
		SET summary = $1, commit_count = $2, metadata = $3, agent_mode = $4,
		    tool_usage_stats = $5, updated_at = $6, source_run_id = $7
		WHERE id = $8
	`, report.Summary, report.CommitCount, report.Metadata, report.AgentMode,
		report.ToolUsageStats, report.UpdatedAt, report.SourceRunID, report.ID)
	if err != nil {
		return fmt.Errorf("failed to update weekly report: %w", err)
	}
	return nil
}

// WeeklyReportExists checks if a weekly report exists for the given repo, year, and week.
func (db *DB) WeeklyReportExists(repoID int64, year, week int) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM weekly_reports
		WHERE repo_id = $1 AND year = $2 AND week = $3
	`, repoID, year, week).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check weekly report existence: %w", err)
	}
	return count > 0, nil
}

// DeleteWeeklyReport deletes a weekly report by ID.
func (db *DB) DeleteWeeklyReport(id int64) error {
	_, err := db.Exec("DELETE FROM weekly_reports WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete weekly report: %w", err)
	}
	return nil
}
