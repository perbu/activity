package web

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/service"
)

// AuthUser represents an authenticated user
type AuthUser struct {
	Email   string
	IsAdmin bool
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const authUserKey contextKey = "authUser"

// AuthMiddleware handles user authentication
type AuthMiddleware struct {
	headerName   string
	adminService *service.AdminService
	devMode      bool
	devUser      string
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(cfg *config.Config, adminService *service.AdminService) *AuthMiddleware {
	return &AuthMiddleware{
		headerName:   cfg.GetAuthHeader(),
		adminService: adminService,
		devMode:      cfg.Web.DevMode,
		devUser:      cfg.GetDevUser(),
	}
}

// Middleware wraps an http.Handler and injects user info into the request context
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user *AuthUser

		if m.devMode {
			// In dev mode, always use the dev user and treat as admin
			user = &AuthUser{
				Email:   m.devUser,
				IsAdmin: true,
			}
		} else {
			// Production mode: read email from header
			email := r.Header.Get(m.headerName)
			if email != "" {
				isAdmin, err := m.adminService.IsAdmin(email)
				if err != nil {
					slog.Error("Failed to check admin status", "email", email, "error", err)
					isAdmin = false
				}
				user = &AuthUser{
					Email:   email,
					IsAdmin: isAdmin,
				}
			}
		}

		// Store user in context (can be nil for anonymous users)
		ctx := context.WithValue(r.Context(), authUserKey, user)

		// Log the request
		if user != nil {
			slog.Info("request", "method", r.Method, "path", r.URL.Path, "user", user.Email, "admin", user.IsAdmin)
		} else {
			slog.Info("request", "method", r.Method, "path", r.URL.Path, "user", "anonymous", "header", m.headerName, "header_value", r.Header.Get(m.headerName))
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUser retrieves the AuthUser from the request context
func GetUser(r *http.Request) *AuthUser {
	user, ok := r.Context().Value(authUserKey).(*AuthUser)
	if !ok {
		return nil
	}
	return user
}

// RequireAdmin returns middleware that requires admin privileges
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// RequireAuth returns middleware that requires authentication (any user)
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			http.Error(w, "Unauthorized: Authentication required", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
