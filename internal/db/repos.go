package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateRepository inserts a new repository into the database.
func (db *DB) CreateRepository(name, url, branch string, private, external bool, description, forgeType, forgeOwner, forgeRepo sql.NullString) (*Repository, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO repositories (name, url, branch, active, private, external, description, forge_type, forge_owner, forge_repo)
		VALUES ($1, $2, $3, true, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, name, url, branch, private, external, description, forgeType, forgeOwner, forgeRepo).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return db.GetRepository(id)
}

// GetRepository retrieves a repository by ID.
func (db *DB) GetRepository(id int64) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, active, COALESCE(private, false), COALESCE(external, false), description,
		       forge_type, forge_owner, forge_repo, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE id = $1
	`, id).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch,
		&repo.Active, &repo.Private, &repo.External, &repo.Description,
		&repo.ForgeType, &repo.ForgeOwner, &repo.ForgeRepo,
		&repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// GetRepositoryByName retrieves a repository by name.
func (db *DB) GetRepositoryByName(name string) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, active, COALESCE(private, false), COALESCE(external, false), description,
		       forge_type, forge_owner, forge_repo, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE name = $1
	`, name).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch,
		&repo.Active, &repo.Private, &repo.External, &repo.Description,
		&repo.ForgeType, &repo.ForgeOwner, &repo.ForgeRepo,
		&repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// ListRepositories retrieves all repositories, optionally filtered by active status.
func (db *DB) ListRepositories(activeOnly *bool) ([]*Repository, error) {
	query := `
		SELECT id, name, url, branch, active, COALESCE(private, false), COALESCE(external, false), description,
		       forge_type, forge_owner, forge_repo, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
	`
	var args []any

	if activeOnly != nil {
		query += " WHERE active = $1"
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
			&repo.ID, &repo.Name, &repo.URL, &repo.Branch,
			&repo.Active, &repo.Private, &repo.External, &repo.Description,
			&repo.ForgeType, &repo.ForgeOwner, &repo.ForgeRepo,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// UpdateRepository updates a repository's fields.
func (db *DB) UpdateRepository(repo *Repository) error {
	repo.UpdatedAt = time.Now()
	_, err := db.Exec(`
		UPDATE repositories
		SET name = $1, url = $2, branch = $3, active = $4, private = $5, external = $6, description = $7,
		    forge_type = $8, forge_owner = $9, forge_repo = $10, updated_at = $11, last_run_at = $12, last_run_sha = $13
		WHERE id = $14
	`, repo.Name, repo.URL, repo.Branch, repo.Active, repo.Private, repo.External, repo.Description,
		repo.ForgeType, repo.ForgeOwner, repo.ForgeRepo, repo.UpdatedAt, repo.LastRunAt, repo.LastRunSHA, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}
	return nil
}

// DeleteRepository deletes a repository by ID.
func (db *DB) DeleteRepository(id int64) error {
	_, err := db.Exec("DELETE FROM repositories WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}
	return nil
}

// SetRepositoryActive sets the active status of a repository.
func (db *DB) SetRepositoryActive(id int64, active bool) error {
	_, err := db.Exec(`
		UPDATE repositories
		SET active = $1, updated_at = NOW()
		WHERE id = $2
	`, active, id)
	if err != nil {
		return fmt.Errorf("failed to set repository active status: %w", err)
	}
	return nil
}
