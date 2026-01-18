package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository CRUD operations

// CreateRepository inserts a new repository into the database
func (db *DB) CreateRepository(name, url, branch, localPath string) (*Repository, error) {
	result, err := db.Exec(`
		INSERT INTO repositories (name, url, branch, local_path, active)
		VALUES (?, ?, ?, ?, 1)
	`, name, url, branch, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository ID: %w", err)
	}

	return db.GetRepository(id)
}

// GetRepository retrieves a repository by ID
func (db *DB) GetRepository(id int64) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE id = ?
	`, id).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
		&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// GetRepositoryByName retrieves a repository by name
func (db *DB) GetRepositoryByName(name string) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE name = ?
	`, name).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
		&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// ListRepositories retrieves all repositories, optionally filtered by active status
func (db *DB) ListRepositories(activeOnly *bool) ([]*Repository, error) {
	query := `
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
	`
	var args []interface{}

	if activeOnly != nil {
		query += " WHERE active = ?"
		args = append(args, *activeOnly)
	}

	query += " ORDER BY name"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		repo := &Repository{}
		err := rows.Scan(
			&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
			&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// UpdateRepository updates a repository's fields
func (db *DB) UpdateRepository(repo *Repository) error {
	repo.UpdatedAt = time.Now()
	_, err := db.Exec(`
		UPDATE repositories
		SET name = ?, url = ?, branch = ?, local_path = ?, active = ?, updated_at = ?, last_run_at = ?, last_run_sha = ?
		WHERE id = ?
	`, repo.Name, repo.URL, repo.Branch, repo.LocalPath, repo.Active, repo.UpdatedAt, repo.LastRunAt, repo.LastRunSHA, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}
	return nil
}

// DeleteRepository deletes a repository by ID
func (db *DB) DeleteRepository(id int64) error {
	_, err := db.Exec("DELETE FROM repositories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}
	return nil
}

// SetRepositoryActive sets the active status of a repository
func (db *DB) SetRepositoryActive(id int64, active bool) error {
	_, err := db.Exec(`
		UPDATE repositories
		SET active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, active, id)
	if err != nil {
		return fmt.Errorf("failed to set repository active status: %w", err)
	}
	return nil
}

// ActivityRun CRUD operations

// CreateActivityRun inserts a new activity run into the database
func (db *DB) CreateActivityRun(repoID int64, startSHA, endSHA string) (*ActivityRun, error) {
	result, err := db.Exec(`
		INSERT INTO activity_runs (repo_id, start_sha, end_sha)
		VALUES (?, ?, ?)
	`, repoID, startSHA, endSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity run: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get activity run ID: %w", err)
	}

	return db.GetActivityRun(id)
}

// GetActivityRun retrieves an activity run by ID
func (db *DB) GetActivityRun(id int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, 0), tool_usage_stats
		FROM activity_runs
		WHERE id = ?
	`, id).Scan(
		&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
		&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
		&run.AgentMode, &run.ToolUsageStats,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("activity run not found")
		}
		return nil, fmt.Errorf("failed to get activity run: %w", err)
	}
	return run, nil
}

// GetLatestActivityRun retrieves the most recent activity run for a repository
func (db *DB) GetLatestActivityRun(repoID int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, 0), tool_usage_stats
		FROM activity_runs
		WHERE repo_id = ?
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

// UpdateActivityRun updates an activity run
func (db *DB) UpdateActivityRun(run *ActivityRun) error {
	_, err := db.Exec(`
		UPDATE activity_runs
		SET completed_at = ?, summary = ?, raw_data = ?, agent_mode = ?, tool_usage_stats = ?
		WHERE id = ?
	`, run.CompletedAt, run.Summary, run.RawData, run.AgentMode, run.ToolUsageStats, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update activity run: %w", err)
	}
	return nil
}
