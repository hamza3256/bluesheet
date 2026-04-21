package worker_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hamza3256/bluesheet/internal/config"
	"github.com/hamza3256/bluesheet/internal/domain"
	"github.com/hamza3256/bluesheet/internal/report"
	"github.com/hamza3256/bluesheet/internal/store"
	"github.com/hamza3256/bluesheet/internal/worker"
)

type mockUploader struct {
	mu      sync.Mutex
	uploads map[string][]byte
}

func (m *mockUploader) Upload(_ context.Context, bucket, key string, body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.uploads[key] = data
	m.mu.Unlock()
	return `"mock-etag"`, nil
}

func setupWorkerTest(t *testing.T) (*store.Repository, *mockUploader, *config.Config) {
	t.Helper()
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("bluesheet_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := store.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		S3Bucket:           "test-bucket",
		WorkerPollInterval: 100 * time.Millisecond,
		WorkerConcurrency:  1,
	}
	up := &mockUploader{uploads: make(map[string][]byte)}
	return store.New(pool), up, cfg
}

func TestWorkerHappyPath(t *testing.T) {
	repo, up, cfg := setupWorkerTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	in := domain.CreateRequestInput{
		Ticker:    "AAPL",
		StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	req, err := repo.CreateRequest(ctx, in)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	gen := report.NewStubGenerator()
	w := worker.New(repo, gen, up, cfg)

	workerCtx, workerCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.Run(workerCtx)
		close(done)
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for request to succeed")
		case <-ticker.C:
			got, err := repo.GetRequest(ctx, req.ID)
			if err != nil {
				t.Fatalf("get request: %v", err)
			}
			if got.Status == domain.StatusSucceeded {
				if got.S3Key == nil {
					t.Fatal("s3_key is nil on succeeded request")
				}
				if got.RowCount == nil || *got.RowCount < 1 {
					t.Errorf("row_count = %v, want >= 1", got.RowCount)
				}

				up.mu.Lock()
				data, ok := up.uploads[*got.S3Key]
				up.mu.Unlock()
				if !ok {
					t.Fatal("upload not found")
				}
				if !bytes.Contains(data, []byte("trade_id")) {
					t.Error("CSV header not found in uploaded data")
				}

				workerCancel()
				<-done
				return
			}
			if got.Status == domain.StatusFailed {
				t.Fatalf("request failed: %v", got.ErrorMessage)
			}
		}
	}
}
