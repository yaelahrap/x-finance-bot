// Package server provides the HTTP server for the bot's admin API.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

// Config holds server configuration.
type Config struct {
	Port           int
	AdminAPIKey    string
	AllowedOrigins []string
	Store          storage.Storage
	Logger         *slog.Logger
}

// Server is the HTTP server for the admin API.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a new HTTP server with all routes and middleware configured.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	Routes(mux, cfg.Store, logger)

	// Apply middleware stack
	handler := Chain(mux,
		Recovery(logger),
		RequestLogger(logger),
		CORS(cfg.AllowedOrigins),
		RequireAPIKey(cfg.AdminAPIKey),
	)

	// Health endpoint should bypass auth
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /health", handleHealth)
	healthMux.Handle("/", handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      healthMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		httpServer: srv,
		logger:     logger,
	}
}

// Start begins listening and serving. It blocks until the server is shut down.
func (s *Server) Start() error {
	s.logger.Info("server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
