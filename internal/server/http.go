package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/publisher"
	"github.com/raflyramadhan/x-finance-bot/internal/server/dashboard"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

// Config holds server configuration.
type Config struct {
	Port           int
	AdminAPIKey    string
	AllowedOrigins []string
	Store          storage.Storage
	Publisher      publisher.Publisher
	PublishNow     PublishNow
	Logger         *slog.Logger
}

// Server is the HTTP server for the admin API.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a new HTTP server with all routes and middleware configured.
//
// Routing layout:
//   - GET /            : redirect to /dashboard
//   - GET /health      : public health probe
//   - GET /dashboard   : embedded HTML dashboard (auth happens client-side via API key)
//   - /api/*           : authenticated admin API
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	apiMux := http.NewServeMux()
	Routes(apiMux, cfg.Store, cfg.Publisher, cfg.PublishNow, logger)

	apiHandler := Chain(apiMux,
		Recovery(logger),
		RequestLogger(logger),
		CORS(cfg.AllowedOrigins),
		RequireAPIKey(cfg.AdminAPIKey),
	)

	rootMux := http.NewServeMux()

	rootMux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
	})

	rootMux.HandleFunc("GET /health", handleHealth)

	rootMux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(dashboard.IndexHTML)
	})

	rootMux.Handle("/api/", apiHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      rootMux,
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
