package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/scheduler"
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

	// Log configured seed admin
	if seedAdmin := cfg.GetSeedAdmin(); seedAdmin != "" {
		slog.Info("Seed admin configured", "email", seedAdmin)
	} else {
		slog.Warn("No seed_admin configured")
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

	// Log current admins and auth header
	slog.Info("Auth header configured", "header", cfg.GetAuthHeader())
	admins, err := services.Admin.List()
	if err != nil {
		slog.Error("Failed to list admins", "error", err)
	} else if len(admins) == 0 {
		slog.Warn("No admin users configured")
	} else {
		for _, admin := range admins {
			slog.Info("Admin user", "email", admin.Email, "created_by", admin.CreatedBy)
		}
	}

	s.registerRoutes()

	return s, nil
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Health check endpoint (no auth required)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Serve static files from embedded filesystem
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))

	// Public routes (wrapped with auth middleware to populate user context)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /repos", s.handleRepoList)
	s.mux.HandleFunc("GET /repos/{name}", s.handleRepoReports)
	s.mux.HandleFunc("GET /reports/{id}", s.handleReportView)

	// Subscription routes (require authentication)
	s.mux.HandleFunc("GET /subscriptions", RequireAuth(s.handleSubscriptions))
	s.mux.HandleFunc("POST /subscriptions/toggle", RequireAuth(s.handleSubscriptionToggle))
	s.mux.HandleFunc("POST /subscriptions/toggle-all", RequireAuth(s.handleSubscriptionToggleAll))

	// Admin routes (require admin privileges)
	s.mux.HandleFunc("GET /admin", RequireAdmin(s.handleAdmin))
	s.mux.HandleFunc("GET /admin/repos", RequireAdmin(s.handleAdminRepos))
	s.mux.HandleFunc("POST /admin/repos/add", RequireAdmin(s.handleAdminRepoAdd))
	s.mux.HandleFunc("POST /admin/repos/remove", RequireAdmin(s.handleAdminRepoRemove))
	s.mux.HandleFunc("POST /admin/repos/toggle", RequireAdmin(s.handleAdminRepoToggle))
	s.mux.HandleFunc("POST /admin/repos/set-url", RequireAdmin(s.handleAdminRepoSetURL))
	s.mux.HandleFunc("POST /admin/repos/update", RequireAdmin(s.handleAdminRepoUpdate))
	s.mux.HandleFunc("GET /admin/subscribers", RequireAdmin(s.handleAdminSubscribers))
	s.mux.HandleFunc("POST /admin/subscribers/add", RequireAdmin(s.handleAdminSubscriberAdd))
	s.mux.HandleFunc("POST /admin/subscribers/remove", RequireAdmin(s.handleAdminSubscriberRemove))
	s.mux.HandleFunc("GET /admin/actions", RequireAdmin(s.handleAdminActions))
	s.mux.HandleFunc("POST /admin/update", RequireAdmin(s.handleAdminUpdateRepos))
	s.mux.HandleFunc("POST /admin/generate", RequireAdmin(s.handleAdminGenerateReport))
	s.mux.HandleFunc("POST /admin/send", RequireAdmin(s.handleAdminSendNewsletter))
	s.mux.HandleFunc("POST /admin/update/stream", RequireAdmin(s.handleAdminUpdateReposStream))
	s.mux.HandleFunc("POST /admin/generate/stream", RequireAdmin(s.handleAdminGenerateReportStream))
	s.mux.HandleFunc("POST /admin/send/stream", RequireAdmin(s.handleAdminSendNewsletterStream))
	s.mux.HandleFunc("GET /admin/admins", RequireAdmin(s.handleAdminAdmins))
	s.mux.HandleFunc("POST /admin/admins/add", RequireAdmin(s.handleAdminAdminAdd))
	s.mux.HandleFunc("POST /admin/admins/remove", RequireAdmin(s.handleAdminAdminRemove))
}

// Start starts the HTTP server with graceful shutdown on SIGINT/SIGTERM.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	handler := s.auth.Middleware(s.mux)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Listen for shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start newsletter auto-send scheduler
	sched := scheduler.New(s.services, s.cfg)
	go sched.Run(ctx)

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for signal or server error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("Shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		slog.Info("Server stopped")
		return nil
	}
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}
