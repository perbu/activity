package web

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/service"
)

// Server is the HTTP server for the web UI
type Server struct {
	db        *db.DB
	services  *service.Services
	cfg       *config.Config
	templates *Templates
	mux       *http.ServeMux
	auth      *AuthMiddleware
	host      string
	port      int
}

// NewServer creates a new web server
func NewServer(database *db.DB, services *service.Services, cfg *config.Config, host string, port int) (*Server, error) {
	templates, err := ParseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	auth := NewAuthMiddleware(cfg, services.Admin)

	s := &Server{
		db:        database,
		services:  services,
		cfg:       cfg,
		templates: templates,
		mux:       http.NewServeMux(),
		auth:      auth,
		host:      host,
		port:      port,
	}

	// Seed admin if needed
	if err := services.Admin.SeedIfNeeded(); err != nil {
		slog.Error("Failed to seed admin", "error", err)
	}

	// Ensure dev admin if in dev mode
	if err := services.Admin.EnsureDevAdmin(); err != nil {
		slog.Error("Failed to ensure dev admin", "error", err)
	}

	if cfg.Web.DevMode {
		slog.Warn("Running in dev mode - auth disabled", "dev_user", cfg.GetDevUser())
	}

	s.registerRoutes()

	return s, nil
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Serve static files from embedded filesystem
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))

	// Public routes (wrapped with auth middleware to populate user context)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /repos", s.handleRepoList)
	s.mux.HandleFunc("GET /repos/{name}", s.handleRepoReports)
	s.mux.HandleFunc("GET /reports/{id}", s.handleReportView)

	// Admin routes (require admin privileges)
	s.mux.HandleFunc("GET /admin", RequireAdmin(s.handleAdmin))
	s.mux.HandleFunc("GET /admin/repos", RequireAdmin(s.handleAdminRepos))
	s.mux.HandleFunc("POST /admin/repos/add", RequireAdmin(s.handleAdminRepoAdd))
	s.mux.HandleFunc("POST /admin/repos/remove", RequireAdmin(s.handleAdminRepoRemove))
	s.mux.HandleFunc("POST /admin/repos/toggle", RequireAdmin(s.handleAdminRepoToggle))
	s.mux.HandleFunc("POST /admin/repos/set-url", RequireAdmin(s.handleAdminRepoSetURL))
	s.mux.HandleFunc("GET /admin/subscribers", RequireAdmin(s.handleAdminSubscribers))
	s.mux.HandleFunc("POST /admin/subscribers/add", RequireAdmin(s.handleAdminSubscriberAdd))
	s.mux.HandleFunc("POST /admin/subscribers/remove", RequireAdmin(s.handleAdminSubscriberRemove))
	s.mux.HandleFunc("GET /admin/actions", RequireAdmin(s.handleAdminActions))
	s.mux.HandleFunc("POST /admin/update", RequireAdmin(s.handleAdminUpdateRepos))
	s.mux.HandleFunc("POST /admin/generate", RequireAdmin(s.handleAdminGenerateReport))
	s.mux.HandleFunc("POST /admin/send", RequireAdmin(s.handleAdminSendNewsletter))
	s.mux.HandleFunc("GET /admin/admins", RequireAdmin(s.handleAdminAdmins))
	s.mux.HandleFunc("POST /admin/admins/add", RequireAdmin(s.handleAdminAdminAdd))
	s.mux.HandleFunc("POST /admin/admins/remove", RequireAdmin(s.handleAdminAdminRemove))
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	// Wrap the mux with auth middleware to populate user context on all requests
	handler := s.auth.Middleware(s.mux)
	return http.ListenAndServe(addr, handler)
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}
