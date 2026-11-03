package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"goempty/pkg/log"

	"go.uber.org/zap"
)

// Server manages HTTP transport configuration and lifecycle for a supplied handler.
// Construct it with New. The caller manages the start and stop sequence.
type Server struct {
	httpServer *http.Server
}

// New configures a server without opening a listener or starting goroutines.
func New(cfg Config, handler http.Handler) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("httpserver: handler is required")
	}
	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}, nil
}

// Start blocks while serving requests. Normal shutdown returns nil.
// Call Stop from another goroutine and wait for Stop before exiting the process.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("httpserver: listen: %w", err)
	}
	log.Info(context.Background(), "HTTP server started", zap.String("addr", listener.Addr().String()))
	// Serve owns and closes the listener, including on an early shutdown.
	if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: start: %w", err)
	}
	return nil
}

// Stop gracefully drains requests, forcing connections closed if Shutdown fails.
// Hijacked connections (such as WebSockets) must be managed by their owner.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("httpserver: stop: %w", errors.Join(err, s.httpServer.Close()))
	}
	return nil
}
