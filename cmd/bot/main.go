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

	"github.com/raflyramadhan/x-finance-bot/internal/ai"
	"github.com/raflyramadhan/x-finance-bot/internal/config"
	"github.com/raflyramadhan/x-finance-bot/internal/decision"
	"github.com/raflyramadhan/x-finance-bot/internal/logger"
	"github.com/raflyramadhan/x-finance-bot/internal/pipeline"
	"github.com/raflyramadhan/x-finance-bot/internal/publisher"
	"github.com/raflyramadhan/x-finance-bot/internal/scheduler"
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

	// Seed database sources
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := storage.SeedSources(ctx, store); err != nil {
		log.Error("failed to seed database sources", "error", err)
		os.Exit(1)
	}

	// Initialize AI components
	aiClient := ai.NewClient(ai.Config{
		Provider: cfg.AI.Provider,
		BaseURL:  cfg.AI.BaseURL,
		APIKey:   cfg.AI.APIKey,
		Model:    cfg.AI.Model,
	})
	reviewer := ai.NewReviewer(aiClient)

	// Initialize decision engine
	policy := decision.DefaultPolicy()
	policy.MinAutoPostScore = cfg.Bot.MinAutoPostScore
	engine := decision.NewEngine(policy)

	// Initialize X publisher
	xConfig := publisher.XClientConfig{
		APIKey:       cfg.X.APIKey,
		APISecret:    cfg.X.APISecret,
		AccessToken:  cfg.X.AccessToken,
		AccessSecret: cfg.X.AccessSecret,
	}
	xClient := publisher.NewXClient(xConfig)

	// Initialize R2 client for media hosting
	r2Client := storage.NewR2Client(storage.R2Config{
		Endpoint:        cfg.Cloudflare.R2Endpoint,
		AccessKeyID:     cfg.Cloudflare.R2AccessKeyID,
		SecretAccessKey: cfg.Cloudflare.R2SecretAccessKey,
		BucketMedia:     cfg.Cloudflare.R2BucketMedia,
		PublicURL:       cfg.Cloudflare.R2PublicURLMedia,
	})

	// Initialize pipeline Orchestrator
	orchestrator := pipeline.NewOrchestrator(
		store,
		reviewer,
		engine,
		xClient,
		r2Client,
		log,
		cfg.Bot.CMCAPIKey,
		cfg.Bot.PostingMode,
	)

	// Initialize scheduler
	sched := scheduler.New(log)

	sched.Register(scheduler.Job{
		Name: "BMKG Alerts",
		Fn:   orchestrator.ProcessBMKGAlerts,
	}, 5*time.Minute)

	sched.Register(scheduler.Job{
		Name: "Crypto Alerts",
		Fn:   orchestrator.ProcessCryptoAlerts,
	}, 5*time.Minute)

	sched.Register(scheduler.Job{
		Name: "News Sources",
		Fn:   orchestrator.ProcessNewsSources,
	}, 15*time.Minute)

	sched.Register(scheduler.Job{
		Name: "Bank Indonesia JISDOR Pulse",
		Fn:   orchestrator.ProcessBIPulse,
	}, 1*time.Hour)

	sched.Register(scheduler.Job{
		Name: "Scheduled Drafts Publisher",
		Fn:   orchestrator.ProcessScheduledDrafts,
	}, 1*time.Minute)

	// Initialize HTTP server
	srv := server.New(server.Config{
		Port:           cfg.App.Port,
		AdminAPIKey:    cfg.Bot.AdminAPIKey,
		AllowedOrigins: []string{"*"},
		Store:          store,
		Logger:         log,
	})

	// Start server in background
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start scheduler in background
	go func() {
		log.Info("starting background scheduler")
		sched.Start(ctx)
	}()

	log.Info("bot is running")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("shutdown signal received")

	// Stop scheduler
	log.Info("stopping scheduler")
	sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", "error", err)
	}

	log.Info("bot stopped")
}
