package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/service"
)

// Server hosts the read-only API on a dedicated port, separate from the
// user-facing web server. Auth is by static bearer tokens, intended for
// agent-to-API access.
type Server struct {
	srv *http.Server
}

// NewServer constructs an API HTTP server. The caller is responsible for
// ensuring cfg.API.Enabled is true.
func NewServer(cfg *config.Config, services *service.Services) (*Server, error) {
	tokens := cfg.API.ResolvedTokens()
	if len(tokens) == 0 {
		return nil, errors.New("api.tokens is empty; configure at least one bearer token")
	}
	slog.Info("API auth configured", "tokens", len(tokens))

	impl := NewImpl(services.Repo, services.Report)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	apiHandler := HandlerWithOptions(NewStrictHandler(impl, nil), StdHTTPServerOptions{
		BaseURL:          "/v1",
		BaseRouter:       mux,
		Middlewares:      []MiddlewareFunc{BearerAuth(tokens)},
		ErrorHandlerFunc: writeJSONError,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port),
		Handler:           apiHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return &Server{srv: srv}, nil
}

func (s *Server) Address() string { return s.srv.Addr }

// Run starts the server and blocks until ctx is cancelled or the listener
// errors. On cancellation it performs a graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	slog.Info("Starting API server", "address", s.Address())

	errCh := make(chan error, 1)
	go func() {
		err := s.srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("Shutting down API server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("api server shutdown: %w", err)
		}
		<-errCh // drain so the listener goroutine has exited
		slog.Info("API server stopped")
		return nil
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// writeJSONError is wired into oapi-codegen as both the request-param and
// response error handler. Request-param failures (bad path/query types) are
// the only expected calls and warrant 400; anything else means a handler
// returned an unexpected error, which we surface as 500.
func writeJSONError(w http.ResponseWriter, _ *http.Request, err error) {
	status := http.StatusInternalServerError
	var ipfe *InvalidParamFormatError
	var rpe *RequiredParamError
	var rhe *RequiredHeaderError
	var tme *TooManyValuesForParamError
	var upe *UnmarshalingParamError
	var uce *UnescapedCookieParamError
	if errors.As(err, &ipfe) || errors.As(err, &rpe) || errors.As(err, &rhe) ||
		errors.As(err, &tme) || errors.As(err, &upe) || errors.As(err, &uce) {
		status = http.StatusBadRequest
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Error{Error: err.Error()})
}
