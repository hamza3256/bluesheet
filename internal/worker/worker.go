package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	w.notify(ctx, req, domain.StatusSucceeded, nil, log)
}

func (w *Worker) fail(ctx context.Context, req *domain.BlueSheetRequest, cause error, log *slog.Logger) {
	log.Error("request failed", "request_id", req.ID, "error", cause)
	_ = w.repo.FailRequest(ctx, req.ID, cause.Error())
	errMsg := cause.Error()
	w.notify(ctx, req, domain.StatusFailed, &errMsg, log)
}

// notify POSTs to the request's callback_url (if set) with the final status.
func (w *Worker) notify(ctx context.Context, req *domain.BlueSheetRequest, status domain.RequestStatus, errMsg *string, log *slog.Logger) {
	if req.CallbackURL == nil || *req.CallbackURL == "" {
		return
	}

	payload := map[string]any{
		"request_id": req.ID,
		"ticker":     req.Ticker,
		"status":     status,
	}
	if errMsg != nil {
		payload["error"] = *errMsg
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, *req.CallbackURL, bytes.NewReader(body))
	if err != nil {
		log.Warn("callback: build request failed", "url", *req.CallbackURL, "error", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Warn("callback: POST failed", "url", *req.CallbackURL, "error", err)
		return
	}
	resp.Body.Close()
	log.Info("callback sent", "url", *req.CallbackURL, "status_code", resp.StatusCode)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
