package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/hamza3256/bluesheet/internal/config"
	"github.com/hamza3256/bluesheet/internal/domain"
	"github.com/hamza3256/bluesheet/internal/report"
	"github.com/hamza3256/bluesheet/internal/storage"
	"github.com/hamza3256/bluesheet/internal/store"
)

type Worker struct {
	repo     *store.Repository
	gen      report.Generator
	uploader storage.Uploader
	cfg      *config.Config
}

func New(repo *store.Repository, gen report.Generator, uploader storage.Uploader, cfg *config.Config) *Worker {
	return &Worker{repo: repo, gen: gen, uploader: uploader, cfg: cfg}
}

// Run starts cfg.WorkerConcurrency goroutines that poll for work.
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.WorkerConcurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.pollLoop(ctx, id)
		}(i)
	}
	wg.Wait()
}

func (w *Worker) pollLoop(ctx context.Context, workerID int) {
	log := slog.With("worker_id", workerID)
	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down")
			return
		default:
		}

		req, err := w.repo.DequeueRequest(ctx)
		if err != nil {
			log.Error("dequeue error", "error", err)
			sleep(ctx, w.cfg.WorkerPollInterval)
			continue
		}
		if req == nil {
			sleep(ctx, w.cfg.WorkerPollInterval)
			continue
		}

		log.Info("processing request", "request_id", req.ID, "ticker", req.Ticker)
		w.process(ctx, req, log)
	}
}

func (w *Worker) process(ctx context.Context, req *domain.BlueSheetRequest, log *slog.Logger) {
	tmpFile, err := os.CreateTemp("", "bluesheet-*.csv")
	if err != nil {
		w.fail(ctx, req, fmt.Errorf("create temp file: %w", err), log)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	rowCount, err := w.gen.Generate(ctx, req, tmpFile)
	tmpFile.Close()
	if err != nil {
		w.fail(ctx, req, fmt.Errorf("generate: %w", err), log)
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		w.fail(ctx, req, fmt.Errorf("reopen file: %w", err), log)
		return
	}
	defer f.Close()

	key := fmt.Sprintf("%s/%s_%s_%s.csv",
		req.ID,
		req.Ticker,
		req.StartTime.Format("20060102"),
		req.EndTime.Format("20060102"),
	)

	etag, err := w.uploader.Upload(ctx, w.cfg.S3Bucket, key, f)
	if err != nil {
		w.fail(ctx, req, fmt.Errorf("upload: %w", err), log)
		return
	}

	if err := w.repo.CompleteRequest(ctx, req.ID, key, etag, rowCount); err != nil {
		w.fail(ctx, req, fmt.Errorf("complete: %w", err), log)
		return
	}

	log.Info("request completed", "request_id", req.ID, "rows", rowCount, "s3_key", key)
}

func (w *Worker) fail(ctx context.Context, req *domain.BlueSheetRequest, cause error, log *slog.Logger) {
	log.Error("request failed", "request_id", req.ID, "error", cause)
	_ = w.repo.FailRequest(ctx, req.ID, cause.Error())
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
