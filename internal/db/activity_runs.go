package db

import (
	"database/sql"
	"fmt"
)

// CreateActivityRun inserts a new activity run into the database.
func (db *DB) CreateActivityRun(repoID int64, startSHA, endSHA string) (*ActivityRun, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO activity_runs (repo_id, start_sha, end_sha)
		VALUES ($1, $2, $3)
		RETURNING id
	`, repoID, startSHA, endSHA).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity run: %w", err)
	}

	return db.GetActivityRun(id)
}

// GetActivityRun retrieves an activity run by ID.
func (db *DB) GetActivityRun(id int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, false), tool_usage_stats
		FROM activity_runs
		WHERE id = $1
	`, id).Scan(
		&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
		&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
		&run.AgentMode, &run.ToolUsageStats,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("activity run not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get activity run: %w", err)
	}
	return run, nil
}

// GetLatestActivityRun retrieves the most recent activity run for a repository.
func (db *DB) GetLatestActivityRun(repoID int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, false), tool_usage_stats
		FROM activity_runs
		WHERE repo_id = $1
		ORDER BY started_at DESC
		LIMIT 1
	`, repoID).Scan(
		&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
		&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
		&run.AgentMode, &run.ToolUsageStats,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No runs yet
		}
		return nil, fmt.Errorf("failed to get latest activity run: %w", err)
	}
	return run, nil
}

// UpdateActivityRun updates an activity run.
func (db *DB) UpdateActivityRun(run *ActivityRun) error {
	_, err := db.Exec(`
		UPDATE activity_runs
		SET completed_at = $1, summary = $2, raw_data = $3, agent_mode = $4, tool_usage_stats = $5
		WHERE id = $6
	`, run.CompletedAt, run.Summary, run.RawData, run.AgentMode, run.ToolUsageStats, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update activity run: %w", err)
	}
	return nil
}
