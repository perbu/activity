package service

import (
	"fmt"
	"log/slog"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
)

// AdminService handles admin user management
type AdminService struct {
	db  *db.DB
	cfg *config.Config
}

// NewAdminService creates a new AdminService
func NewAdminService(database *db.DB, cfg *config.Config) *AdminService {
	return &AdminService{
		db:  database,
		cfg: cfg,
	}
}

// Add creates a new admin user
func (s *AdminService) Add(email, createdBy string) (*db.Admin, error) {
	// Check if already exists
	existing, err := s.db.GetAdminByEmail(email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("admin '%s' already exists", email)
	}

	admin, err := s.db.CreateAdmin(email, createdBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin: %w", err)
	}

	slog.Info("Admin added", "email", email, "created_by", createdBy)
	return admin, nil
}

// Remove deletes an admin user by ID
func (s *AdminService) Remove(id int64) error {
	admin, err := s.db.GetAdmin(id)
	if err != nil {
		return fmt.Errorf("admin not found: %w", err)
	}

	if err := s.db.DeleteAdmin(id); err != nil {
		return fmt.Errorf("failed to delete admin: %w", err)
	}

	slog.Info("Admin removed", "email", admin.Email)
	return nil
}

// IsAdmin checks if an email is an admin
func (s *AdminService) IsAdmin(email string) (bool, error) {
	return s.db.IsAdmin(email)
}

// List returns all admin users
func (s *AdminService) List() ([]*db.Admin, error) {
	return s.db.ListAdmins()
}

// SeedIfNeeded creates the seed admin if no admins exist
func (s *AdminService) SeedIfNeeded() error {
	count, err := s.db.AdminCount()
	if err != nil {
		return fmt.Errorf("failed to count admins: %w", err)
	}

	if count > 0 {
		return nil // Admins already exist
	}

	seedEmail := s.cfg.GetSeedAdmin()
	if seedEmail == "" {
		slog.Warn("No admins configured and no seed_admin specified")
		return nil
	}

	admin, err := s.db.CreateAdmin(seedEmail, "system")
	if err != nil {
		return fmt.Errorf("failed to create seed admin: %w", err)
	}

	slog.Info("Seed admin created", "email", admin.Email)
	return nil
}

// EnsureDevAdmin ensures the dev user is an admin (for dev mode)
func (s *AdminService) EnsureDevAdmin() error {
	if !s.cfg.Web.DevMode {
		return nil
	}

	devUser := s.cfg.GetDevUser()
	isAdmin, err := s.db.IsAdmin(devUser)
	if err != nil {
		return fmt.Errorf("failed to check dev admin: %w", err)
	}

	if !isAdmin {
		_, err := s.db.CreateAdmin(devUser, "dev_mode")
		if err != nil {
			return fmt.Errorf("failed to create dev admin: %w", err)
		}
		slog.Info("Dev admin created", "email", devUser)
	}

	return nil
}
