package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/config"
	"github.com/raflyramadhan/x-finance-bot/internal/logger"
	"github.com/raflyramadhan/x-finance-bot/internal/server"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(logger.Options{
		Level:   "info",
		Env:     cfg.App.Env,
		Service: "x-finance-bot",
	})

	log.Info("starting x-finance-bot",
		"env", cfg.App.Env,
		"port", cfg.App.Port,
		"posting_mode", cfg.Bot.PostingMode,
	)

	// Initialize storage
	store, err := storage.NewSQLite(cfg.Database.URL)
	if err != nil {
		log.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Initialize HTTP server
	srv := server.New(server.Config{
		Port:           cfg.App.Port,
		AdminAPIKey:    cfg.Bot.AdminAPIKey,
		AllowedOrigins: []string{"*"},
		Store:          store,
		Logger:         log,
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in background
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("bot is running")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", "error", err)
	}

	log.Info("bot stopped")
}
