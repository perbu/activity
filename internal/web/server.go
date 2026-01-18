package web

import (
	"fmt"
	"log"
	"net/http"

	"github.com/perbu/activity/internal/db"
)

// Server is the HTTP server for the web UI
type Server struct {
	db        *db.DB
	templates *Templates
	mux       *http.ServeMux
	host      string
	port      int
}

// NewServer creates a new web server
func NewServer(database *db.DB, host string, port int) (*Server, error) {
	templates, err := ParseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	s := &Server{
		db:        database,
		templates: templates,
		mux:       http.NewServeMux(),
		host:      host,
		port:      port,
	}

	s.registerRoutes()

	return s, nil
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /repos", s.handleRepoList)
	s.mux.HandleFunc("GET /repos/{name}", s.handleRepoReports)
	s.mux.HandleFunc("GET /reports/{id}", s.handleReportView)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("Starting web server at http://%s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}
