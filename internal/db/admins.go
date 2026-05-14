package db

import (
	"database/sql"
	"fmt"
)

// CreateAdmin inserts a new admin user into the database.
func (db *DB) CreateAdmin(email, createdBy string) (*Admin, error) {
	var createdByVal interface{}
	if createdBy != "" {
		createdByVal = createdBy
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO admins (email, created_by)
		VALUES ($1, $2)
		RETURNING id
	`, email, createdByVal).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin: %w", err)
	}

	return db.GetAdmin(id)
}

// GetAdmin retrieves an admin by ID.
func (db *DB) GetAdmin(id int64) (*Admin, error) {
	admin := &Admin{}
	err := db.QueryRow(`
		SELECT id, email, created_at, created_by
		FROM admins
		WHERE id = $1
	`, id).Scan(&admin.ID, &admin.Email, &admin.CreatedAt, &admin.CreatedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get admin: %w", err)
	}
	return admin, nil
}

// GetAdminByEmail retrieves an admin by email.
func (db *DB) GetAdminByEmail(email string) (*Admin, error) {
	admin := &Admin{}
	err := db.QueryRow(`
		SELECT id, email, created_at, created_by
		FROM admins
		WHERE email = $1
	`, email).Scan(&admin.ID, &admin.Email, &admin.CreatedAt, &admin.CreatedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get admin: %w", err)
	}
	return admin, nil
}

// ListAdmins retrieves all admins.
func (db *DB) ListAdmins() ([]*Admin, error) {
	rows, err := db.Query(`
		SELECT id, email, created_at, created_by
		FROM admins
		ORDER BY email
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}
	defer rows.Close()

	var admins []*Admin
	for rows.Next() {
		admin := &Admin{}
		if err := rows.Scan(&admin.ID, &admin.Email, &admin.CreatedAt, &admin.CreatedBy); err != nil {
			return nil, fmt.Errorf("failed to scan admin: %w", err)
		}
		admins = append(admins, admin)
	}

	return admins, nil
}

// DeleteAdmin deletes an admin by ID.
func (db *DB) DeleteAdmin(id int64) error {
	_, err := db.Exec("DELETE FROM admins WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete admin: %w", err)
	}
	return nil
}

// IsAdmin checks if an email is an admin.
func (db *DB) IsAdmin(email string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM admins WHERE email = $1
	`, email).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}
	return count > 0, nil
}

// AdminCount returns the number of admins.
func (db *DB) AdminCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count admins: %w", err)
	}
	return count, nil
}
