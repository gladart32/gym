package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

type Server struct {
	httpServer *http.Server
	log        *slog.Logger
}

func New(cfg *config.Config, log *slog.Logger, health *handler.Health, tasksH *handler.Tasks, authH *handler.Auth, authService *auth.Service) *Server {
	router := NewRouter(log, cfg, health, tasksH, authH, authService)

	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.HTTP.Address(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		log: log,
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled,
// then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.log.Info("http server started", "address", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutting down http server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}

		s.log.Info("http server stopped")
		return nil
	}
}
