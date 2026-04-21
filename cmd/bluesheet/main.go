package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hamza3256/bluesheet/internal/config"
	"github.com/hamza3256/bluesheet/internal/httpapi"
	"github.com/hamza3256/bluesheet/internal/report"
	"github.com/hamza3256/bluesheet/internal/storage"
	"github.com/hamza3256/bluesheet/internal/store"
	"github.com/hamza3256/bluesheet/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: bluesheet <api|worker|migrate>")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "api":
		runAPI(ctx, cfg)
	case "worker":
		runWorker(ctx, cfg)
	case "migrate":
		runMigrate(cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runAPI(ctx context.Context, cfg *config.Config) {
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := store.New(db)
	srv := httpapi.NewServer(cfg, repo)

	slog.Info("starting API server", "addr", cfg.HTTPAddr)
	if err := srv.Run(ctx); err != nil {
		slog.Error("api server error", "error", err)
		os.Exit(1)
	}
}

func runWorker(ctx context.Context, cfg *config.Config) {
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := store.New(db)
	uploader, err := storage.NewS3Uploader(ctx, cfg)
	if err != nil {
		slog.Error("s3 client init failed", "error", err)
		os.Exit(1)
	}

	gen := report.NewStubGenerator()
	w := worker.New(repo, gen, uploader, cfg)

	slog.Info("starting worker", "concurrency", cfg.WorkerConcurrency, "poll_interval", cfg.WorkerPollInterval)
	w.Run(ctx)
}

func runMigrate(cfg *config.Config) {
	ctx := context.Background()
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied successfully")
}
